package ephemeral

import (
	"context"
	"testing"
	"time"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

func TestEphemeralMemoryStoreAndBurnOnRead(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(100 * time.Millisecond)
	mgr := NewManager(store)
	defer mgr.Close()

	env := protocol.NewEnvelope("alice", "bob", 500) // 500ms lifespan
	env.EphemeralPublicKey = make([]byte, 32)
	env.IV = make([]byte, 12)
	env.Ciphertext = []byte("encrypted_secret_stream")
	env.AuthTag = make([]byte, 16)

	// Save envelope
	if err := mgr.SaveEnvelope(ctx, env); err != nil {
		t.Fatalf("failed to save envelope: %v", err)
	}

	// Retrieve before burn
	retrieved, err := mgr.RetrieveEnvelope(ctx, env.ID)
	if err != nil {
		t.Fatalf("failed to retrieve envelope: %v", err)
	}
	if retrieved.ID != env.ID {
		t.Fatalf("retrieved ID mismatch: got %s, want %s", retrieved.ID, env.ID)
	}

	// Trigger burn-on-read
	if err := mgr.BurnOnRead(ctx, env.ID); err != nil {
		t.Fatalf("failed to burn envelope: %v", err)
	}

	// Second retrieve must fail with ErrEnvelopeNotFound
	_, err = mgr.RetrieveEnvelope(ctx, env.ID)
	if err == nil {
		t.Fatalf("expected error after burn-on-read, but envelope was still retrievable")
	}
}

func TestEphemeralTTLExpiration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(50 * time.Millisecond)
	mgr := NewManager(store)
	defer mgr.Close()

	// Short 50ms TTL
	env := protocol.NewEnvelope("alice", "bob", 50)
	env.EphemeralPublicKey = make([]byte, 32)
	env.IV = make([]byte, 12)
	env.Ciphertext = []byte("short_lived_secret")
	env.AuthTag = make([]byte, 16)

	if err := mgr.SaveEnvelope(ctx, env); err != nil {
		t.Fatal(err)
	}

	// Wait 120ms for expiration
	time.Sleep(120 * time.Millisecond)

	_, err := mgr.RetrieveEnvelope(ctx, env.ID)
	if err == nil {
		t.Fatalf("expected expired envelope to be purged, but it was found")
	}
}
