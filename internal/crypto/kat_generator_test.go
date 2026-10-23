package crypto

import (
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

type KATFixtures struct {
	HandleNormalization []HandleNormFixture    `json:"handle_normalization"`
	PairwiseKDF         []PairwiseKDFFixture    `json:"pairwise_kdf"`
	X3DH                X3DHFixture             `json:"x3dh"`
	AEAD                AEADFixture             `json:"aead"`
	TSAEAD              AEADFixture             `json:"ts_aead"`
	RatchetKDFChain     []RatchetChainStep      `json:"ratchet_kdf_chain,omitempty"`
	RatchetMessages     []RatchetMessageFixture `json:"ratchet_messages,omitempty"`
	EnvelopeWire        EnvelopeWireFixture     `json:"envelope_wire"`
}

type RatchetMessageFixture struct {
	Index           int    `json:"index"`
	Sender          string `json:"sender"`
	Recipient       string `json:"recipient"`
	RatchetSeq      uint32 `json:"ratchet_seq"`
	ChainSeq        uint32 `json:"chain_seq"`
	TimestampMS     int64  `json:"timestamp_ms"`
	EphemeralPubB64 string `json:"ephemeral_pub_b64"`
	IVB64           string `json:"iv_b64"`
	CiphertextB64   string `json:"ciphertext_b64"`
	AuthTagB64      string `json:"auth_tag_b64"`
	Plaintext       string `json:"plaintext"`
}

type HandleNormFixture struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

type PairwiseKDFFixture struct {
	User1         string `json:"user1"`
	User2         string `json:"user2"`
	Normalized1   string `json:"normalized1"`
	Normalized2   string `json:"normalized2"`
	InfoString    string `json:"info_string"`
	DerivedKeyHex string `json:"derived_key_hex"`
}

type X3DHFixture struct {
	AliceIKPrivHex     string `json:"alice_ik_priv_hex"`
	AliceIKPubHex      string `json:"alice_ik_pub_hex"`
	AliceEKPrivHex     string `json:"alice_ek_priv_hex"`
	AliceEKPubHex      string `json:"alice_ek_pub_hex"`
	BobIKPrivHex       string `json:"bob_ik_priv_hex"`
	BobIKPubHex        string `json:"bob_ik_pub_hex"`
	BobSPKPrivHex      string `json:"bob_spk_priv_hex"`
	BobSPKPubHex       string `json:"bob_spk_pub_hex"`
	BobOPKPrivHex      string `json:"bob_opk_priv_hex"`
	BobOPKPubHex       string `json:"bob_opk_pub_hex"`
	BobSignPrivHex     string `json:"bob_sign_priv_hex"`
	BobSignPubHex      string `json:"bob_sign_pub_hex"`
	BobSPKSigHex       string `json:"bob_spk_sig_hex"`
	MasterSharedKeyHex string `json:"master_shared_key_hex"`
}

type AEADFixture struct {
	KeyHex        string `json:"key_hex"`
	IVBase64      string `json:"iv_b64"`
	Plaintext     string `json:"plaintext"`
	AAD           string `json:"aad"`
	CiphertextB64 string `json:"ciphertext_b64"`
	AuthTagB64    string `json:"auth_tag_b64"`
}

type RatchetChainStep struct {
	Step          int    `json:"step"`
	ChainKeyInHex string `json:"chain_key_in_hex"`
	NextCKHex     string `json:"next_ck_hex"`
	MessageKeyHex string `json:"message_key_hex"`
}

type EnvelopeWireFixture struct {
	Envelope *protocol.Envelope `json:"envelope"`
	WireJSON string             `json:"wire_json"`
}

func deterministicKeyPair(seedByte byte) *KeyPair {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(seedByte + byte(i)*3)
	}
	curve := ecdh.X25519()
	priv, err := curve.NewPrivateKey(seed)
	if err != nil {
		panic(err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  priv.PublicKey().Bytes(),
	}
}

func TestGenerateKATFixtures(t *testing.T) {
	var fixtures KATFixtures

	// 1. Handle Normalization
	normInputs := []string{
		"@student_guest",
		"@bob",
		"STUDENT_ALICE",
		"  @carol  ",
		"guest",
		"study_group",
		"@STUDENT_BOB",
		"campus_dev",
		"alice",
	}
	for _, in := range normInputs {
		fixtures.HandleNormalization = append(fixtures.HandleNormalization, HandleNormFixture{
			Input:    in,
			Expected: CanonicalHandle(in),
		})
	}

	// 2. Pairwise KDF
	pairs := [][2]string{
		{"student_guest", "alice"},
		{"alice", "bob"},
		{"carol", "student_alice"},
		{"@bob", "@carol"},
		{"study_group", "alice"},
	}
	for _, p := range pairs {
		k, err := DerivePairwiseKey(p[0], p[1])
		if err != nil {
			t.Fatal(err)
		}
		u1 := CanonicalHandle(p[0])
		u2 := CanonicalHandle(p[1])
		var f, s string
		if u1 <= u2 {
			f, s = u1, u2
		} else {
			f, s = u2, u1
		}
		info := fmt.Sprintf("CampusNet-v1-Pairwise:%d:%s:%d:%s", len(f), f, len(s), s)
		fixtures.PairwiseKDF = append(fixtures.PairwiseKDF, PairwiseKDFFixture{
			User1:         p[0],
			User2:         p[1],
			Normalized1:   u1,
			Normalized2:   u2,
			InfoString:    info,
			DerivedKeyHex: hex.EncodeToString(k),
		})
	}

	// 3. AEAD Encrypt / Decrypt
	aeadKey, _ := DerivePairwiseKey("student_guest", "alice")
	iv := []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21}
	plaintext := "Hello Alice. Confidential ephemeral notes for Lab 4."
	aad := "alice:student_guest:context_v1"

	block, err := newCipherBlock(aeadKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, iv, []byte(plaintext), []byte(aad))
	tagStart := len(sealed) - 16
	cipherBytes := sealed[:tagStart]
	tagBytes := sealed[tagStart:]

	fixtures.AEAD = AEADFixture{
		KeyHex:        hex.EncodeToString(aeadKey),
		IVBase64:      base64.StdEncoding.EncodeToString(iv),
		Plaintext:     plaintext,
		AAD:           aad,
		CiphertextB64: base64.StdEncoding.EncodeToString(cipherBytes),
		AuthTagB64:    base64.StdEncoding.EncodeToString(tagBytes),
	}

	// 3b. TypeScript-generated AEAD (for cross-implementation TS-encrypt -> Go-decrypt verification)
	fixtures.TSAEAD = AEADFixture{
		KeyHex:        "6f5ff0716d0954a05f2630f8c50d0579110acaa25d3124ae494606f4dfd537cf",
		IVBase64:      "AQIDBAUGBwgJCgsM",
		Plaintext:     "Response from Web to CLI: Lab notebook received and verified.",
		AAD:           "context_web_to_cli_v1",
		CiphertextB64: "PEs3WhyI4uFLs8b79DChkcud3JoAlmSwT5ptWQj33S0ZDAwv9bQw29oQNC8zVCVc5FGdcdPETNeowGWO+w==",
		AuthTagB64:    "wLa+1uc2dGshgGPVbs+PhA==",
	}

	// 4. X3DH Shared Secret
	aliceIK := deterministicKeyPair(10)
	aliceEK := deterministicKeyPair(20)
	bobIK := deterministicKeyPair(30)
	bobSPK := deterministicKeyPair(40)
	bobOPK := deterministicKeyPair(50)

	// Bob signs SPK with Ed25519
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 60)
	}
	bobSignPriv := ed25519.NewKeyFromSeed(seed)
	bobSignPub := bobSignPriv.Public().(ed25519.PublicKey)
	bobSPKSig := ed25519.Sign(bobSignPriv, bobSPK.PublicKey)

	// Compute X3DH shared key
	dh1, _ := ComputeDH(aliceIK.PrivateKey, bobSPK.PublicKey)
	dh2, _ := ComputeDH(aliceEK.PrivateKey, bobIK.PublicKey)
	dh3, _ := ComputeDH(aliceEK.PrivateKey, bobSPK.PublicKey)
	dh4, _ := ComputeDH(aliceEK.PrivateKey, bobOPK.PublicKey)

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

	fixtures.X3DH = X3DHFixture{
		AliceIKPrivHex:     hex.EncodeToString(aliceIK.PrivateKey.Bytes()),
		AliceIKPubHex:      hex.EncodeToString(aliceIK.PublicKey),
		AliceEKPrivHex:     hex.EncodeToString(aliceEK.PrivateKey.Bytes()),
		AliceEKPubHex:      hex.EncodeToString(aliceEK.PublicKey),
		BobIKPrivHex:       hex.EncodeToString(bobIK.PrivateKey.Bytes()),
		BobIKPubHex:        hex.EncodeToString(bobIK.PublicKey),
		BobSPKPrivHex:      hex.EncodeToString(bobSPK.PrivateKey.Bytes()),
		BobSPKPubHex:       hex.EncodeToString(bobSPK.PublicKey),
		BobOPKPrivHex:      hex.EncodeToString(bobOPK.PrivateKey.Bytes()),
		BobOPKPubHex:       hex.EncodeToString(bobOPK.PublicKey),
		BobSignPrivHex:     hex.EncodeToString(bobSignPriv),
		BobSignPubHex:      hex.EncodeToString(bobSignPub),
		BobSPKSigHex:       hex.EncodeToString(bobSPKSig),
		MasterSharedKeyHex: hex.EncodeToString(masterKey),
	}

	// 5. Symmetric Ratchet KDF-Chain (First 5 message keys from known root)
	currCK := make([]byte, 32)
	copy(currCK, masterKey)
	for step := 0; step < 5; step++ {
		nextCK, msgKey, err := kdfChain(currCK)
		if err != nil {
			t.Fatal(err)
		}
		fixtures.RatchetKDFChain = append(fixtures.RatchetKDFChain, RatchetChainStep{
			Step:          step,
			ChainKeyInHex: hex.EncodeToString(currCK),
			NextCKHex:     hex.EncodeToString(nextCK),
			MessageKeyHex: hex.EncodeToString(msgKey),
		})
		currCK = nextCK
	}

	// 6. Envelope Wire Serialization
	testEnv := protocol.NewEnvelope("alice", "bob", 10000)
	testEnv.ID = "11111111-2222-3333-4444-555555555555"
	testEnv.TimestampMS = 1729680000000
	testEnv.EphemeralPublicKey = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	testEnv.IV = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	testEnv.Ciphertext = []byte("encrypted_test_payload_bytes_32b")
	testEnv.AuthTag = []byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	wireBytes, err := json.Marshal(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	fixtures.EnvelopeWire = EnvelopeWireFixture{
		Envelope: testEnv,
		WireJSON: string(wireBytes),
	}

	// Save to testdata/crypto_kat_fixtures.json
	outPath := filepath.Join("..", "..", "testdata", "crypto_kat_fixtures.json")
	data, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Generated %s successfully", outPath)
}
