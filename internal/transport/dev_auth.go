//go:build devauth

package transport

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/hit-chat/campusnet/internal/crypto"
)

type DevLoginRequest struct {
	Handle string `json:"handle"`
}

type DevLoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	UserHandle  string `json:"user_handle"`
	DeviceID    string `json:"device_id"`
}

// registerDevAuthRoutes registers the dev login endpoint active only under devauth build tag.
func (r *Router) registerDevAuthRoutes() {
	r.mux.HandleFunc("/api/v1/dev/login", r.handleDevLogin)
}

func (r *Router) handleDevLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Restrict to local loopback and private networks
	if !isPrivateNetwork(req) {
		http.Error(w, "Forbidden: Dev auth is only accessible from private network / loopback", http.StatusForbidden)
		return
	}

	// Must be explicitly enabled at server launch
	if !r.devAuthEnabled {
		http.Error(w, "Dev auth is disabled on this server instance (launch with --dev-auth)", http.StatusForbidden)
		return
	}

	limitRequestBody(w, req)
	var body DevLoginRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	cleanHandle := crypto.CanonicalHandle(body.Handle)
	if cleanHandle == "" {
		cleanHandle = "student_guest"
	}

	// Deterministic / random device ID for dev session
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	deviceID := "dev-web-" + cleanHandle + "-" + hex.EncodeToString(b)

	token, err := r.tokenMgr.GenerateDeviceToken(deviceID, cleanHandle, "web")
	if err != nil {
		http.Error(w, "Failed to generate dev token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := DevLoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		UserHandle:  cleanHandle,
		DeviceID:    deviceID,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func isPrivateNetwork(r *http.Request) bool {
	if isLoopback(r) {
		return true
	}
	host := r.RemoteAddr
	if strings.Contains(host, ":") {
		h, _, err := net.SplitHostPort(host)
		if err == nil {
			host = h
		}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback()
}
