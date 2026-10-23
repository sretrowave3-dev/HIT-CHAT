package ephemeral

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hit-chat/campusnet/internal/crypto"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

type memoryRecord struct {
	env       *protocol.Envelope
	expiresAt time.Time
}

// MemoryStore provides a thread-safe, in-memory ephemeral store with active TTL sweeps.
type MemoryStore struct {
	mu     sync.RWMutex
	items  map[string]memoryRecord
	stopCh chan struct{}
}

// NewMemoryStore initializes an in-memory ephemeral store and launches an eviction ticker.
func NewMemoryStore(sweepInterval time.Duration) *MemoryStore {
	if sweepInterval <= 0 {
		sweepInterval = 500 * time.Millisecond
	}

	ms := &MemoryStore{
		items:  make(map[string]memoryRecord),
		stopCh: make(chan struct{}),
	}

	go ms.startSweeper(sweepInterval)
	return ms
}

func (m *MemoryStore) StoreEnvelope(ctx context.Context, env *protocol.Envelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ttl := time.Duration(env.TTLMilliseconds) * time.Millisecond
	m.items[env.ID] = memoryRecord{
		env:       env,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (m *MemoryStore) GetEnvelope(ctx context.Context, id string) (*protocol.Envelope, error) {
	m.mu.RLock()
	rec, ok := m.items[id]
	m.mu.RUnlock()

	if !ok || time.Now().After(rec.expiresAt) {
		return nil, ErrEnvelopeNotFound
	}
	return rec.env, nil
}

func (m *MemoryStore) GetPendingEnvelopesForRecipient(ctx context.Context, recipient string) ([]*protocol.Envelope, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	cleanRecipient := strings.TrimPrefix(strings.ToLower(recipient), "@")
	var result []*protocol.Envelope

	for _, rec := range m.items {
		if now.Before(rec.expiresAt) && rec.env != nil {
			target := strings.TrimPrefix(strings.ToLower(rec.env.RecipientID), "@")
			if target == cleanRecipient || target == "student_"+cleanRecipient || cleanRecipient == "student_"+target {
				result = append(result, rec.env)
			}
		}
	}
	return result, nil
}

func (m *MemoryStore) BurnEnvelope(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.items[id]
	if !ok {
		return nil
	}

	// Zero-fill sensitive ciphertext in memory before removing pointer
	if rec.env != nil {
		crypto.ZeroBytes(rec.env.Ciphertext)
		crypto.ZeroBytes(rec.env.AuthTag)
		crypto.ZeroBytes(rec.env.IV)
		crypto.ZeroBytes(rec.env.EphemeralPublicKey)
	}

	delete(m.items, id)
	return nil
}

func (m *MemoryStore) startSweeper(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.sweep()
		}
	}
}

func (m *MemoryStore) sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, rec := range m.items {
		if now.After(rec.expiresAt) {
			if rec.env != nil {
				crypto.ZeroBytes(rec.env.Ciphertext)
				crypto.ZeroBytes(rec.env.AuthTag)
			}
			delete(m.items, id)
		}
	}
}

func (m *MemoryStore) Close() error {
	close(m.stopCh)
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rec := range m.items {
		if rec.env != nil {
			crypto.ZeroBytes(rec.env.Ciphertext)
		}
		delete(m.items, id)
	}
	return nil
}
