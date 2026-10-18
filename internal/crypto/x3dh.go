package crypto

import (
	"crypto/ed25519"
	"errors"
	"fmt"
)

var (
	ErrSignatureInvalid = errors.New("signed pre-key signature verification failed")
	ErrMissingKeyBundle = errors.New("incomplete pre-key bundle for peer")
)

// X3DHSessionKeys contains the shared master key and session initialization details.
type X3DHSessionKeys struct {
	MasterSharedKey []byte // 32 bytes derived via HKDF
	AliceIKPub      []byte // Alice's public identity key
	AliceEKPub      []byte // Alice's ephemeral public key
	UsedOTK         []byte // The one-time prekey consumed (if any)
}

// PreKeyBundle holds the public keys required to perform an X3DH handshake.
type PreKeyBundle struct {
	IdentityKey         []byte // 32 bytes X25519
	IdentitySigningKey  []byte // 32 bytes Ed25519 public key (for verifying SPK)
	SignedPreKey        []byte // 32 bytes X25519
	SignedPreKeySig     []byte // 64 bytes Ed25519 signature
	OneTimePreKey       []byte // Optional: 32 bytes X25519
}

// AliceInitiateHandshake runs Alice's side of X3DH to establish a shared master key with Bob.
func AliceInitiateHandshake(
	aliceIK *KeyPair,
	bobBundle *PreKeyBundle,
) (*X3DHSessionKeys, error) {
	if bobBundle == nil || len(bobBundle.IdentityKey) != 32 || len(bobBundle.SignedPreKey) != 32 {
		return nil, ErrMissingKeyBundle
	}

	// Verify Bob's SPK signature if signing key is provided
	if len(bobBundle.IdentitySigningKey) == 32 && len(bobBundle.SignedPreKeySig) == 64 {
		if !ed25519.Verify(bobBundle.IdentitySigningKey, bobBundle.SignedPreKey, bobBundle.SignedPreKeySig) {
			return nil, ErrSignatureInvalid
		}
	}

	// 1. Generate Alice's Ephemeral Key Pair (EK_A)
	aliceEK, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generating ephemeral key: %w", err)
	}

	// 2. Compute Diffie-Hellman shared secrets:
	// DH1 = DH(IK_A, SPK_B)
	dh1, err := ComputeDH(aliceIK.PrivateKey, bobBundle.SignedPreKey)
	if err != nil {
		return nil, fmt.Errorf("dh1 calculation failed: %w", err)
	}

	// DH2 = DH(EK_A, IK_B)
	dh2, err := ComputeDH(aliceEK.PrivateKey, bobBundle.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("dh2 calculation failed: %w", err)
	}

	// DH3 = DH(EK_A, SPK_B)
	dh3, err := ComputeDH(aliceEK.PrivateKey, bobBundle.SignedPreKey)
	if err != nil {
		return nil, fmt.Errorf("dh3 calculation failed: %w", err)
	}

	// Combine DH outputs: DH1 || DH2 || DH3
	var dhConcat []byte
	dhConcat = append(dhConcat, dh1...)
	dhConcat = append(dhConcat, dh2...)
	dhConcat = append(dhConcat, dh3...)

	// DH4 = DH(EK_A, OPK_B) if Bob provided a One-Time Prekey
	var usedOTK []byte
	if len(bobBundle.OneTimePreKey) == 32 {
		dh4, err := ComputeDH(aliceEK.PrivateKey, bobBundle.OneTimePreKey)
		if err != nil {
			return nil, fmt.Errorf("dh4 calculation failed: %w", err)
		}
		dhConcat = append(dhConcat, dh4...)
		usedOTK = bobBundle.OneTimePreKey
	}

	// 3. Derive Master Shared Key using HKDF
	info := []byte("CampusNet_X3DH_v1")
	salt := []byte("CampusNet_Ratchet_Salt_2026")
	sk, err := HKDFDerive(salt, dhConcat, info, 32)
	if err != nil {
		return nil, fmt.Errorf("hkdf master key derivation failed: %w", err)
	}

	return &X3DHSessionKeys{
		MasterSharedKey: sk,
		AliceIKPub:      aliceIK.PublicKey,
		AliceEKPub:      aliceEK.PublicKey,
		UsedOTK:         usedOTK,
	}, nil
}

// BobReceiveHandshake computes the identical master shared key on Bob's side.
func BobReceiveHandshake(
	bobIK *KeyPair,
	bobSPK *KeyPair,
	bobOTK *KeyPair, // Optional, can be nil if no OTK was used
	aliceIKPub []byte,
	aliceEKPub []byte,
) ([]byte, error) {
	if len(aliceIKPub) != 32 || len(aliceEKPub) != 32 {
		return nil, errors.New("invalid Alice public key lengths")
	}

	// DH1 = DH(SPK_B, IK_A)
	dh1, err := ComputeDH(bobSPK.PrivateKey, aliceIKPub)
	if err != nil {
		return nil, fmt.Errorf("dh1 calculation failed: %w", err)
	}

	// DH2 = DH(IK_B, EK_A)
	dh2, err := ComputeDH(bobIK.PrivateKey, aliceEKPub)
	if err != nil {
		return nil, fmt.Errorf("dh2 calculation failed: %w", err)
	}

	// DH3 = DH(SPK_B, EK_A)
	dh3, err := ComputeDH(bobSPK.PrivateKey, aliceEKPub)
	if err != nil {
		return nil, fmt.Errorf("dh3 calculation failed: %w", err)
	}

	var dhConcat []byte
	dhConcat = append(dhConcat, dh1...)
	dhConcat = append(dhConcat, dh2...)
	dhConcat = append(dhConcat, dh3...)

	// DH4 = DH(OPK_B, EK_A) if Bob OTK was consumed
	if bobOTK != nil {
		dh4, err := ComputeDH(bobOTK.PrivateKey, aliceEKPub)
		if err != nil {
			return nil, fmt.Errorf("dh4 calculation failed: %w", err)
		}
		dhConcat = append(dhConcat, dh4...)
	}

	info := []byte("CampusNet_X3DH_v1")
	salt := []byte("CampusNet_Ratchet_Salt_2026")
	sk, err := HKDFDerive(salt, dhConcat, info, 32)
	if err != nil {
		return nil, fmt.Errorf("hkdf master key derivation failed: %w", err)
	}

	return sk, nil
}
