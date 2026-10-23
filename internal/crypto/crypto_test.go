package crypto

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

func TestKeyPairGenerationAndECDH(t *testing.T) {
	aliceKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate Alice keypair: %v", err)
	}

	bobKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate Bob keypair: %v", err)
	}

	if len(aliceKey.PublicKey) != 32 || len(bobKey.PublicKey) != 32 {
		t.Fatalf("invalid public key length: expected 32 bytes")
	}

	// DH(AlicePriv, BobPub) == DH(BobPriv, AlicePub)
	secretAlice, err := ComputeDH(aliceKey.PrivateKey, bobKey.PublicKey)
	if err != nil {
		t.Fatalf("Alice DH calculation failed: %v", err)
	}

	secretBob, err := ComputeDH(bobKey.PrivateKey, aliceKey.PublicKey)
	if err != nil {
		t.Fatalf("Bob DH calculation failed: %v", err)
	}

	if !bytes.Equal(secretAlice, secretBob) {
		t.Fatalf("Diffie-Hellman secrets mismatch: %x != %x", secretAlice, secretBob)
	}
}

func TestAEADPayloadEncryption(t *testing.T) {
	key := make([]byte, 32)
	copy(key, []byte("01234567890123456789012345678901"))

	plaintext := []byte("Classified CampusNet Ephemeral Data")
	assocData := []byte("context:auth_headers:2026")

	nonce, ciphertext, tag, err := EncryptPayload(key, plaintext, assocData)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	decrypted, err := DecryptPayload(key, nonce, ciphertext, tag, assocData)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("decrypted text does not match plaintext: got %s, want %s", decrypted, plaintext)
	}

	// Verify tampering detection
	badTag := make([]byte, len(tag))
	copy(badTag, tag)
	badTag[0] ^= 0xFF
	_, err = DecryptPayload(key, nonce, ciphertext, badTag, assocData)
	if err == nil {
		t.Fatalf("expected decryption failure on tampered tag, but got nil")
	}
}

func TestX3DHHandshake(t *testing.T) {
	aliceIK, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	bobIK, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bobSPK, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bobOTK, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	bobSignPub, bobSignPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	bobSPKSig := ed25519.Sign(bobSignPriv, bobSPK.PublicKey)

	bobBundle := &PreKeyBundle{
		IdentityKey:        bobIK.PublicKey,
		IdentitySigningKey: bobSignPub,
		SignedPreKey:       bobSPK.PublicKey,
		SignedPreKeySig:    bobSPKSig,
		OneTimePreKey:      bobOTK.PublicKey,
	}

	// Alice initiates handshake
	aliceSession, err := AliceInitiateHandshake(aliceIK, bobBundle)
	if err != nil {
		t.Fatalf("Alice initiate handshake failed: %v", err)
	}

	// Bob receives and computes shared key
	bobMasterKey, err := BobReceiveHandshake(bobIK, bobSPK, bobOTK, aliceSession.AliceIKPub, aliceSession.AliceEKPub)
	if err != nil {
		t.Fatalf("Bob receive handshake failed: %v", err)
	}

	if !bytes.Equal(aliceSession.MasterSharedKey, bobMasterKey) {
		t.Fatalf("X3DH derived keys do not match!\nAlice: %x\nBob:   %x", aliceSession.MasterSharedKey, bobMasterKey)
	}
}

func TestX3DHSignatureVerificationStrict(t *testing.T) {
	aliceIK, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bobIK, _ := GenerateKeyPair()
	bobSPK, _ := GenerateKeyPair()
	bobSignPub, bobSignPriv, _ := ed25519.GenerateKey(nil)
	validSig := ed25519.Sign(bobSignPriv, bobSPK.PublicKey)

	t.Run("missing_signing_key_rejected", func(t *testing.T) {
		bundle := &PreKeyBundle{
			IdentityKey:     bobIK.PublicKey,
			SignedPreKey:    bobSPK.PublicKey,
			SignedPreKeySig: validSig,
		}
		_, err := AliceInitiateHandshake(aliceIK, bundle)
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("expected ErrSignatureInvalid for omitted signing key, got: %v", err)
		}
	})

	t.Run("missing_spk_sig_rejected", func(t *testing.T) {
		bundle := &PreKeyBundle{
			IdentityKey:        bobIK.PublicKey,
			IdentitySigningKey: bobSignPub,
			SignedPreKey:       bobSPK.PublicKey,
		}
		_, err := AliceInitiateHandshake(aliceIK, bundle)
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("expected ErrSignatureInvalid for omitted signature, got: %v", err)
		}
	})

	t.Run("tampered_spk_rejected", func(t *testing.T) {
		attackerSPK, _ := GenerateKeyPair()
		bundle := &PreKeyBundle{
			IdentityKey:        bobIK.PublicKey,
			IdentitySigningKey: bobSignPub,
			SignedPreKey:       attackerSPK.PublicKey, // swapped SPK
			SignedPreKeySig:    validSig,              // signature was for bobSPK
		}
		_, err := AliceInitiateHandshake(aliceIK, bundle)
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Fatalf("expected ErrSignatureInvalid for tampered SPK, got: %v", err)
		}
	})
}

func TestDoubleRatchetEndToEnd(t *testing.T) {
	// Pre-requisite: Shared master key from X3DH
	sharedKey := make([]byte, 32)
	copy(sharedKey, []byte("master_shared_key_test_32_bytes!"))

	bobDH, _ := GenerateKeyPair()

	// Initialize ratchets
	aliceRatchet, err := NewDoubleRatchetInitiator(sharedKey, bobDH.PublicKey)
	if err != nil {
		t.Fatalf("failed to init Alice ratchet: %v", err)
	}

	bobRatchet := NewDoubleRatchetReceiver(sharedKey, bobDH)

	// Message 1: Alice -> Bob
	msg1 := []byte("Meet at Main Library 2nd floor")
	env1, err := aliceRatchet.Encrypt("alice", "bob", msg1, 10000)
	if err != nil {
		t.Fatalf("Alice encryption failed: %v", err)
	}

	decrypted1, err := bobRatchet.Decrypt(env1)
	if err != nil {
		t.Fatalf("Bob decryption failed: %v", err)
	}
	if !bytes.Equal(msg1, decrypted1) {
		t.Fatalf("Message 1 mismatch: got %s, want %s", decrypted1, msg1)
	}

	// Message 2: Bob -> Alice (DH ratchet turns)
	msg2 := []byte("Understood, bringing encrypted laptop.")
	env2, err := bobRatchet.Encrypt("bob", "alice", msg2, 10000)
	if err != nil {
		t.Fatalf("Bob encryption failed: %v", err)
	}

	decrypted2, err := aliceRatchet.Decrypt(env2)
	if err != nil {
		t.Fatalf("Alice decryption failed: %v", err)
	}
	if !bytes.Equal(msg2, decrypted2) {
		t.Fatalf("Message 2 mismatch: got %s, want %s", decrypted2, msg2)
	}

	// Message 3: Out of order test (Alice sends M3 and M4, Bob receives M4 then M3)
	msg3 := []byte("Sequence Message 3")
	msg4 := []byte("Sequence Message 4")

	env3, _ := aliceRatchet.Encrypt("alice", "bob", msg3, 10000)
	env4, _ := aliceRatchet.Encrypt("alice", "bob", msg4, 10000)

	// Bob decrypts M4 first
	dec4, err := bobRatchet.Decrypt(env4)
	if err != nil {
		t.Fatalf("Bob failed to decrypt out-of-order M4: %v", err)
	}
	if !bytes.Equal(msg4, dec4) {
		t.Fatalf("M4 mismatch: got %s, want %s", dec4, msg4)
	}

	// Bob then decrypts skipped M3
	dec3, err := bobRatchet.Decrypt(env3)
	if err != nil {
		t.Fatalf("Bob failed to decrypt skipped M3: %v", err)
	}
	if !bytes.Equal(msg3, dec3) {
		t.Fatalf("M3 mismatch: got %s, want %s", dec3, msg3)
	}
}

func TestPairwiseIsolationAndDowngradeRejection(t *testing.T) {
	// 1. Normal pairwise encryption between alice and bob
	msg := []byte("Strict pairwise confidential message")
	iv, cipher, tag, err := EncryptPairwisePayload("alice", "bob", msg)
	if err != nil {
		t.Fatalf("pairwise encryption failed: %v", err)
	}

	decrypted, err := DecryptPairwisePayload("alice", "bob", iv, cipher, tag)
	if err != nil {
		t.Fatalf("pairwise decryption failed: %v", err)
	}
	if !bytes.Equal(msg, decrypted) {
		t.Fatalf("decrypted text mismatch")
	}

	// 2. Third party (charlie and eve) MUST NOT be able to decrypt alice and bob's message
	_, err = DecryptPairwisePayload("charlie", "eve", iv, cipher, tag)
	if err == nil {
		t.Fatalf("security violation: third party charlie/eve decrypted alice/bob message!")
	}

	// 3. Attacker encrypts message with the demo campus key and attempts to inject as Alice
	ivDemo, cipherDemo, tagDemo, err := EncryptMessagePayload([]byte("Forged attacker message"))
	if err != nil {
		t.Fatalf("failed demo encryption: %v", err)
	}

	// DecryptPairwisePayload MUST REJECT the forged message (no downgrade attack)
	_, err = DecryptPairwisePayload("alice", "bob", ivDemo, cipherDemo, tagDemo)
	if err == nil {
		t.Fatalf("security vulnerability: DecryptPairwisePayload accepted forged message encrypted with campus demo key!")
	}
}


func TestDoubleRatchetStateRollbackOnFailedDecrypt(t *testing.T) {
	sharedKey := make([]byte, 32)
	copy(sharedKey, []byte("state_rollback_test_master_key!!"))

	bobDH, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	aliceRatchet, err := NewDoubleRatchetInitiator(sharedKey, bobDH.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	bobRatchet := NewDoubleRatchetReceiver(sharedKey, bobDH)

	// Step 1: Alice sends initial message to initialize Bob's receiving chain
	m1 := []byte("Initial message 1")
	env1, err := aliceRatchet.Encrypt("alice", "bob", m1, 10000)
	if err != nil {
		t.Fatal(err)
	}
	dec1, err := bobRatchet.Decrypt(env1)
	if err != nil || !bytes.Equal(m1, dec1) {
		t.Fatalf("initial message decrypt failed: %v", err)
	}

	// Step 2: Attacker injects a forged packet with a new ephemeral public key and invalid ciphertext/tag
	attackerDH, _ := GenerateKeyPair()
	forgedEnv := &protocol.Envelope{
		ID:                 "forged-msg-123",
		SenderID:           "alice",
		RecipientID:        "bob",
		TimestampMS:        env1.TimestampMS + 1000,
		TTLMilliseconds:    10000,
		EphemeralPublicKey: attackerDH.PublicKey, // New unexpected ephemeral key
		RatchetSeq:         1,
		ChainSeq:           0,
		IV:                 make([]byte, 12),
		Ciphertext:         []byte("forged_corrupted_ciphertext_data"),
		AuthTag:            make([]byte, 16),
	}

	_, err = bobRatchet.Decrypt(forgedEnv)
	if err == nil {
		t.Fatalf("expected decryption error on forged packet, got nil")
	}

	// Step 3: Alice sends a legitimate follow-up message
	m2 := []byte("Legitimate Alice Message After Attack")
	env2, err := aliceRatchet.Encrypt("alice", "bob", m2, 10000)
	if err != nil {
		t.Fatalf("alice encrypt failed: %v", err)
	}

	// Bob MUST still be able to decrypt Alice's legitimate message without state desynchronization
	dec2, err := bobRatchet.Decrypt(env2)
	if err != nil {
		t.Fatalf("CRITICAL STATE DESYNCHRONIZATION: Bob failed to decrypt Alice's legitimate message after attack: %v", err)
	}
	if !bytes.Equal(m2, dec2) {
		t.Fatalf("decrypted message mismatch: got %s, want %s", dec2, m2)
	}
}
