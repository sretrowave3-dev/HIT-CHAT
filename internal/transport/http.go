package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hit-chat/campusnet/internal/auth"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

// Router sets up HTTP REST endpoints.
type Router struct {
	mux           *http.ServeMux
	deviceFlow    *auth.DeviceFlowManager
	bundleReg     *auth.BundleRegistry
	tokenMgr      *auth.TokenManager
	geoResolver   *geo.BSSIDResolver
	privacyFilter *geo.ZeroBypassPrivacyFilter
}

func NewRouter(
	dfm *auth.DeviceFlowManager,
	bundles *auth.BundleRegistry,
	tokens *auth.TokenManager,
	geo *geo.BSSIDResolver,
	filter *geo.ZeroBypassPrivacyFilter,
) *Router {
	r := &Router{
		mux:           http.NewServeMux(),
		deviceFlow:    dfm,
		bundleReg:     bundles,
		tokenMgr:      tokens,
		geoResolver:   geo,
		privacyFilter: filter,
	}

	r.registerRoutes()
	return r
}

func (r *Router) Handler() http.Handler {
	return CORSMiddleware(r.mux)
}

func (r *Router) registerRoutes() {
	r.mux.HandleFunc("/api/v1/health", r.handleHealth)
	r.mux.HandleFunc("/api/v1/auth/session", r.handleSession)
	r.mux.HandleFunc("/api/v1/auth/device/code", r.handleDeviceCode)
	r.mux.HandleFunc("/api/v1/auth/device/token", r.handleDeviceToken)
	r.mux.HandleFunc("/api/v1/auth/device/verify", r.handleDeviceVerify)
	r.mux.HandleFunc("/api/v1/auth/device/pending", r.handleDevicePending)
	r.mux.HandleFunc("/api/v1/auth/device/auto", r.handleDeviceAuto)
	r.mux.HandleFunc("/api/v1/crypto/bundle", r.handleBundleUpload)
	r.mux.HandleFunc("/api/v1/crypto/bundle/", r.handleBundleFetch)
	r.mux.HandleFunc("/api/v1/geo/resolve", r.handleGeoResolve)
	r.mux.HandleFunc("/", r.handleStaticOrSPA)
}

func (r *Router) handleStaticOrSPA(w http.ResponseWriter, req *http.Request) {
	// Guard: if request path starts with /api/ or /ws/, it's an unhandled API endpoint
	if strings.HasPrefix(req.URL.Path, "/api/") || strings.HasPrefix(req.URL.Path, "/ws/") {
		http.NotFound(w, req)
		return
	}

	staticDir := "web/dist"
	absStaticDir, err := filepath.Abs(staticDir)
	if err != nil {
		http.Error(w, "Server directory resolution error", http.StatusInternalServerError)
		return
	}

	// Sanitize and defend against directory traversal attacks
	cleanPath := strings.TrimLeft(filepath.Clean(req.URL.Path), "/\\")
	targetPath := filepath.Join(absStaticDir, cleanPath)

	// Ensure targetPath resides strictly within absStaticDir
	rel, err := filepath.Rel(absStaticDir, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "..") {
		http.Error(w, "Access Denied: Path traversal detected", http.StatusForbidden)
		return
	}

	// Check if target file exists and is not a directory
	info, err := os.Stat(targetPath)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, req, targetPath)
		return
	}

	// Fallback to index.html for client-side SPA routing
	indexPath := filepath.Join(absStaticDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, req, indexPath)
		return
	}

	// If web/dist is not built yet, display a clean welcome page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>CampusNet Ephemeral Gateway</title></head>
<body style="font-family:system-ui,sans-serif;background:#0d1117;color:#c9d1d9;padding:40px;text-align:center;">
  <h1 style="color:#10b981;">⚡ CampusNet Gateway Server v1.0.0</h1>
  <p>Zero-Knowledge Ephemeral Communications Relay is ACTIVE.</p>
  <p>API Health: <a style="color:#38bdf8;" href="/api/v1/health">/api/v1/health</a></p>
  <p>To serve the full React web portal here, run: <code>npm --prefix web run build</code></p>
</body>
</html>`))
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "campusnet-gateway",
		"version": "1.0.0",
	})
}

func (r *Router) handleSession(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		UserHandle string `json:"user_handle"`
		DeviceID   string `json:"device_id"`
		Platform   string `json:"platform"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)

	if body.UserHandle == "" {
		body.UserHandle = "alice"
	}
	body.UserHandle = strings.TrimPrefix(strings.ToLower(body.UserHandle), "@")

	if body.Platform == "" {
		body.Platform = "web"
	}
	if body.DeviceID == "" {
		body.DeviceID = fmt.Sprintf("dev-%s-%x", body.Platform, time.Now().UnixNano()%0xFFFFFF)
	}

	token, err := r.tokenMgr.GenerateDeviceToken(body.DeviceID, body.UserHandle, body.Platform)
	if err != nil {
		http.Error(w, `{"error":"failed to generate session token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.DeviceTokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   86400 * 30,
		DeviceID:    body.DeviceID,
		UserHandle:  body.UserHandle,
	})
}

func (r *Router) handleDeviceCode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body protocol.DeviceCodeRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	resp, err := r.deviceFlow.RequestCode(&body)
	if err != nil {
		http.Error(w, `{"error":"failed to generate device code"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (r *Router) handleDeviceToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		DeviceCode string `json:"device_code"`
		ClientID   string `json:"client_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	resp, err := r.deviceFlow.PollToken(body.DeviceCode, body.ClientID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (r *Router) handleDeviceVerify(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		UserCode   string `json:"user_code"`
		UserHandle string `json:"user_handle"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	if err := r.deviceFlow.VerifyUserCode(body.UserCode, body.UserHandle); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "authorized"})
}

func (r *Router) handleDevicePending(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions := r.deviceFlow.GetPendingSessions()
	type pendingView struct {
		UserCode  string `json:"user_code"`
		Platform  string `json:"platform"`
		ClientID  string `json:"client_id"`
		CreatedAt int64  `json:"created_at"`
		ExpiresIn int64  `json:"expires_in"`
	}

	list := make([]pendingView, 0)
	now := time.Now()
	for _, s := range sessions {
		rem := s.ExpiresAt.Sub(now).Seconds()
		if rem > 0 {
			list = append(list, pendingView{
				UserCode:  s.UserCode,
				Platform:  s.Platform,
				ClientID:  s.ClientID,
				CreatedAt: s.CreatedAt.Unix(),
				ExpiresIn: int64(rem),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pending_devices": list,
		"pending":         list,
		"count":           len(list),
	})
}

func (r *Router) handleDeviceAuto(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		UserHandle string `json:"user_handle"`
		UserCode   string `json:"user_code"`
		Platform   string `json:"platform"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)

	if body.UserHandle == "" {
		body.UserHandle = "cli_user"
	}
	body.UserHandle = strings.TrimPrefix(strings.ToLower(body.UserHandle), "@")

	// 1. If a specific user_code is provided, verify it directly
	if body.UserCode != "" {
		if err := r.deviceFlow.VerifyUserCode(body.UserCode, body.UserHandle); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "authorized",
			"user_code":   body.UserCode,
			"user_handle": body.UserHandle,
		})
		return
	}

	// 2. If no user_code, try to auto-authorize the latest pending device session
	session, err := r.deviceFlow.AutoAuthorizeLatest(body.UserHandle)
	if err == nil && session != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "authorized",
			"user_code":   session.UserCode,
			"user_handle": session.UserHandle,
			"device_id":   session.DeviceID,
		})
		return
	}

	// 3. If no pending session, mint an immediate session (instant auto-connect)
	if body.Platform == "" {
		body.Platform = "cli"
	}
	deviceID := fmt.Sprintf("dev-%s-%x", body.Platform, time.Now().UnixNano()%0xFFFFFF)
	token, err := r.tokenMgr.GenerateDeviceToken(deviceID, body.UserHandle, body.Platform)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.DeviceTokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   86400 * 30,
		DeviceID:    deviceID,
		UserHandle:  body.UserHandle,
	})
}

func (r *Router) handleBundleUpload(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate client to prevent MITM key hijacking attacks
	tokenStr := req.URL.Query().Get("token")
	if tokenStr == "" {
		authHeader := req.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	var claims *auth.DeviceClaims
	if r.tokenMgr != nil {
		if tokenStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization token required to register pre-key bundle"})
			return
		}
		var err error
		claims, err = r.tokenMgr.ValidateToken(tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired authorization token"})
			return
		}
	}

	var bundle protocol.PreKeyBundle
	if err := json.NewDecoder(req.Body).Decode(&bundle); err != nil {
		http.Error(w, `{"error":"invalid bundle payload"}`, http.StatusBadRequest)
		return
	}

	if bundle.DeviceID == "" && claims != nil {
		bundle.DeviceID = claims.DeviceID
	}
	if bundle.UserHandle == "" && claims != nil {
		bundle.UserHandle = claims.UserHandle
	}

	// Prevent user impersonation: verified user must match bundle user handle
	if claims != nil {
		cleanClaim := strings.TrimPrefix(strings.ToLower(claims.UserHandle), "@")
		cleanBundle := strings.TrimPrefix(strings.ToLower(bundle.UserHandle), "@")
		cleanClaim = strings.TrimPrefix(cleanClaim, "student_")
		cleanBundle = strings.TrimPrefix(cleanBundle, "student_")

		if cleanClaim != cleanBundle {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden: cannot register bundle for another user account"})
			return
		}
	}

	if strings.TrimSpace(bundle.DeviceID) == "" || strings.TrimSpace(bundle.IdentityKey) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "device_id and identity_key are required"})
		return
	}

	if err := r.bundleReg.RegisterBundle(&bundle); err != nil {
		http.Error(w, `{"error":"failed to register bundle"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "bundle_registered"})
}

func (r *Router) handleBundleFetch(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := strings.TrimPrefix(req.URL.Path, "/api/v1/crypto/bundle/")
	if target == "" {
		http.Error(w, `{"error":"missing target device id or handle"}`, http.StatusBadRequest)
		return
	}

	bundle, err := r.bundleReg.FetchBundle(target)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bundle)
}

func (r *Router) handleGeoResolve(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload protocol.PresencePayload
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"invalid geo request payload"}`, http.StatusBadRequest)
		return
	}

	sanitized := r.privacyFilter.SanitizePresence(payload.UserHandle, &payload)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sanitized)
}
