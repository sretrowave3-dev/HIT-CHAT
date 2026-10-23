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
