package crypto

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

var (
	ErrOutOfOrderLimitExceeded = errors.New("maximum skipped message threshold exceeded")
	ErrDuplicateOrOldMessage   = errors.New("message key already consumed or sequence expired")
	ErrRatchetKeyNotFound      = errors.New("no message key found to decrypt envelope")
)

const (
	MaxSkippedKeys = 1000
)

// SkippedKey holds an unused message key for an out-of-order message.
type SkippedKey struct {
	Key       []byte
	CreatedAt time.Time
}

// DoubleRatchet maintains the cryptographic state machine for an E2EE session.
type DoubleRatchet struct {
	mu sync.Mutex

	// DH Ratchet keys
	DHSendLocal  *KeyPair // Local DH key pair
	DHRecvRemote []byte   // Remote DH public key (32 bytes)

	// Symmetric chains
	RootKey      []byte // RK (32 bytes)
	SendChainKey []byte // CKS (32 bytes)
	RecvChainKey []byte // CKR (32 bytes)

	// Counters
	Ns uint32 // Number of messages sent in current sending chain
	Nr uint32 // Number of messages received in current receiving chain
	Pn uint32 // Number of messages in previous sending chain

	// Skipped message keys buffer: key is "RatchetSeq:ChainSeq"
	SkippedKeys map[string]SkippedKey
}

// NewDoubleRatchetInitiator initializes Alice's ratchet state using the X3DH master key and Bob's DH key.
func NewDoubleRatchetInitiator(sharedKey []byte, bobDHRemotePub []byte) (*DoubleRatchet, error) {
	dhLocal, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generating initiator ratchet key: %w", err)
	}

	dhSecret, err := ComputeDH(dhLocal.PrivateKey, bobDHRemotePub)
	if err != nil {
		return nil, fmt.Errorf("initial dh ratchet failed: %w", err)
	}

	// KDF_RK(sharedKey, dhSecret) -> (RootKey, SendChainKey)
	rootKey, sendChainKey, err := kdfRoot(sharedKey, dhSecret)
	if err != nil {
		return nil, err
	}

	return &DoubleRatchet{
		DHSendLocal:  dhLocal,
		DHRecvRemote: bobDHRemotePub,
		RootKey:      rootKey,
		SendChainKey: sendChainKey,
		RecvChainKey: nil,
		Ns:           0,
		Nr:           0,
		Pn:           0,
		SkippedKeys:  make(map[string]SkippedKey),
	}, nil
}

// NewDoubleRatchetReceiver initializes Bob's ratchet state using the X3DH master key and Bob's DH keypair.
func NewDoubleRatchetReceiver(sharedKey []byte, bobDHLocal *KeyPair) *DoubleRatchet {
	return &DoubleRatchet{
		DHSendLocal:  bobDHLocal,
		DHRecvRemote: nil,
		RootKey:      sharedKey,
		SendChainKey: nil,
		RecvChainKey: nil,
		Ns:           0,
		Nr:           0,
		Pn:           0,
		SkippedKeys:  make(map[string]SkippedKey),
	}
}

// Encrypt ratchets the symmetric sending chain and encrypts the plaintext inside an Envelope.
func (r *DoubleRatchet) Encrypt(senderID, recipientID string, plaintext []byte, ttlMS int64) (*protocol.Envelope, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.SendChainKey == nil {
		return nil, errors.New("cannot encrypt: sending chain not initialized")
	}

	// 1. Advance KDF sending chain: derive message key and next chain key
	nextCK, messageKey, err := kdfChain(r.SendChainKey)
	if err != nil {
		return nil, fmt.Errorf("advancing send chain: %w", err)
	}
	r.SendChainKey = nextCK
	defer ZeroBytes(messageKey)

	// 2. Build associated data to bind routing headers cryptographically
	env := protocol.NewEnvelope(senderID, recipientID, ttlMS)
	env.EphemeralPublicKey = r.DHSendLocal.PublicKey
	env.RatchetSeq = r.Pn
	env.ChainSeq = r.Ns
	r.Ns++

	assocData := fmt.Sprintf("%s:%s:%d:%d:%d", env.SenderID, env.RecipientID, env.RatchetSeq, env.ChainSeq, env.TimestampMS)

	// 3. Encrypt payload
	iv, ciphertext, tag, err := EncryptPayload(messageKey, plaintext, []byte(assocData))
	if err != nil {
		return nil, fmt.Errorf("payload encryption failed: %w", err)
	}

	env.IV = iv
	env.Ciphertext = ciphertext
	env.AuthTag = tag

	return env, nil
}

// Decrypt processes an incoming envelope, handling DH ratchet advancements and out-of-order skipped keys.
func (r *DoubleRatchet) Decrypt(env *protocol.Envelope) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	assocData := fmt.Sprintf("%s:%s:%d:%d:%d", env.SenderID, env.RecipientID, env.RatchetSeq, env.ChainSeq, env.TimestampMS)

	// 1. Check if key is already in skipped keys buffer (out-of-order resolution)
	skipKeyID := fmt.Sprintf("%d:%d", env.RatchetSeq, env.ChainSeq)
	if sk, ok := r.SkippedKeys[skipKeyID]; ok {
		delete(r.SkippedKeys, skipKeyID)
		defer ZeroBytes(sk.Key)
		return DecryptPayload(sk.Key, env.IV, env.Ciphertext, env.AuthTag, []byte(assocData))
	}

	// 2. If new remote DH public key arrived, advance DH ratchet
	if r.DHRecvRemote == nil || !bytes.Equal(env.EphemeralPublicKey, r.DHRecvRemote) {
		// Save any skipped keys from remaining messages in current receiving chain
		if err := r.skipMessageKeys(env.RatchetSeq); err != nil {
			return nil, err
		}
		if err := r.dhRatchetStep(env.EphemeralPublicKey); err != nil {
			return nil, err
		}
	}

	// 3. Save any skipped keys in current receiving chain up to env.ChainSeq
	if err := r.skipMessageKeys(env.ChainSeq); err != nil {
		return nil, err
	}

	// 4. Derive message key for this sequence
	nextCK, messageKey, err := kdfChain(r.RecvChainKey)
	if err != nil {
		return nil, fmt.Errorf("advancing recv chain: %w", err)
	}
	r.RecvChainKey = nextCK
	r.Nr++
	defer ZeroBytes(messageKey)

	// 5. Decrypt and verify payload
	plaintext, err := DecryptPayload(messageKey, env.IV, env.Ciphertext, env.AuthTag, []byte(assocData))
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

func (r *DoubleRatchet) skipMessageKeys(untilSeq uint32) error {
	if r.RecvChainKey == nil {
		return nil
	}

	for r.Nr < untilSeq {
		if len(r.SkippedKeys) >= MaxSkippedKeys {
			return ErrOutOfOrderLimitExceeded
		}
		nextCK, mk, err := kdfChain(r.RecvChainKey)
		if err != nil {
			return err
		}
		r.RecvChainKey = nextCK
		skipKeyID := fmt.Sprintf("%d:%d", r.Pn, r.Nr)
		r.SkippedKeys[skipKeyID] = SkippedKey{
			Key:       mk,
			CreatedAt: time.Now(),
		}
		r.Nr++
	}
	return nil
}

func (r *DoubleRatchet) dhRatchetStep(newRemotePub []byte) error {
	r.Pn = r.Ns
	r.Ns = 0
	r.Nr = 0
	r.DHRecvRemote = newRemotePub

	// 1. DH output with current send private key and new remote public key
	dhSecret1, err := ComputeDH(r.DHSendLocal.PrivateKey, newRemotePub)
	if err != nil {
		return err
	}

	// Advance Root Key and derive new Receiving Chain Key
	newRK, recvCK, err := kdfRoot(r.RootKey, dhSecret1)
	if err != nil {
		return err
	}
	r.RootKey = newRK
	r.RecvChainKey = recvCK

	// 2. Generate new local DH keypair
	newLocalDH, err := GenerateKeyPair()
	if err != nil {
		return err
	}
	r.DHSendLocal = newLocalDH

	// 3. DH output with new local private key and new remote public key
	dhSecret2, err := ComputeDH(r.DHSendLocal.PrivateKey, newRemotePub)
	if err != nil {
		return err
	}

	// Advance Root Key and derive new Sending Chain Key
	newRK2, sendCK, err := kdfRoot(r.RootKey, dhSecret2)
	if err != nil {
		return err
	}
	r.RootKey = newRK2
	r.SendChainKey = sendCK

	return nil
}

func kdfRoot(rootKey, dhSecret []byte) (newRootKey, chainKey []byte, err error) {
	derived, err := HKDFDerive(rootKey, dhSecret, []byte("CampusNet_Ratchet_KDF_RK"), 64)
	if err != nil {
		return nil, nil, err
	}
	return derived[:32], derived[32:], nil
}

func kdfChain(chainKey []byte) (nextChainKey, messageKey []byte, err error) {
	// HKDF chain expansion: constant inputs "0x01" for message key, "0x02" for next chain key
	nextCK, err := HKDFDerive(chainKey, []byte{0x02}, []byte("CampusNet_Chain_Next"), 32)
	if err != nil {
		return nil, nil, err
	}
	mk, err := HKDFDerive(chainKey, []byte{0x01}, []byte("CampusNet_Message_Key"), 32)
	if err != nil {
		return nil, nil, err
	}
	return nextCK, mk, nil
}
