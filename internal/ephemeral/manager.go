package ephemeral

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

var (
	ErrEnvelopeNotFound = errors.New("envelope not found, already burned, or expired")
	ErrUnauthorizedBurn = errors.New("unauthorized: caller is not the recipient or sender of this envelope")
)

// Store defines the storage operations required for ephemeral E2EE message lifecycles.
type Store interface {
	StoreEnvelope(ctx context.Context, env *protocol.Envelope) error
	GetEnvelope(ctx context.Context, id string) (*protocol.Envelope, error)
	GetPendingEnvelopesForRecipient(ctx context.Context, recipient string) ([]*protocol.Envelope, error)
	BurnEnvelope(ctx context.Context, id string) error
	Close() error
}

// Manager coordinates the ephemeral lifecycle, burn-on-read triggers, and expiration sweeps.
type Manager struct {
	store Store

	mu        sync.RWMutex
	listeners map[string][]chan string // messageID -> notification channels
}

// NewManager creates a new Ephemerality Manager.
func NewManager(store Store) *Manager {
	return &Manager{
		store:     store,
		listeners: make(map[string][]chan string),
	}
}

// SaveEnvelope persists the envelope in the ephemeral store with its defined millisecond TTL.
func (m *Manager) SaveEnvelope(ctx context.Context, env *protocol.Envelope) error {
	if env == nil || env.ID == "" {
		return errors.New("cannot store nil or unindexed envelope")
	}
	if env.IsExpired() {
		return protocol.ErrEnvelopeExpired
	}

	return m.store.StoreEnvelope(ctx, env)
}

// RetrieveEnvelope retrieves the envelope if it is still valid and not burned.
func (m *Manager) RetrieveEnvelope(ctx context.Context, id string) (*protocol.Envelope, error) {
	env, err := m.store.GetEnvelope(ctx, id)
	if err != nil {
		return nil, ErrEnvelopeNotFound
	}
	if env.IsExpired() {
		_ = m.store.BurnEnvelope(ctx, id)
		return nil, ErrEnvelopeNotFound
	}
	return env, nil
}

// GetPendingForRecipient retrieves all unexpired pending envelopes destined for recipient.
func (m *Manager) GetPendingForRecipient(ctx context.Context, recipient string) ([]*protocol.Envelope, error) {
	return m.store.GetPendingEnvelopesForRecipient(ctx, recipient)
}

// BurnOnRead triggers an immediate cryptographic unlink and zeroing of the envelope.
// It also notifies any listeners awaiting confirmation of burn.
func (m *Manager) BurnOnRead(ctx context.Context, id string) error {
	err := m.store.BurnEnvelope(ctx, id)

	m.mu.Lock()
	chans, exists := m.listeners[id]
	if exists {
		delete(m.listeners, id)
	}
	m.mu.Unlock()

	if exists {
		for _, ch := range chans {
			select {
			case ch <- id:
			default:
			}
		}
	}

	if err != nil {
		return fmt.Errorf("burn-on-read failed: %w", err)
	}
	return nil
}

// BurnOnReadForUser verifies authorization before burning the envelope.
// IDOR DEFENSE: Only the legitimate recipient or sender may burn the envelope.
func (m *Manager) BurnOnReadForUser(ctx context.Context, id string, userHandle string) error {
	env, err := m.store.GetEnvelope(ctx, id)
	if err != nil {
		return ErrEnvelopeNotFound
	}

	cleanUser := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(userHandle)), "@")
	cleanUser = strings.TrimPrefix(cleanUser, "student_")

	cleanRecipient := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(env.RecipientID)), "@")
	cleanRecipient = strings.TrimPrefix(cleanRecipient, "student_")

	cleanSender := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(env.SenderID)), "@")
	cleanSender = strings.TrimPrefix(cleanSender, "student_")

	if cleanUser != cleanRecipient && cleanUser != cleanSender {
		return ErrUnauthorizedBurn
	}

	return m.BurnOnRead(ctx, id)
}

// WatchBurn registers a channel to be notified when a message is burned.
func (m *Manager) WatchBurn(id string) <-chan string {
	ch := make(chan string, 1)
	m.mu.Lock()
	m.listeners[id] = append(m.listeners[id], ch)
	m.mu.Unlock()
	return ch
}

// Close gracefully closes the underlying store.
func (m *Manager) Close() error {
	return m.store.Close()
}
