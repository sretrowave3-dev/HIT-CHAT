package auth

import (
	"errors"
	"strings"
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

	// Map handle and aliases
	if bundle.UserHandle != "" {
		clean := strings.TrimPrefix(strings.ToLower(bundle.UserHandle), "@")
		aliases := []string{clean}
		if strings.HasPrefix(clean, "student_") {
			aliases = append(aliases, strings.TrimPrefix(clean, "student_"))
		} else {
			aliases = append(aliases, "student_"+clean)
		}

		for _, alias := range aliases {
			var updated []string
			for _, did := range b.handleToDevs[alias] {
				if did != bundle.DeviceID {
					updated = append(updated, did)
				}
			}
			// Place latest device at index 0
			b.handleToDevs[alias] = append([]string{bundle.DeviceID}, updated...)
		}
	}

	return nil
}

// FetchBundle retrieves a public pre-key bundle for X3DH handshake, popping one OTK.
func (b *BundleRegistry) FetchBundle(targetID string) (*protocol.PreKeyBundle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clean target: remove '@' and lowercase
	clean := strings.TrimPrefix(strings.ToLower(targetID), "@")

	var sb *storedBundle
	if dev, ok := b.bundles[clean]; ok {
		sb = dev
	} else if dev, ok := b.bundles[targetID]; ok {
		sb = dev
	} else if devs, ok := b.handleToDevs[clean]; ok && len(devs) > 0 {
		sb = b.bundles[devs[0]] // Return primary / latest device
	} else if strings.HasPrefix(clean, "student_") {
		trimmed := strings.TrimPrefix(clean, "student_")
		if devs, ok := b.handleToDevs[trimmed]; ok && len(devs) > 0 {
			sb = b.bundles[devs[0]]
		}
	} else {
		studentAlias := "student_" + clean
		if devs, ok := b.handleToDevs[studentAlias]; ok && len(devs) > 0 {
			sb = b.bundles[devs[0]]
		}
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
