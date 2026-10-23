package crypto

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidSecretLength = errors.New("invalid secret key length for hybrid derivation")
)

// DeriveHybridPQSharedSecret combines a classical Curve25519 ECDH secret with a
// post-quantum KEM (ML-KEM-768 / Kyber) shared secret via RFC 5869 HKDF-SHA256.
// POST-QUANTUM DEFENSE: Ensures forward secrecy against both quantum adversaries
// ("harvest-now, decrypt-later") and classical mathematical cryptanalysis.
func DeriveHybridPQSharedSecret(classicalECDHSecret, pqKEMSecret, info []byte) ([]byte, error) {
	if len(classicalECDHSecret) < 32 {
		return nil, fmt.Errorf("%w: classical secret must be at least 32 bytes, got %d", ErrInvalidSecretLength, len(classicalECDHSecret))
	}
	if len(pqKEMSecret) < 32 {
		return nil, fmt.Errorf("%w: post-quantum secret must be at least 32 bytes, got %d", ErrInvalidSecretLength, len(pqKEMSecret))
	}

	salt := []byte("CampusNet_PQ_Hybrid_Salt_v1")
	ikm := append(append([]byte{}, classicalECDHSecret...), pqKEMSecret...)
	defer ZeroBytes(ikm)

	hybridInfo := append([]byte("CampusNet_PQXDH_Hybrid_v1:"), info...)
	derived, err := HKDFDerive(salt, ikm, hybridInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("hybrid key derivation failed: %w", err)
	}

	return derived, nil
}
