package protocol

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	ConversationID     string `json:"conversation_id,omitempty"`
	Seq                uint64 `json:"seq,omitempty"`
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

// DeriveConversationID computes canonical conversation identifier between two parties.
func DeriveConversationID(user1, user2 string) string {
	u1 := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(user1)), "@")
	u2 := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(user2)), "@")
	if strings.HasPrefix(u2, "group_") || strings.HasPrefix(u2, "study_") || strings.HasPrefix(u2, "robotics_") {
		return "group:" + u2
	}
	if u1 <= u2 {
		return fmt.Sprintf("conv:%s:%s", u1, u2)
	}
	return fmt.Sprintf("conv:%s:%s", u2, u1)
}

// NewUUIDv7 generates an RFC 9562 UUIDv7 with millisecond timestamp ordering.
func NewUUIDv7() string {
	var uuid [16]byte
	now := time.Now().UTC().UnixMilli()
	uuid[0] = byte(now >> 40)
	uuid[1] = byte(now >> 32)
	uuid[2] = byte(now >> 24)
	uuid[3] = byte(now >> 16)
	uuid[4] = byte(now >> 8)
	uuid[5] = byte(now)
	_, _ = rand.Read(uuid[6:])
	uuid[6] = (uuid[6] & 0x0f) | 0x70 // Version 7
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant RFC 4122 / 9562
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// NewEnvelope initializes an envelope with a UUIDv7 and current timestamp.
func NewEnvelope(senderID, recipientID string, ttlMS int64) *Envelope {
	uuidStr := NewUUIDv7()

	if ttlMS <= 0 {
		ttlMS = 10000 // Default 10 second ephemerality
	}

	return &Envelope{
		ID:              uuidStr,
		ConversationID:  DeriveConversationID(senderID, recipientID),
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
	if len(e.Ciphertext) > 1048576 {
		return errors.New("ciphertext payload exceeds maximum size limit of 1MB")
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
