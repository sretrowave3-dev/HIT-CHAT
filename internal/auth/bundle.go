package auth

import (
	"errors"
	"sync"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

var (
	ErrDeviceBundleExists = errors.New("device bundle already exists")
)

type storedBundle struct {
	DeviceID        string
	UserHandle      string
	IdentityKey     string
	SignedPreKey    string
	SignedPreKeySig string
	OTKs            []string
	ConsumedOTKs    map[string]bool
}

// BundleRegistry maintains public pre-key bundles in-memory or persisted via PostgreSQL.
// ZERO-KNOWLEDGE INVARIANT: Registry stores only public keys and signatures.
type BundleRegistry struct {
	mu           sync.RWMutex
	bundles      map[string]*storedBundle // deviceID -> bundle
	handleToDevs map[string][]string      // userHandle -> []deviceID
}

func NewBundleRegistry() *BundleRegistry {
	return &BundleRegistry{
		bundles:      make(map[string]*storedBundle),
		handleToDevs: make(map[string][]string),
	}
}

// RegisterBundle registers or updates the public keys for a device.
func (b *BundleRegistry) RegisterBundle(bundle *protocol.PreKeyBundle) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	sb := &storedBundle{
		DeviceID:        bundle.DeviceID,
		UserHandle:      bundle.UserHandle,
		IdentityKey:     bundle.IdentityKey,
		SignedPreKey:    bundle.SignedPreKey,
		SignedPreKeySig: bundle.SignedPreKeySig,
		OTKs:            make([]string, len(bundle.OneTimePreKeys)),
		ConsumedOTKs:    make(map[string]bool),
	}
	copy(sb.OTKs, bundle.OneTimePreKeys)

	b.bundles[bundle.DeviceID] = sb

	// Map handle
	if bundle.UserHandle != "" {
		found := false
		for _, did := range b.handleToDevs[bundle.UserHandle] {
			if did == bundle.DeviceID {
				found = true
				break
			}
		}
		if !found {
			b.handleToDevs[bundle.UserHandle] = append(b.handleToDevs[bundle.UserHandle], bundle.DeviceID)
		}
	}

	return nil
}

// FetchBundle retrieves a public pre-key bundle for X3DH handshake, popping one OTK.
func (b *BundleRegistry) FetchBundle(targetID string) (*protocol.PreKeyBundle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Target can be either deviceID or @userHandle
	var sb *storedBundle
	if dev, ok := b.bundles[targetID]; ok {
		sb = dev
	} else if devs, ok := b.handleToDevs[targetID]; ok && len(devs) > 0 {
		sb = b.bundles[devs[0]] // Return primary device
	}

	if sb == nil {
		return nil, protocol.ErrBundleNotFound
	}

	// Pop an unconsumed OTK if available
	var selectedOTK string
	for _, otk := range sb.OTKs {
		if !sb.ConsumedOTKs[otk] {
			sb.ConsumedOTKs[otk] = true
			selectedOTK = otk
			break
		}
	}

	return &protocol.PreKeyBundle{
		DeviceID:              sb.DeviceID,
		UserHandle:            sb.UserHandle,
		IdentityKey:           sb.IdentityKey,
		SignedPreKey:          sb.SignedPreKey,
		SignedPreKeySig:       sb.SignedPreKeySig,
		SelectedOneTimePreKey: selectedOTK,
	}, nil
}
