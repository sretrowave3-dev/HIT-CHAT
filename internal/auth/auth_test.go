package auth

import (
	"strings"
	"testing"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

func TestDeviceAuthorizationFlow(t *testing.T) {
	tokenMgr := NewTokenManager("test-secret-key-32-chars-campus!")
	dfm := NewDeviceFlowManager(tokenMgr, "http://localhost:8080/pair")

	req := &protocol.DeviceCodeRequest{
		ClientID:          "cli-device-101",
		Platform:          "cli",
		PublicIdentityKey: "base64pubkey==",
	}

	// 1. Request code
	resp, err := dfm.RequestCode(req)
	if err != nil {
		t.Fatalf("RequestCode failed: %v", err)
	}

	if !strings.HasPrefix(resp.UserCode, "CNET-") {
		t.Fatalf("expected user code format CNET-XXXX-XXXX, got %s", resp.UserCode)
	}

	// 2. Poll token before approval -> should return ErrDevicePending
	_, err = dfm.PollToken(resp.DeviceCode, req.ClientID)
	if err != protocol.ErrDevicePending {
		t.Fatalf("expected ErrDevicePending, got: %v", err)
	}

	// 3. User verifies user code on web portal
	err = dfm.VerifyUserCode(resp.UserCode, "student_alice")
	if err != nil {
		t.Fatalf("VerifyUserCode failed: %v", err)
	}

	// 4. Poll token after approval -> should succeed with valid JWT
	tokenResp, err := dfm.PollToken(resp.DeviceCode, req.ClientID)
	if err != nil {
		t.Fatalf("PollToken after authorization failed: %v", err)
	}

	if tokenResp.AccessToken == "" {
		t.Fatal("empty access token returned")
	}

	// 5. Validate token claims
	claims, err := tokenMgr.ValidateToken(tokenResp.AccessToken)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}

	if claims.UserHandle != "student_alice" {
		t.Fatalf("claims userHandle mismatch: got %s, want student_alice", claims.UserHandle)
	}
}

func TestPreKeyBundleRegistry(t *testing.T) {
	reg := NewBundleRegistry()

	bundle := &protocol.PreKeyBundle{
		DeviceID:        "dev-123",
		UserHandle:      "student_bob",
		IdentityKey:     "ik_pub_key",
		SignedPreKey:    "spk_pub_key",
		SignedPreKeySig: "spk_signature",
		OneTimePreKeys:  []string{"otk_1", "otk_2"},
	}

	if err := reg.RegisterBundle(bundle); err != nil {
		t.Fatalf("failed to register bundle: %v", err)
	}

	// Fetch 1: should return otk_1
	f1, err := reg.FetchBundle("student_bob")
	if err != nil {
		t.Fatalf("fetch 1 failed: %v", err)
	}
	if f1.SelectedOneTimePreKey != "otk_1" {
		t.Fatalf("expected otk_1, got %s", f1.SelectedOneTimePreKey)
	}

	// Fetch 2: should return otk_2
	f2, err := reg.FetchBundle("student_bob")
	if err != nil {
		t.Fatalf("fetch 2 failed: %v", err)
	}
	if f2.SelectedOneTimePreKey != "otk_2" {
		t.Fatalf("expected otk_2, got %s", f2.SelectedOneTimePreKey)
	}

	// Fetch 3: OTKs exhausted, returns empty selected OTK
	f3, err := reg.FetchBundle("student_bob")
	if err != nil {
		t.Fatalf("fetch 3 failed: %v", err)
	}
	if f3.SelectedOneTimePreKey != "" {
		t.Fatalf("expected empty OTK, got %s", f3.SelectedOneTimePreKey)
	}
}
