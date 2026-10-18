package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hit-chat/campusnet/internal/auth"
	"github.com/hit-chat/campusnet/internal/ephemeral"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/internal/transport"
	"github.com/redis/go-redis/v9"
)

func main() {
	port := flag.Int("port", 8080, "HTTP and WebSocket server port")
	redisAddr := flag.String("redis", "", "Redis address (e.g. localhost:6379). Defaults to in-memory store if unset")
	jwtSecret := flag.String("secret", "campusnet-dev-session-secret-change-in-production-2026", "JWT session signing secret")
	flag.Parse()

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
	tokenMgr := auth.NewTokenManager(*jwtSecret)
	deviceFlowMgr := auth.NewDeviceFlowManager(tokenMgr, fmt.Sprintf("http://localhost:%d/pair", *port))
	bundleRegistry := auth.NewBundleRegistry()
	geoResolver := geo.NewBSSIDResolver("")
	privacyFilter := geo.NewPrivacyFilter(geoResolver)

	// 3. Initialize WebSocket Hub & Transport
	hub := transport.NewHub(ephemMgr, privacyFilter)
	go hub.Run()

	router := transport.NewRouter(deviceFlowMgr, bundleRegistry, tokenMgr, geoResolver, privacyFilter)

	// 4. Register WebSocket stream handler
	mainMux := http.NewServeMux()
	mainMux.Handle("/api/v1/", router.Handler())
	mainMux.HandleFunc("/ws/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(tokenMgr, w, r)
	})

	serverAddr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      transport.CORSMiddleware(mainMux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 5. Start Server in Goroutine
	go func() {
		log.Printf("[READY] Gateway listening on http://0.0.0.0%s", serverAddr)
		log.Printf("[READY] WebSocket stream available at ws://0.0.0.0%s/ws/v1/stream", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server listen failed: %v", err)
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
