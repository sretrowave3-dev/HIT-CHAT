package protocol

import (
	"encoding/json"
	"testing"
)

// FuzzEnvelopeJSON tests that parsing malformed or adversarial envelope JSON payloads
// never triggers runtime panics or unbounded memory consumption.
func FuzzEnvelopeJSON(f *testing.F) {
	// Seed with valid and boundary envelopes
	env := NewEnvelope("alice", "bob", 5000)
	env.Ciphertext = []byte("encrypted_sample_data")
	env.IV = make([]byte, 12)
	env.AuthTag = make([]byte, 16)
	b, _ := json.Marshal(env)
	f.Add(b)
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"id":"","sender_id":"test","recipient_id":null}`))
	f.Add([]byte(`{"ttl_ms":-999999999999999999}`))
	f.Add([]byte(`{"ratchet_seq":4294967295,"chain_seq":4294967295}`))
	f.Add([]byte("\x00\x01\x02\x03\xff\xfe"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var target Envelope
		_ = json.Unmarshal(data, &target)
	})
}

// FuzzStreamFrameJSON tests full-duplex WebSocket frame deserialization against corrupt input.
func FuzzStreamFrameJSON(f *testing.F) {
	f1 := StreamFrame{
		Op:        OpSendEnvelope,
		SessionID: "sess-123",
		Envelope:  NewEnvelope("u1", "u2", 1000),
	}
	b1, _ := json.Marshal(f1)
	f.Add(b1)
	f.Add([]byte(`{"op":"UNKNOWN_OP_TEST"}`))
	f.Add([]byte(`{"op":"SEND_ENVELOPE","envelope":{"ciphertext":"AAAA"}}`))
	f.Add([]byte(`{"presence":{"bssid_hash":"invalid_hex","floor_level":999999}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var frame StreamFrame
		_ = json.Unmarshal(data, &frame)
	})
}
