package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hit-chat/campusnet/internal/auth"
	"github.com/hit-chat/campusnet/internal/geo"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

func setupTestRouter() (*Router, *auth.TokenManager, *auth.DeviceFlowManager) {
	tokenMgr := auth.NewTokenManager("test-secret-key-32-chars-long!!")
	dfm := auth.NewDeviceFlowManager(tokenMgr, "http://localhost:8080/pair")
	bundleReg := auth.NewBundleRegistry()
	geoRes := geo.NewBSSIDResolver("test-salt")
	privacyFilter := geo.NewPrivacyFilter(geoRes)
	router := NewRouter(dfm, bundleReg, tokenMgr, geoRes, privacyFilter)
	return router, tokenMgr, dfm
}

func TestPendingDeviceEnumerationProtection(t *testing.T) {
	router, tokenMgr, dfm := setupTestRouter()

	// Create a pending session for Charlie
	_, err := dfm.RequestCode(&protocol.DeviceCodeRequest{
		ClientID: "test-cli",
		Platform: "cli",
		UserHandle: "charlie",
	})
	if err != nil {
		t.Fatalf("failed to request code: %v", err)
	}

	// 1. Remote request without authentication MUST be rejected with 401
	remoteReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/device/pending", nil)
	remoteReq.RemoteAddr = "192.168.1.100:54321" // Remote IP
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, remoteReq)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for remote pending enumeration, got %d", rec.Code)
	}

	// 2. Remote request with authenticated token for Alice should NOT see Charlie's code
	aliceToken, _ := tokenMgr.GenerateDeviceToken("dev-alice", "alice", "web")
	authReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/device/pending", nil)
	authReq.RemoteAddr = "192.168.1.100:54321"
	authReq.Header.Set("Authorization", "Bearer "+aliceToken)
	authRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(authRec, authReq)

	if authRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for authenticated user, got %d", authRec.Code)
	}

	var data struct {
		Pending []struct {
			UserCode string `json:"user_code"`
		} `json:"pending"`
		Count int `json:"count"`
	}
	_ = json.Unmarshal(authRec.Body.Bytes(), &data)
	if data.Count != 0 {
		t.Fatalf("security violation: Alice was able to see Charlie's pending device code!")
	}

	// 3. Local loopback request is permitted for local CLI auto-connect
	localReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/device/pending", nil)
	localReq.RemoteAddr = "127.0.0.1:54321"
	localRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(localRec, localReq)

	if localRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for loopback, got %d", localRec.Code)
	}
}

func TestRemoteSessionMintingProhibition(t *testing.T) {
	router, _, _ := setupTestRouter()

	// Remote attacker attempts to mint a token for victim Alice via /api/v1/auth/session
	body := map[string]string{
		"user_handle": "alice",
		"platform": "web",
	}
	bBytes, _ := json.Marshal(body)

	remoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session", bytes.NewReader(bBytes))
	remoteReq.RemoteAddr = "203.0.113.50:43210" // Remote untrusted IP
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, remoteReq)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for remote unauthenticated session minting, got %d", rec.Code)
	}
}

func TestCSWSHAndCORSOriginVerification(t *testing.T) {
	// 1. WebSocket origin checks
	evilReq := httptest.NewRequest(http.MethodGet, "/ws/v1/stream", nil)
	evilReq.Header.Set("Origin", "http://evil-phishing-site.com")
	if isOriginAllowed(evilReq) {
		t.Fatalf("security violation: isOriginAllowed accepted evil-phishing-site.com!")
	}

	goodLocalReq := httptest.NewRequest(http.MethodGet, "/ws/v1/stream", nil)
	goodLocalReq.Header.Set("Origin", "http://localhost:5173")
	if !isOriginAllowed(goodLocalReq) {
		t.Fatalf("expected isOriginAllowed to accept http://localhost:5173")
	}

	cliReq := httptest.NewRequest(http.MethodGet, "/ws/v1/stream", nil)
	if !isOriginAllowed(cliReq) {
		t.Fatalf("expected isOriginAllowed to accept native CLI without Origin header")
	}

	// 2. CORS and Security Headers
	router, _, _ := setupTestRouter()
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, evilReq)

	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Fatalf("security violation: CORS middleware emitted wildcard '*' to untrusted origin!")
	}

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing X-Content-Type-Options: nosniff security header")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing X-Frame-Options: DENY clickjacking protection header")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("missing Content-Security-Policy security header")
	}
}

func TestRequestBodySizeLimit(t *testing.T) {
	router, _, _ := setupTestRouter()

	// Create 2MB payload (exceeds 1MB limit)
	hugePayload := make([]byte, 2*1024*1024)
	for i := range hugePayload {
		hugePayload[i] = 'A'
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session", bytes.NewReader(hugePayload))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	// MaxBytesReader causes decoder error and rejects with 400 or 413
	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 413 or 400 for oversized payload, got %d", rec.Code)
	}
}


func TestStaticPathTraversalProtection(t *testing.T) {
	router, _, _ := setupTestRouter()

	traversalProbes := []string{
		"/../../go.mod",
		"/../go.mod",
		"/assets/../../go.mod",
		"/static/..%2f..%2fgo.mod",
		"/..%2f..%2fsecret",
		"/..\\..\\windows\\win.ini",
	}

	for _, probe := range traversalProbes {
		req := httptest.NewRequest(http.MethodGet, probe, nil)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("probe %q: expected 403 Forbidden, got %d", probe, rec.Code)
		}
	}

	// Verify valid SPA route still returns 200 OK
	spaReq := httptest.NewRequest(http.MethodGet, "/chat", nil)
	spaRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(spaRec, spaReq)
	if spaRec.Code != http.StatusOK {
		t.Fatalf("expected valid SPA route /chat to return 200 OK, got %d", spaRec.Code)
	}
}
