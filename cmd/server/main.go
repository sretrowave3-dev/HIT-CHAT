package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hit-chat/campusnet/internal/auth"
	"github.com/hit-chat/campusnet/internal/ephemeral"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/internal/transport"
	"github.com/redis/go-redis/v9"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Host address to bind to (e.g. 127.0.0.1 or 0.0.0.0)")
	port := flag.Int("port", 8080, "HTTP and WebSocket server port")
	lan := flag.Bool("lan", false, "Enable LAN access (binds 0.0.0.0, enables local HTTPS and discovers LAN IPs)")
	enableTLS := flag.Bool("tls", false, "Enforce local HTTPS/WSS with auto-generated local dev CA")
	devAuth := flag.Bool("dev-auth", false, "Enable development identity switcher endpoint (restricted to private network)")
	redisAddr := flag.String("redis", "", "Redis address (e.g. localhost:6379). Defaults to in-memory store if unset")
	jwtSecret := flag.String("secret", "campusnet-dev-session-secret-change-in-production-2026", "JWT session signing secret")
	flag.Parse()

	bindHost := *host
	if *lan && bindHost == "127.0.0.1" {
		bindHost = "0.0.0.0"
	}

	useTLS := *enableTLS || *lan

	effectiveJWTSecret := *jwtSecret
	if envSecret := os.Getenv("JWT_SECRET"); envSecret != "" {
		effectiveJWTSecret = envSecret
	}

	campusSalt := os.Getenv("CAMPUS_SALT")

	// Fail-closed enforcement for production deployments
	isProduction := strings.EqualFold(os.Getenv("CAMPUSNET_ENV"), "production") || strings.EqualFold(os.Getenv("ENV"), "production")
	if isProduction {
		if effectiveJWTSecret == "campusnet-dev-session-secret-change-in-production-2026" || len(effectiveJWTSecret) < 32 {
			log.Fatalf("[FATAL] Security misconfiguration: Production mode requires a strong, non-default JWT_SECRET of at least 32 characters!")
		}
		if campusSalt == "" || campusSalt == geo.DefaultCampusSalt {
			log.Fatalf("[FATAL] Security misconfiguration: Production mode requires a non-default CAMPUS_SALT environment variable!")
		}
	}

	log.Println("==================================================================")
	log.Println("           CAMPUSNET EPHEMERAL GATEWAY SERVER v1.0.0             ")
	log.Println("     High-Security E2EE Ratchet Relay & Burn-On-Read Core         ")
	log.Println("==================================================================")

	// 1. Initialize Ephemeral Store (Redis with fallback to In-Memory)
	var ephemStore ephemeral.Store
	if *redisAddr != "" {
		log.Printf("[INIT] Connecting to Redis at %s...", *redisAddr)
		rdb := redis.NewClient(&redis.Options{
			Addr: *redisAddr,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := rdb.Ping(ctx).Err(); err != nil {
			cancel()
			log.Printf("[WARN] Redis ping failed (%v). Falling back to resilient In-Memory store.", err)
			ephemStore = ephemeral.NewMemoryStore(500 * time.Millisecond)
		} else {
			cancel()
			log.Println("[INIT] Connected to Redis ephemeral backend.")
			ephemStore = ephemeral.NewRedisStore(rdb)
		}
	} else {
		log.Println("[INIT] No Redis address specified. Running In-Memory ephemeral store.")
		ephemStore = ephemeral.NewMemoryStore(500 * time.Millisecond)
	}

	ephemMgr := ephemeral.NewManager(ephemStore)
	defer ephemMgr.Close()

	// 2. Initialize Core Subsystems
	tokenMgr := auth.NewTokenManager(effectiveJWTSecret)
	deviceFlowMgr := auth.NewDeviceFlowManager(tokenMgr, fmt.Sprintf("http://localhost:%d/pair", *port))
	bundleRegistry := auth.NewBundleRegistry()
	geoResolver := geo.NewBSSIDResolver(campusSalt)
	privacyFilter := geo.NewPrivacyFilter(geoResolver)

	// 3. Initialize WebSocket Hub & Transport
	hub := transport.NewHub(ephemMgr, privacyFilter)
	go hub.Run()

	router := transport.NewRouter(deviceFlowMgr, bundleRegistry, tokenMgr, geoResolver, privacyFilter)
	router.SetDevAuthEnabled(*devAuth)

	// Discover LAN IPs
	lanIPs, err := transport.GetLocalIPv4Addresses()
	if err != nil {
		log.Printf("[WARN] Failed to discover local IPv4 addresses: %v", err)
	}

	var certFile, keyFile, caPath string
	if useTLS {
		certFile, keyFile, caPath, err = transport.EnsureDevCertificates(".campusnet-tls", lanIPs)
		if err != nil {
			log.Fatalf("[FATAL] Generating local TLS certificates failed: %v", err)
		}
	}

	// 4. Register WebSocket stream handler and REST/SPA router
	mainMux := http.NewServeMux()
	mainMux.HandleFunc("/ws/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(tokenMgr, w, r)
	})
	mainMux.Handle("/", router.Handler())

	serverAddr := fmt.Sprintf("%s:%d", bindHost, *port)
	srv := &http.Server{
		Addr:           serverAddr,
		Handler:        transport.CORSMiddleware(mainMux),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB header limit to protect against header bomb DoS
	}

	// 5. Start Server in Goroutine
	go func() {
		transport.PrintLANBanner(bindHost, *port, lanIPs, useTLS, caPath)
		if useTLS {
			if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[FATAL] HTTPS Server listen failed: %v", err)
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[FATAL] Server listen failed: %v", err)
			}
		}
	}()

	// 6. Graceful Shutdown on Signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[SHUTDOWN] Shutting down CampusNet Gateway gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	}
	log.Println("[SHUTDOWN] Server exited cleanly. Ephemeral buffers purged.")
}
