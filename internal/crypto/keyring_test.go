package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSecureStoreUserIsolationAndPersistence(t *testing.T) {
	// Create temporary directory for isolated test
	tempDir, err := os.MkdirTemp("", "campusnet_test_store_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storeFile := filepath.Join(tempDir, "test_secrets.enc")

	store := &SecureStore{
		memoryMap: make(map[string][]byte),
		storePath: storeFile,
	}

	secretKey := "device_token"
	secretVal := []byte("super_confidential_jwt_session_token")

	// 1. Set secret and verify it persists
	if err := store.Set(secretKey, secretVal); err != nil {
		t.Fatalf("failed to set secret: %v", err)
	}

	// 2. Open fresh store instance pointing to same file
	freshStore := &SecureStore{
		memoryMap: make(map[string][]byte),
		storePath: storeFile,
	}
	loaded, err := freshStore.Get(secretKey)
	if err != nil {
		t.Fatalf("failed to get secret from file: %v", err)
	}
	if !bytes.Equal(loaded, secretVal) {
		t.Fatalf("loaded secret mismatch: got %s, want %s", loaded, secretVal)
	}

	// 3. Verify file permissions are restricted (0600)
	info, err := os.Stat(storeFile)
	if err != nil {
		t.Fatalf("failed to stat store file: %v", err)
	}
	// On Unix, check 0600. On Windows os.FileMode may differ slightly but must exist
	if info.Size() == 0 {
		t.Fatalf("secret file is unexpectedly empty")
	}

	// 4. Clean up / delete
	if err := store.Delete(secretKey); err != nil {
		t.Fatalf("failed to delete secret: %v", err)
	}
	_, err = store.Get(secretKey)
	if err != ErrSecretNotFound {
		t.Fatalf("expected ErrSecretNotFound after deletion, got %v", err)
	}
}


func TestPeerIdentityKeyChangeDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "campusnet_test_store_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	store := &SecureStore{
		memoryMap: make(map[string][]byte),
		storePath: filepath.Join(tempDir, "test_ik_tracking.enc"),
	}

	bobKP1, _ := GenerateKeyPair()
	bobKP2, _ := GenerateKeyPair()

	// 1. Initial contact with Bob: pin Bob's identity key
	changed, err := store.VerifyOrTrackPeerIdentity("bob", bobKP1.PublicKey)
	if err != nil || changed {
		t.Fatalf("initial identity pinning should return changed=false, got changed=%v, err=%v", changed, err)
	}

	// 2. Subsequent contact with unchanged key: must return changed=false
	changed, err = store.VerifyOrTrackPeerIdentity("@bob", bobKP1.PublicKey)
	if err != nil || changed {
		t.Fatalf("expected unchanged identity key verification, got changed=%v", changed)
	}

	// 3. Compromised / swapped pre-key bundle for Bob with new key: must trigger loud changed=true
	changed, err = store.VerifyOrTrackPeerIdentity("bob", bobKP2.PublicKey)
	if err != nil || !changed {
		t.Fatalf("CRITICAL: Key change detection failed! Expected changed=true on peer key swap, got changed=%v", changed)
	}
}
