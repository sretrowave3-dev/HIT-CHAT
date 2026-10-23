package ephemeral

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/hit-chat/campusnet/internal/crypto"
	"github.com/hit-chat/campusnet/pkg/protocol"
)

var (
	ErrStoreFull              = errors.New("ephemeral store capacity reached")
	ErrRecipientQuotaExceeded = errors.New("recipient unread envelope quota exceeded")
)

type memoryRecord struct {
	env       *protocol.Envelope
	expiresAt time.Time
}

// MemoryStore provides a thread-safe, in-memory ephemeral store with active TTL sweeps.
type MemoryStore struct {
	mu                    sync.RWMutex
	items                 map[string]memoryRecord
	stopCh                chan struct{}
	maxTotalEnvelopes     int
	maxRecipientEnvelopes int
}

// NewMemoryStore initializes an in-memory ephemeral store and launches an eviction ticker.
func NewMemoryStore(sweepInterval time.Duration) *MemoryStore {
	if sweepInterval <= 0 {
		sweepInterval = 500 * time.Millisecond
	}

	ms := &MemoryStore{
		items:                 make(map[string]memoryRecord),
		stopCh:                make(chan struct{}),
		maxTotalEnvelopes:     10000,
		maxRecipientEnvelopes: 100,
	}

	go ms.startSweeper(sweepInterval)
	return ms
}

// SetQuotas updates the memory bounds and per-recipient limits.
func (m *MemoryStore) SetQuotas(maxTotal, maxRecipient int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if maxTotal > 0 {
		m.maxTotalEnvelopes = maxTotal
	}
	if maxRecipient > 0 {
		m.maxRecipientEnvelopes = maxRecipient
	}
}

func (m *MemoryStore) StoreEnvelope(ctx context.Context, env *protocol.Envelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 1. Check global capacity bounds; evict expired inline if near threshold
	if len(m.items) >= m.maxTotalEnvelopes {
		for id, rec := range m.items {
			if now.After(rec.expiresAt) {
				delete(m.items, id)
			}
		}
		if len(m.items) >= m.maxTotalEnvelopes {
			return ErrStoreFull
		}
	}

	// 2. Check per-recipient pending unread quota to prevent inbox flooding DoS
	cleanRecipient := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(env.RecipientID)), "@")
	cleanRecipient = strings.TrimPrefix(cleanRecipient, "student_")

	recipientCount := 0
	for _, rec := range m.items {
		if now.Before(rec.expiresAt) && rec.env != nil {
			target := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(rec.env.RecipientID)), "@")
			target = strings.TrimPrefix(target, "student_")
			if target == cleanRecipient {
				recipientCount++
			}
		}
	}

	if recipientCount >= m.maxRecipientEnvelopes {
		return ErrRecipientQuotaExceeded
	}

	ttl := time.Duration(env.TTLMilliseconds) * time.Millisecond
	m.items[env.ID] = memoryRecord{
		env:       env,
		expiresAt: now.Add(ttl),
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
