package crypto

import (
	"bytes"
	"crypto/cipher"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

func loadKATFixtures(t *testing.T) *KATFixtures {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "crypto_kat_fixtures.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read KAT fixtures at %s: %v", path, err)
	}
	var f KATFixtures
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("failed to parse KAT fixtures: %v", err)
	}
	return &f
}

func TestKAT_HandleNormalization(t *testing.T) {
	f := loadKATFixtures(t)
	for _, item := range f.HandleNormalization {
		actual := CanonicalHandle(item.Input)
		if actual != item.Expected {
			t.Errorf("Handle %q: expected %q, got %q", item.Input, item.Expected, actual)
		}
	}
}

func TestKAT_PairwiseKDF(t *testing.T) {
	f := loadKATFixtures(t)
	for _, item := range f.PairwiseKDF {
		derived, err := DerivePairwiseKey(item.User1, item.User2)
		if err != nil {
			t.Errorf("DerivePairwiseKey(%q, %q) error: %v", item.User1, item.User2, err)
			continue
		}
		actualHex := hex.EncodeToString(derived)
		if actualHex != item.DerivedKeyHex {
			t.Errorf("DerivePairwiseKey(%q, %q): expected %s, got %s", item.User1, item.User2, item.DerivedKeyHex, actualHex)
		}
	}
}

func TestKAT_AEADEncryptDecrypt(t *testing.T) {
	f := loadKATFixtures(t)
	key, _ := hex.DecodeString(f.AEAD.KeyHex)
	iv, _ := base64.StdEncoding.DecodeString(f.AEAD.IVBase64)
	expectedCipher, _ := base64.StdEncoding.DecodeString(f.AEAD.CiphertextB64)
	expectedTag, _ := base64.StdEncoding.DecodeString(f.AEAD.AuthTagB64)

	// Test Decrypt against fixture
	decrypted, err := DecryptPayload(key, iv, expectedCipher, expectedTag, []byte(f.AEAD.AAD))
	if err != nil {
		t.Fatalf("DecryptPayload failed on fixture: %v", err)
	}
	if string(decrypted) != f.AEAD.Plaintext {
		t.Fatalf("Decrypted plaintext mismatch: expected %q, got %q", f.AEAD.Plaintext, string(decrypted))
	}

	// Test Encrypt with fixed IV matches fixture
	block, err := newCipherBlock(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, iv, []byte(f.AEAD.Plaintext), []byte(f.AEAD.AAD))
	tagStart := len(sealed) - 16
	actualCipher := sealed[:tagStart]
	actualTag := sealed[tagStart:]

	if !bytes.Equal(actualCipher, expectedCipher) {
		t.Fatalf("Ciphertext mismatch on known IV")
	}
	if !bytes.Equal(actualTag, expectedTag) {
		t.Fatalf("AuthTag mismatch on known IV")
	}
}

func TestKAT_TSEncrypt_GoDecrypt(t *testing.T) {
	f := loadKATFixtures(t)
	key, _ := hex.DecodeString(f.TSAEAD.KeyHex)
	iv, _ := base64.StdEncoding.DecodeString(f.TSAEAD.IVBase64)
	expectedCipher, _ := base64.StdEncoding.DecodeString(f.TSAEAD.CiphertextB64)
	expectedTag, _ := base64.StdEncoding.DecodeString(f.TSAEAD.AuthTagB64)

	decrypted, err := DecryptPayload(key, iv, expectedCipher, expectedTag, []byte(f.TSAEAD.AAD))
	if err != nil {
		t.Fatalf("Go failed to decrypt TypeScript-produced ciphertext: %v", err)
	}
	if string(decrypted) != f.TSAEAD.Plaintext {
		t.Fatalf("Decrypted plaintext mismatch: expected %q, got %q", f.TSAEAD.Plaintext, string(decrypted))
	}
}

func TestKAT_X3DHSharedSecret(t *testing.T) {
	f := loadKATFixtures(t)
	x := f.X3DH

	aliceIKPriv, _ := hex.DecodeString(x.AliceIKPrivHex)
	aliceEKPriv, _ := hex.DecodeString(x.AliceEKPrivHex)
	bobIKPub, _ := hex.DecodeString(x.BobIKPubHex)
	bobSPKPub, _ := hex.DecodeString(x.BobSPKPubHex)
	bobOPKPub, _ := hex.DecodeString(x.BobOPKPubHex)
	bobSignPub, _ := hex.DecodeString(x.BobSignPubHex)
	bobSPKSig, _ := hex.DecodeString(x.BobSPKSigHex)

	// Verify Bob's SPK signature
	if !ed25519.Verify(bobSignPub, bobSPKPub, bobSPKSig) {
		t.Fatalf("Bob SPK signature verification failed on fixture")
	}

	// Compute DH
	curve := deterministicKeyPair(1).PrivateKey.Curve()
	aIK, _ := curve.NewPrivateKey(aliceIKPriv)
	aEK, _ := curve.NewPrivateKey(aliceEKPriv)

	dh1, err := ComputeDH(aIK, bobSPKPub)
	if err != nil {
		t.Fatal(err)
	}
	dh2, err := ComputeDH(aEK, bobIKPub)
	if err != nil {
		t.Fatal(err)
	}
	dh3, err := ComputeDH(aEK, bobSPKPub)
	if err != nil {
		t.Fatal(err)
	}
	dh4, err := ComputeDH(aEK, bobOPKPub)
	if err != nil {
		t.Fatal(err)
	}

	var dhConcat []byte
	dhConcat = append(dhConcat, dh1...)
	dhConcat = append(dhConcat, dh2...)
	dhConcat = append(dhConcat, dh3...)
	dhConcat = append(dhConcat, dh4...)

	info := []byte("CampusNet_X3DH_v1")
	salt := []byte("CampusNet_Ratchet_Salt_2026")
	masterKey, err := HKDFDerive(salt, dhConcat, info, 32)
	if err != nil {
		t.Fatal(err)
	}

	if hex.EncodeToString(masterKey) != x.MasterSharedKeyHex {
		t.Fatalf("MasterSharedKey mismatch: expected %s, got %s", x.MasterSharedKeyHex, hex.EncodeToString(masterKey))
	}
}

func TestKAT_RatchetMessages(t *testing.T) {
	f := loadKATFixtures(t)
	masterKey, _ := hex.DecodeString(f.X3DH.MasterSharedKeyHex)
	bobSPKPriv, _ := hex.DecodeString(f.X3DH.BobSPKPrivHex)

	curve := deterministicKeyPair(1).PrivateKey.Curve()
	bSPK, _ := curve.NewPrivateKey(bobSPKPriv)
	bobDH := &KeyPair{
		PrivateKey: bSPK,
		PublicKey:  bSPK.PublicKey().Bytes(),
	}

	aliceDH := deterministicKeyPair(70)
	aliceRatchet, err := NewDoubleRatchetInitiatorWithDH(masterKey, aliceDH, bobDH.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	bobRatchet := NewDoubleRatchetReceiver(masterKey, bobDH)

	for _, m := range f.RatchetMessages {
		iv, _ := base64.StdEncoding.DecodeString(m.IVB64)
		cipherBytes, _ := base64.StdEncoding.DecodeString(m.CiphertextB64)
		tag, _ := base64.StdEncoding.DecodeString(m.AuthTagB64)
		ephemPub, _ := base64.StdEncoding.DecodeString(m.EphemeralPubB64)

		env := &protocol.Envelope{
			SenderID:           m.Sender,
			RecipientID:        m.Recipient,
			RatchetSeq:         m.RatchetSeq,
			ChainSeq:           m.ChainSeq,
			EphemeralPublicKey: ephemPub,
			IV:                 iv,
			Ciphertext:         cipherBytes,
			AuthTag:            tag,
			TimestampMS:        m.TimestampMS,
		}

		if m.Sender == "alice" {
			dec, err := bobRatchet.Decrypt(env)
			if err != nil {
				t.Fatalf("Bob failed to decrypt msg %d: %v", m.Index, err)
			}
			if string(dec) != m.Plaintext {
				t.Fatalf("Msg %d plaintext mismatch: expected %q, got %q", m.Index, m.Plaintext, string(dec))
			}
		} else {
			dec, err := aliceRatchet.Decrypt(env)
			if err != nil {
				t.Fatalf("Alice failed to decrypt msg %d: %v", m.Index, err)
			}
			if string(dec) != m.Plaintext {
				t.Fatalf("Msg %d plaintext mismatch: expected %q, got %q", m.Index, m.Plaintext, string(dec))
			}
		}
	}
}

func TestKAT_EnvelopeSerialization(t *testing.T) {
	f := loadKATFixtures(t)
	expectedJSON := strings.TrimSpace(f.EnvelopeWire.WireJSON)

	var env protocol.Envelope
	if err := json.Unmarshal([]byte(expectedJSON), &env); err != nil {
		t.Fatalf("failed to unmarshal WireJSON: %v", err)
	}

	if env.ID != f.EnvelopeWire.Envelope.ID {
		t.Errorf("ID mismatch: %s != %s", env.ID, f.EnvelopeWire.Envelope.ID)
	}
	if !bytes.Equal(env.IV, f.EnvelopeWire.Envelope.IV) {
		t.Errorf("IV mismatch")
	}
	if !bytes.Equal(env.Ciphertext, f.EnvelopeWire.Envelope.Ciphertext) {
		t.Errorf("Ciphertext mismatch")
	}
	if !bytes.Equal(env.AuthTag, f.EnvelopeWire.Envelope.AuthTag) {
		t.Errorf("AuthTag mismatch")
	}
}
