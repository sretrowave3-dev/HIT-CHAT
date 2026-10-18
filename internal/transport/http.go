package transport

import (
	"encoding/json"
	"net/http"
	"strings"

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
	r.mux.HandleFunc("/api/v1/auth/device/code", r.handleDeviceCode)
	r.mux.HandleFunc("/api/v1/auth/device/token", r.handleDeviceToken)
	r.mux.HandleFunc("/api/v1/auth/device/verify", r.handleDeviceVerify)
	r.mux.HandleFunc("/api/v1/crypto/bundle", r.handleBundleUpload)
	r.mux.HandleFunc("/api/v1/crypto/bundle/", r.handleBundleFetch)
	r.mux.HandleFunc("/api/v1/geo/resolve", r.handleGeoResolve)
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "campusnet-gateway",
		"version": "1.0.0",
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

func (r *Router) handleBundleUpload(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var bundle protocol.PreKeyBundle
	if err := json.NewDecoder(req.Body).Decode(&bundle); err != nil {
		http.Error(w, `{"error":"invalid bundle payload"}`, http.StatusBadRequest)
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
