package protocol

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidEnvelope     = errors.New("invalid cryptographic envelope")
	ErrInvalidEphemeralKey = errors.New("invalid ephemeral public key length (must be 32 bytes)")
	ErrInvalidIV           = errors.New("invalid IV/nonce length (must be 12 bytes)")
	ErrInvalidAuthTag      = errors.New("invalid auth tag length (must be 16 bytes)")
	ErrEnvelopeExpired     = errors.New("cryptographic envelope has expired")
)

// Envelope is the zero-knowledge wire container for end-to-end encrypted messages.
// The backend server inspects ONLY routing headers (ID, RecipientID, SenderID, TTL, Timestamp).
// Ciphertext, EphemeralPublicKey, IV, and AuthTag are opaque to the server.
type Envelope struct {
	ID                 string `json:"id"`
	RecipientID        string `json:"recipient_id"`
	SenderID           string `json:"sender_id"`
	EphemeralPublicKey []byte `json:"ephemeral_public_key"` // 32 bytes X25519
	RatchetSeq         uint32 `json:"ratchet_seq"`           // DH ratchet sequence
	ChainSeq           uint32 `json:"chain_seq"`             // Symmetric KDF chain sequence
	IV                 []byte `json:"iv"`                    // 12 bytes ChaCha20-Poly1305 nonce
	Ciphertext         []byte `json:"ciphertext"`            // Encrypted payload
	AuthTag            []byte `json:"auth_tag"`              // 16 bytes AEAD MAC tag
	TTLMilliseconds    int64  `json:"ttl_ms"`                // Message lifespan (e.g. 5000 = 5s)
	TimestampMS        int64  `json:"timestamp_ms"`          // Sent timestamp in UTC epoch ms
}

// NewEnvelope initializes an envelope with a random UUID and current timestamp.
func NewEnvelope(senderID, recipientID string, ttlMS int64) *Envelope {
	uuidBytes := make([]byte, 16)
	_, _ = rand.Read(uuidBytes)
	uuidStr := fmt.Sprintf("%x-%x-%x-%x-%x", uuidBytes[0:4], uuidBytes[4:6], uuidBytes[6:8], uuidBytes[8:10], uuidBytes[10:16])

	if ttlMS <= 0 {
		ttlMS = 10000 // Default 10 second ephemerality
	}

	return &Envelope{
		ID:              uuidStr,
		SenderID:        senderID,
		RecipientID:     recipientID,
		TTLMilliseconds: ttlMS,
		TimestampMS:     time.Now().UTC().UnixMilli(),
	}
}

// Validate verifies the cryptographic envelope has sound structural dimensions.
func (e *Envelope) Validate() error {
	if e.ID == "" || e.RecipientID == "" || e.SenderID == "" {
		return ErrInvalidEnvelope
	}
	if len(e.EphemeralPublicKey) != 32 {
		return ErrInvalidEphemeralKey
	}
	if len(e.IV) != 12 {
		return ErrInvalidIV
	}
	if len(e.AuthTag) != 16 {
		return ErrInvalidAuthTag
	}
	if len(e.Ciphertext) == 0 {
		return errors.New("empty ciphertext payload")
	}
	return nil
}

// IsExpired checks if the envelope's millisecond-grade TTL has elapsed.
func (e *Envelope) IsExpired() bool {
	if e.TTLMilliseconds <= 0 {
		return false
	}
	elapsed := time.Now().UTC().UnixMilli() - e.TimestampMS
	return elapsed >= e.TTLMilliseconds
}

// RemainingTTL calculates the remaining life in milliseconds. Returns 0 if expired.
func (e *Envelope) RemainingTTL() time.Duration {
	elapsed := time.Now().UTC().UnixMilli() - e.TimestampMS
	remaining := e.TTLMilliseconds - elapsed
	if remaining <= 0 {
		return 0
	}
	return time.Duration(remaining) * time.Millisecond
}

// MarshalJSON provides safe JSON serialization with base64 encoded byte arrays.
func (e *Envelope) MarshalJSON() ([]byte, error) {
	type Alias Envelope
	return json.Marshal(&struct {
		*Alias
		EphemeralPublicKey string `json:"ephemeral_public_key"`
		IV                 string `json:"iv"`
		Ciphertext         string `json:"ciphertext"`
		AuthTag            string `json:"auth_tag"`
	}{
		Alias:              (*Alias)(e),
		EphemeralPublicKey: base64.StdEncoding.EncodeToString(e.EphemeralPublicKey),
		IV:                 base64.StdEncoding.EncodeToString(e.IV),
		Ciphertext:         base64.StdEncoding.EncodeToString(e.Ciphertext),
		AuthTag:            base64.StdEncoding.EncodeToString(e.AuthTag),
	})
}

// UnmarshalJSON provides safe JSON deserialization decoding base64 byte arrays.
func (e *Envelope) UnmarshalJSON(data []byte) error {
	type Alias Envelope
	aux := &struct {
		*Alias
		EphemeralPublicKey string `json:"ephemeral_public_key"`
		IV                 string `json:"iv"`
		Ciphertext         string `json:"ciphertext"`
		AuthTag            string `json:"auth_tag"`
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var err error
	if aux.EphemeralPublicKey != "" {
		if e.EphemeralPublicKey, err = base64.StdEncoding.DecodeString(aux.EphemeralPublicKey); err != nil {
			return fmt.Errorf("decoding ephemeral_public_key: %w", err)
		}
	}
	if aux.IV != "" {
		if e.IV, err = base64.StdEncoding.DecodeString(aux.IV); err != nil {
			return fmt.Errorf("decoding iv: %w", err)
		}
	}
	if aux.Ciphertext != "" {
		if e.Ciphertext, err = base64.StdEncoding.DecodeString(aux.Ciphertext); err != nil {
			return fmt.Errorf("decoding ciphertext: %w", err)
		}
	}
	if aux.AuthTag != "" {
		if e.AuthTag, err = base64.StdEncoding.DecodeString(aux.AuthTag); err != nil {
			return fmt.Errorf("decoding auth_tag: %w", err)
		}
	}
	return nil
}
