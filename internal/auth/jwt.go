package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrTokenExpired = errors.New("authentication token has expired")
	ErrTokenInvalid = errors.New("invalid signature or malformed token")
)

type DeviceClaims struct {
	DeviceID   string `json:"device_id"`
	UserHandle string `json:"user_handle"`
	Platform   string `json:"platform"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
}

type TokenManager struct {
	secretKey []byte
}

func NewTokenManager(secret string) *TokenManager {
	if secret == "" {
		secret = "campusnet-dev-session-secret-change-in-production-2026"
	}
	return &TokenManager{
		secretKey: []byte(secret),
	}
}

// GenerateDeviceToken signs a compact JWT containing device claims.
func (tm *TokenManager) GenerateDeviceToken(deviceID, userHandle, platform string) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	claims := DeviceClaims{
		DeviceID:   deviceID,
		UserHandle: userHandle,
		Platform:   platform,
		IssuedAt:   time.Now().Unix(),
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signatureInput := fmt.Sprintf("%s.%s", headerB64, claimsB64)

	mac := hmac.New(sha256.New, tm.secretKey)
	mac.Write([]byte(signatureInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s.%s", headerB64, claimsB64, sigB64), nil
}

// ValidateToken parses and verifies the signature and expiration of a JWT token.
func (tm *TokenManager) ValidateToken(tokenString string) (*DeviceClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}

	// 1. Algorithm Pinning: strictly enforce HS256 algorithm (reject "none" or alg confusion attacks)
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrTokenInvalid
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Alg != "HS256" {
		return nil, ErrTokenInvalid
	}

	// 2. Cryptographic Signature Verification using constant-time comparison
	signatureInput := fmt.Sprintf("%s.%s", parts[0], parts[1])
	mac := hmac.New(sha256.New, tm.secretKey)
	mac.Write([]byte(signatureInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrTokenInvalid
	}

	// 3. Claims deserialization and temporal checks
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	var claims DeviceClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, ErrTokenInvalid
	}

	now := time.Now().Unix()
	if now > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}
	// Defend against future-dated token fraud
	if claims.IssuedAt > now+300 {
		return nil, ErrTokenInvalid
	}

	return &claims, nil
}
