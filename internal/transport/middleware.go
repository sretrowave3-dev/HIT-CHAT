package transport

import (
	"context"
	"net/http"
	"strings"

	"github.com/hit-chat/campusnet/internal/auth"
)

type contextKey string

const (
	ClaimsContextKey contextKey = "device_claims"
)

// AuthMiddleware validates JWT bearer tokens on incoming HTTP requests.
func AuthMiddleware(tokenMgr *auth.TokenManager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		tokenStr := ""

		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			tokenStr = strings.TrimSpace(authHeader[7:])
		} else {
			// Also allow token in query parameter for WebSocket upgrade
			tokenStr = r.URL.Query().Get("token")
		}

		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization header or token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := tokenMgr.ValidateToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// CORSMiddleware enables cross-origin requests for verified clients and attaches strict security headers.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security Headers (Clickjacking, MIME-sniffing, CSP)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; font-src 'self'; img-src 'self' data:; frame-ancestors 'none';")

		origin := r.Header.Get("Origin")
		if origin != "" && isOriginAllowed(r) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}

		// Path traversal and directory traversal defense
		uri := r.RequestURI
		path := r.URL.Path
		rawPath := r.URL.RawPath
		lowerURI := strings.ToLower(uri)
		if strings.Contains(uri, "..") || strings.Contains(path, "..") || strings.Contains(rawPath, "..") ||
			strings.Contains(uri, `\`) || strings.Contains(path, `\`) ||
			strings.Contains(lowerURI, "%2e") || strings.Contains(lowerURI, "%2f%2f") ||
			strings.Contains(uri, "\x00") || strings.Contains(lowerURI, "%00") {
			http.Error(w, "Access Denied: Path traversal detected", http.StatusForbidden)
			return
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
