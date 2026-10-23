package crypto

import (
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestCheckStoredHandle(t *testing.T) {
	s, err := NewSecureStore()
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.Get("user_handle")
	t.Logf("STORED HANDLE: %q (err: %v)", string(h), err)
	d, _ := s.Get("device_id")
	t.Logf("STORED DEVICE: %q", string(d))
}

func TestReproductionKAT(t *testing.T) {
	k1, err := DerivePairwiseKey("student_guest", "alice")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("KEY (student_guest, alice): %s", hex.EncodeToString(k1))

	k2, err := DerivePairwiseKey("@student_guest", "@alice")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("KEY (@student_guest, @alice): %s", hex.EncodeToString(k2))

	k3, err := DerivePairwiseKey("guest", "alice")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("KEY (guest, alice): %s", hex.EncodeToString(k3))

	// Fixed 12-byte IV for testing
	iv := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	plain := []byte("Hello Alice. Testing CLI to Web message.")

	block, err := newCipherBlock(k1)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, iv, plain, nil)
	tagStart := len(sealed) - 16
	ciphertext := sealed[:tagStart]
	tag := sealed[tagStart:]

	t.Logf("IV (b64): %s", base64.StdEncoding.EncodeToString(iv))
	t.Logf("CIPHERTEXT (b64): %s", base64.StdEncoding.EncodeToString(ciphertext))
	t.Logf("TAG (b64): %s", base64.StdEncoding.EncodeToString(tag))
}
