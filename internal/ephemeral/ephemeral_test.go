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

func TestBurnOnReadAuthorizationAndIDORPrevention(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(100 * time.Millisecond)
	mgr := NewManager(store)
	defer mgr.Close()

	env := protocol.NewEnvelope("alice", "bob", 10000)
	env.EphemeralPublicKey = make([]byte, 32)
	env.IV = make([]byte, 12)
	env.Ciphertext = []byte("confidential_for_bob")
	env.AuthTag = make([]byte, 16)

	if err := mgr.SaveEnvelope(ctx, env); err != nil {
		t.Fatalf("failed to save envelope: %v", err)
	}

	// Attacker eve attempts to burn Alice and Bob's message
	err := mgr.BurnOnReadForUser(ctx, env.ID, "eve")
	if err == nil {
		t.Fatalf("security violation: unauthorized user eve was able to burn Alice/Bob message!")
	}

	// Envelope MUST still be intact for legitimate recipient Bob
	retrieved, err := mgr.RetrieveEnvelope(ctx, env.ID)
	if err != nil || retrieved == nil {
		t.Fatalf("message was destroyed by attacker eve!")
	}

	// Legitimate recipient Bob burns on read -> must succeed
	err = mgr.BurnOnReadForUser(ctx, env.ID, "bob")
	if err != nil {
		t.Fatalf("legitimate recipient failed to burn message: %v", err)
	}

	// Now message is truly gone
	_, err = mgr.RetrieveEnvelope(ctx, env.ID)
	if err == nil {
		t.Fatalf("expected message to be gone after legitimate burn")
	}
}

func TestMemoryStoreCapacityAndQuotaEnforcement(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(100 * time.Millisecond)
	store.SetQuotas(10, 3) // max 10 total, max 3 per recipient
	mgr := NewManager(store)
	defer mgr.Close()

	// Fill Bob's quota of 3
	for i := 0; i < 3; i++ {
		env := protocol.NewEnvelope("alice", "bob", 10000)
		env.EphemeralPublicKey = make([]byte, 32)
		env.IV = make([]byte, 12)
		env.Ciphertext = []byte("msg")
		env.AuthTag = make([]byte, 16)
		if err := mgr.SaveEnvelope(ctx, env); err != nil {
			t.Fatalf("failed to save envelope %d: %v", i, err)
		}
	}

	// 4th envelope to Bob must be rejected due to quota limit
	overflowEnv := protocol.NewEnvelope("alice", "bob", 10000)
	overflowEnv.EphemeralPublicKey = make([]byte, 32)
	overflowEnv.IV = make([]byte, 12)
	overflowEnv.Ciphertext = []byte("overflow")
	overflowEnv.AuthTag = make([]byte, 16)
	err := mgr.SaveEnvelope(ctx, overflowEnv)
	if err != ErrRecipientQuotaExceeded {
		t.Fatalf("expected ErrRecipientQuotaExceeded, got: %v", err)
	}
}
