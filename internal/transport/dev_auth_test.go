//go:build devauth

package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDevAuthLoginHandler(t *testing.T) {
	router, tokenMgr, _ := setupTestRouter()

	// 1. With devAuthEnabled = false -> 403 Forbidden
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login", bytes.NewBufferString(`{"handle":"bob"}`))
	req1.RemoteAddr = "127.0.0.1:54321"
	rec1 := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when dev-auth is disabled, got %d", rec1.Code)
	}

	// 2. Enable devAuth
	router.SetDevAuthEnabled(true)

	// Remote public IP -> 403 Forbidden
	reqPublic := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login", bytes.NewBufferString(`{"handle":"bob"}`))
	reqPublic.RemoteAddr = "93.184.216.34:54321"
	recPublic := httptest.NewRecorder()
	router.Handler().ServeHTTP(recPublic, reqPublic)
	if recPublic.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for public IP, got %d", recPublic.Code)
	}

	// Loopback IP -> 200 OK
	reqLocal := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login", bytes.NewBufferString(`{"handle":"bob"}`))
	reqLocal.RemoteAddr = "127.0.0.1:54321"
	recLocal := httptest.NewRecorder()
	router.Handler().ServeHTTP(recLocal, reqLocal)
	if recLocal.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for local dev login, got %d: %s", recLocal.Code, recLocal.Body.String())
	}

	var resp DevLoginResponse
	if err := json.Unmarshal(recLocal.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	if resp.UserHandle != "bob" {
		t.Errorf("expected user handle 'bob', got %q", resp.UserHandle)
	}
	if resp.AccessToken == "" {
		t.Errorf("expected non-empty access token")
	}

	// Verify token is valid and parses to bob
	claims, err := tokenMgr.ValidateToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("generated dev token is invalid: %v", err)
	}
	if claims.UserHandle != "bob" {
		t.Errorf("claims user handle mismatch: expected 'bob', got %q", claims.UserHandle)
	}

	// Private LAN IP -> 200 OK
	reqLAN := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login", bytes.NewBufferString(`{"handle":"@carol"}`))
	reqLAN.RemoteAddr = "192.168.1.55:54321"
	recLAN := httptest.NewRecorder()
	router.Handler().ServeHTTP(recLAN, reqLAN)
	if recLAN.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for private LAN IP, got %d", recLAN.Code)
	}
	var carolResp DevLoginResponse
	_ = json.Unmarshal(recLAN.Body.Bytes(), &carolResp)
	if carolResp.UserHandle != "carol" {
		t.Errorf("expected normalized handle 'carol', got %q", carolResp.UserHandle)
	}
}
