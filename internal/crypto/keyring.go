package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrSecretNotFound = errors.New("secret not found in secure storage")
)

const (
	CampusNetService = "CampusNet_Keyring"
)

// SecureStore provides memory-protected persistence for device secrets and ratchet state.
type SecureStore struct {
	mu        sync.Mutex
	memoryMap map[string][]byte
	storePath string
}

// NewSecureStore initializes secure local storage.
func NewSecureStore() (*SecureStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	appDir := filepath.Join(home, ".campusnet")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return nil, fmt.Errorf("creating secure config dir: %w", err)
	}

	return &SecureStore{
		memoryMap: make(map[string][]byte),
		storePath: filepath.Join(appDir, "secrets.enc"),
	}, nil
}

// Set stores a sensitive secret, ensuring previous memory buffers are overwritten.
func (s *SecureStore) Set(key string, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Zero existing memory copy if present
	if old, exists := s.memoryMap[key]; exists {
		ZeroBytes(old)
	}

	// Copy secret into fresh buffer
	buf := make([]byte, len(secret))
	copy(buf, secret)
	s.memoryMap[key] = buf

	// Persist encrypted to local isolated storage
	return s.persistToFile()
}

// Get retrieves a secret from memory or decrypts from isolated storage.
func (s *SecureStore) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if val, ok := s.memoryMap[key]; ok {
		cp := make([]byte, len(val))
		copy(cp, val)
		return cp, nil
	}

	// Try loading from file
	if err := s.loadFromFile(); err != nil {
		return nil, err
	}

	if val, ok := s.memoryMap[key]; ok {
		cp := make([]byte, len(val))
		copy(cp, val)
		return cp, nil
	}

	return nil, ErrSecretNotFound
}

// Delete purges and zeroes a secret from memory and storage.
func (s *SecureStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if val, ok := s.memoryMap[key]; ok {
		ZeroBytes(val)
		delete(s.memoryMap, key)
	}
	return s.persistToFile()
}

// PurgeAll zeroes all in-memory buffers immediately.
func (s *SecureStore) PurgeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, val := range s.memoryMap {
		ZeroBytes(val)
		delete(s.memoryMap, k)
	}
}

func (s *SecureStore) deriveMachineKey() []byte {
	hostname, _ := os.Hostname()
	salt := "CampusNet_Local_Hardware_Binding_v1"
	h := sha256.Sum256([]byte(hostname + salt))
	return h[:]
}

// deriveUserMachineKey binds keyring encryption to both local hardware AND current OS user profile.
// MULTI-USER ISOLATION: Prevents other local users on the same machine from decrypting secrets.
func (s *SecureStore) deriveUserMachineKey() []byte {
	hostname, _ := os.Hostname()
	user := os.Getenv("USERNAME")
	if user == "" {
		user = os.Getenv("USER")
	}
	home, _ := os.UserHomeDir()

	salt := []byte("CampusNet_User_Hardware_Binding_Salt_v2")
	ikm := []byte(fmt.Sprintf("%s:%s:%s", hostname, user, home))
	info := []byte("CampusNet-Keyring-v2-AES256GCM")

	k, err := HKDFDerive(salt, ikm, info, 32)
	if err == nil {
		return k
	}
	h := sha256.Sum256([]byte(hostname + user + "CampusNet_Local_Hardware_Binding_v2"))
	return h[:]
}

func (s *SecureStore) persistToFile() error {
	// User- and machine-bound AES-256-GCM encryption for isolated storage
	key := s.deriveUserMachineKey()
	defer ZeroBytes(key)

	var payload []byte
	for k, v := range s.memoryMap {
		line := fmt.Sprintf("%s=%s\n", k, base64.StdEncoding.EncodeToString(v))
		payload = append(payload, []byte(line)...)
	}

	if len(payload) == 0 {
		_ = os.Remove(s.storePath)
		return nil
	}

	iv, ciphertext, tag, err := EncryptPayload(key, payload, []byte("CampusNet_Keyring_Storage"))
	if err != nil {
		return err
	}

	var combined []byte
	combined = append(combined, iv...)
	combined = append(combined, tag...)
	combined = append(combined, ciphertext...)

	return os.WriteFile(s.storePath, combined, 0600)
}

func (s *SecureStore) loadFromFile() error {
	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(data) < 28 { // 12 iv + 16 tag
		return errors.New("corrupted secret file")
	}

	iv := data[:12]
	tag := data[12:28]
	ciphertext := data[28:]

	// 1. Try decrypting with user-bound key (v2)
	key := s.deriveUserMachineKey()
	plaintext, err := DecryptPayload(key, iv, ciphertext, tag, []byte("CampusNet_Keyring_Storage"))
	ZeroBytes(key)

	// 2. If v2 fails, attempt backward-compatible migration from v1 legacy machine key
	if err != nil {
		v1Key := s.deriveMachineKey()
		plaintext, err = DecryptPayload(v1Key, iv, ciphertext, tag, []byte("CampusNet_Keyring_Storage"))
		ZeroBytes(v1Key)
		if err != nil {
			return fmt.Errorf("decrypting local keyring store: %w", err)
		}
		// Upgrade to v2 immediately on successful legacy decryption
		defer func() {
			_ = s.persistToFile()
		}()
	}
	defer ZeroBytes(plaintext)

	// Parse lines
	var currentKey, currentVal []byte
	isKey := true
	for _, b := range plaintext {
		if b == '=' && isKey {
			isKey = false
			continue
		}
		if b == '\n' {
			if len(currentKey) > 0 {
				decoded, err := base64.StdEncoding.DecodeString(string(currentVal))
				if err == nil {
					s.memoryMap[string(currentKey)] = decoded
				}
			}
			currentKey = nil
			currentVal = nil
			isKey = true
			continue
		}
		if isKey {
			currentKey = append(currentKey, b)
		} else {
			currentVal = append(currentVal, b)
		}
	}

	return nil
}
