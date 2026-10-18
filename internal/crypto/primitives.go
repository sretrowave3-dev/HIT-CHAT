package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

var (
	ErrDecryptionFailed = errors.New("aead decryption or authentication failed")
	ErrKeyLength        = errors.New("invalid key length")
	ErrNonceLength      = errors.New("invalid nonce length")
)

// KeyPair represents an X25519 ECDH key pair.
type KeyPair struct {
	PrivateKey *ecdh.PrivateKey
	PublicKey  []byte // 32 bytes
}

// GenerateKeyPair generates a new Curve25519 (X25519) key pair using crypto/rand.
func GenerateKeyPair() (*KeyPair, error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating X25519 key: %w", err)
	}

	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  priv.PublicKey().Bytes(),
	}, nil
}

// KeyPairFromPrivateBytes reconstructs a KeyPair from raw 32-byte private key.
func KeyPairFromPrivateBytes(privBytes []byte) (*KeyPair, error) {
	curve := ecdh.X25519()
	priv, err := curve.NewPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("reconstructing private key: %w", err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  priv.PublicKey().Bytes(),
	}, nil
}

// ComputeDH computes the 32-byte Diffie-Hellman shared secret between local private key and remote public key.
func ComputeDH(localPriv *ecdh.PrivateKey, remotePubKeyBytes []byte) ([]byte, error) {
	curve := ecdh.X25519()
	remotePub, err := curve.NewPublicKey(remotePubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing remote public key: %w", err)
	}

	secret, err := localPriv.ECDH(remotePub)
	if err != nil {
		return nil, fmt.Errorf("ecdh computation failed: %w", err)
	}
	return secret, nil
}

// HKDFExtract implements HKDF-Extract using HMAC-SHA256 (RFC 5869).
func HKDFExtract(salt, ikm []byte) []byte {
	if len(salt) == 0 {
		salt = make([]byte, 32) // default salt is HashLen zeros
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	return mac.Sum(nil)
}

// HKDFExpand implements HKDF-Expand using HMAC-SHA256 (RFC 5869).
func HKDFExpand(prk, info []byte, length int) ([]byte, error) {
	hashLen := 32
	n := (length + hashLen - 1) / hashLen
	if n > 255 {
		return nil, errors.New("hkdf requested length exceeds maximum bound")
	}

	okm := make([]byte, 0, n*hashLen)
	var t []byte

	for i := 1; i <= n; i++ {
		mac := hmac.New(sha256.New, prk)
		mac.Write(t)
		mac.Write(info)
		mac.Write([]byte{byte(i)})
		t = mac.Sum(nil)
		okm = append(okm, t...)
	}

	return okm[:length], nil
}

// HKDFDerive performs full HKDF (Extract + Expand) returning derived key material.
func HKDFDerive(salt, ikm, info []byte, length int) ([]byte, error) {
	prk := HKDFExtract(salt, ikm)
	return HKDFExpand(prk, info, length)
}

// EncryptPayload encrypts plaintext using standard AES-256-GCM (or ChaCha20-Poly1305) with 12-byte random IV.
// It returns (iv, ciphertextWithoutTag, authTag, error).
func EncryptPayload(key, plaintext, associatedData []byte) ([]byte, []byte, []byte, error) {
	if len(key) != 32 {
		return nil, nil, nil, ErrKeyLength
	}

	// Using standard AES-256-GCM for universal hardware acceleration & standard lib support
	block, err := newCipherBlock(key)
	if err != nil {
		return nil, nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, nil, err
	}

	// Seal appends tag (16 bytes) to the end of ciphertext
	sealed := gcm.Seal(nil, nonce, plaintext, associatedData)
	tagStart := len(sealed) - 16
	ciphertext := sealed[:tagStart]
	tag := sealed[tagStart:]

	return nonce, ciphertext, tag, nil
}

// DecryptPayload verifies the auth tag and decrypts the ciphertext.
func DecryptPayload(key, nonce, ciphertext, authTag, associatedData []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrKeyLength
	}
	if len(nonce) != 12 {
		return nil, ErrNonceLength
	}

	block, err := newCipherBlock(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	sealed := make([]byte, 0, len(ciphertext)+len(authTag))
	sealed = append(sealed, ciphertext...)
	sealed = append(sealed, authTag...)

	plaintext, err := gcm.Open(nil, nonce, sealed, associatedData)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// ZeroBytes overwrites sensitive byte slices with zeros to protect memory from memory dumps.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func newCipherBlock(key []byte) (cipher.Block, error) {
	// AES-256 block creation
	return aes.NewCipher(key)
}
