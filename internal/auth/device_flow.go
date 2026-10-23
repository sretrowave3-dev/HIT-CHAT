package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

var (
	ErrInvalidUserCode   = errors.New("invalid or expired user verification code")
	ErrCodeAlreadyUsed   = errors.New("device code already authorized")
	ErrDeviceFlowExpired = errors.New("device authorization request has expired")
	ErrTooManyAttempts   = errors.New("too many failed verification attempts, please retry later")
)

const (
	MaxPendingSessions = 500
)

type DeviceAuthorizationState string

const (
	StatePending    DeviceAuthorizationState = "PENDING"
	StateAuthorized DeviceAuthorizationState = "AUTHORIZED"
	StateDenied     DeviceAuthorizationState = "DENIED"
	StateExpired    DeviceAuthorizationState = "EXPIRED"
)

type PendingDeviceSession struct {
	DeviceCode          string
	UserCode            string
	ClientID            string
	Platform            string
	PublicIdentityKey   string
	RequestedUserHandle string
	State               DeviceAuthorizationState
	UserHandle          string
	DeviceID            string
	ExpiresAt           time.Time
	CreatedAt           time.Time
}

// DeviceFlowManager implements the RFC 8628 Device Authorization Flow.
type DeviceFlowManager struct {
	mu              sync.RWMutex
	byDeviceCode    map[string]*PendingDeviceSession
	byUserCode      map[string]*PendingDeviceSession
	failedAttempts  map[string]int
	lastFailedReset time.Time
	tokenManager    *TokenManager
	verificationURI string
}

// NewDeviceFlowManager creates a new Device Flow Manager.
func NewDeviceFlowManager(tokenMgr *TokenManager, verificationURI string) *DeviceFlowManager {
	if verificationURI == "" {
		verificationURI = "http://localhost:5173/pair"
	}
	dfm := &DeviceFlowManager{
		byDeviceCode:    make(map[string]*PendingDeviceSession),
		byUserCode:      make(map[string]*PendingDeviceSession),
		failedAttempts:  make(map[string]int),
		lastFailedReset: time.Now(),
		tokenManager:    tokenMgr,
		verificationURI: verificationURI,
	}

	go dfm.cleanupSweeper(1 * time.Minute)
	return dfm
}

// RequestCode initializes a new device login session and returns verification parameters.
func (d *DeviceFlowManager) RequestCode(req *protocol.DeviceCodeRequest) (*protocol.DeviceCodeResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Enforce capacity bounds to defend against memory exhaustion DoS attacks
	if len(d.byDeviceCode) >= MaxPendingSessions {
		now := time.Now()
		for code, s := range d.byDeviceCode {
			if now.After(s.ExpiresAt) {
				delete(d.byDeviceCode, code)
				delete(d.byUserCode, s.UserCode)
			}
		}
		if len(d.byDeviceCode) >= MaxPendingSessions {
			var oldestCode string
			var oldestTime time.Time
			for code, s := range d.byDeviceCode {
				if oldestTime.IsZero() || s.CreatedAt.Before(oldestTime) {
					oldestTime = s.CreatedAt
					oldestCode = code
				}
			}
			if s, ok := d.byDeviceCode[oldestCode]; ok {
				delete(d.byUserCode, s.UserCode)
				delete(d.byDeviceCode, oldestCode)
			}
		}
	}

	deviceCode := generateRandomHex(16)
	userCode := generateHumanCode()
	expiresIn := 600 // 10 minutes

	session := &PendingDeviceSession{
		DeviceCode:          deviceCode,
		UserCode:            userCode,
		ClientID:            req.ClientID,
		Platform:            req.Platform,
		PublicIdentityKey:   req.PublicIdentityKey,
		RequestedUserHandle: req.UserHandle,
		State:               StatePending,
		ExpiresAt:           time.Now().Add(time.Duration(expiresIn) * time.Second),
		CreatedAt:           time.Now(),
	}

	d.byDeviceCode[deviceCode] = session
	d.byUserCode[userCode] = session

	return &protocol.DeviceCodeResponse{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: d.verificationURI,
		ExpiresIn:       expiresIn,
		Interval:        2, // Polling interval in seconds
	}, nil
}

// PollToken checks if the device code has been approved and returns an access token.
func (d *DeviceFlowManager) PollToken(deviceCode, clientID string) (*protocol.DeviceTokenResponse, error) {
	d.mu.RLock()
	session, exists := d.byDeviceCode[deviceCode]
	d.mu.RUnlock()

	if !exists {
		return nil, protocol.ErrDeviceCodeExpired
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, protocol.ErrDeviceCodeExpired
	}

	switch session.State {
	case StatePending:
		return nil, protocol.ErrDevicePending
	case StateDenied:
		return nil, protocol.ErrDeviceDenied
	case StateAuthorized:
		// Generate JWT token for this device session
		token, err := d.tokenManager.GenerateDeviceToken(session.DeviceID, session.UserHandle, session.Platform)
		if err != nil {
			return nil, fmt.Errorf("generating token: %w", err)
		}

		// Purge session after successful redemption
		d.mu.Lock()
		delete(d.byDeviceCode, deviceCode)
		delete(d.byUserCode, session.UserCode)
		d.mu.Unlock()

		return &protocol.DeviceTokenResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   86400 * 30, // 30 days
			DeviceID:    session.DeviceID,
			UserHandle:  session.UserHandle,
		}, nil
	default:
		return nil, protocol.ErrDeviceCodeExpired
	}
}

// VerifyUserCode approves a pending device session using the human-readable user code.
func (d *DeviceFlowManager) VerifyUserCode(userCode, userHandle string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	normCode := strings.ToUpper(strings.TrimSpace(userCode))
	now := time.Now()

	// Reset failure tracker periodically
	if now.Sub(d.lastFailedReset) > 60*time.Second {
		d.failedAttempts = make(map[string]int)
		d.lastFailedReset = now
	}

	// Rate-limit brute-force attempts on codes
	if d.failedAttempts[normCode] >= 5 || d.failedAttempts["__global__"] >= 15 {
		return ErrTooManyAttempts
	}

	session, exists := d.byUserCode[normCode]
	if !exists {
		d.failedAttempts[normCode]++
		d.failedAttempts["__global__"]++
		return ErrInvalidUserCode
	}

	if time.Now().After(session.ExpiresAt) {
		delete(d.byUserCode, normCode)
		delete(d.byDeviceCode, session.DeviceCode)
		return ErrDeviceFlowExpired
	}

	if session.State == StateAuthorized {
		return ErrCodeAlreadyUsed
	}

	session.State = StateAuthorized
	session.UserHandle = userHandle
	session.DeviceID = fmt.Sprintf("dev-%s-%s", session.Platform, generateRandomHex(6))

	// Reset attempts on successful verification
	delete(d.failedAttempts, normCode)

	return nil
}

// GetPendingSessions returns all active device authorization sessions awaiting approval.
func (d *DeviceFlowManager) GetPendingSessions() []*PendingDeviceSession {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now()
	var pending []*PendingDeviceSession
	for _, s := range d.byDeviceCode {
		if s.State == StatePending && now.Before(s.ExpiresAt) {
			pending = append(pending, s)
		}
	}
	return pending
}

// AutoAuthorizeLatest finds the most recent pending session and approves it immediately.
func (d *DeviceFlowManager) AutoAuthorizeLatest(userHandle string) (*PendingDeviceSession, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if userHandle == "" {
		userHandle = "cli_user"
	}
	cleanTarget := strings.TrimPrefix(strings.ToLower(userHandle), "@")
	cleanTarget = strings.TrimPrefix(cleanTarget, "student_")

	now := time.Now()
	var latest *PendingDeviceSession
	for _, s := range d.byDeviceCode {
		if s.State == StatePending && now.Before(s.ExpiresAt) {
			// Defend against session hijack: only authorize if requested handle matches
			if s.RequestedUserHandle != "" {
				sReq := strings.TrimPrefix(strings.ToLower(s.RequestedUserHandle), "@")
				sReq = strings.TrimPrefix(sReq, "student_")
				if sReq != cleanTarget {
					continue // Different requested user, do not auto-authorize!
				}
			}
			if latest == nil || s.CreatedAt.After(latest.CreatedAt) {
				latest = s
			}
		}
	}

	if latest == nil {
		return nil, errors.New("no pending device authorization requests found for user")
	}

	latest.State = StateAuthorized
	latest.UserHandle = userHandle
	latest.DeviceID = fmt.Sprintf("dev-%s-%s", latest.Platform, generateRandomHex(6))

	return latest, nil
}

func (d *DeviceFlowManager) cleanupSweeper(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for code, s := range d.byDeviceCode {
			if now.After(s.ExpiresAt) {
				delete(d.byDeviceCode, code)
				delete(d.byUserCode, s.UserCode)
			}
		}
		d.mu.Unlock()
	}
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func generateHumanCode() string {
	chars := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	part1 := make([]byte, 4)
	part2 := make([]byte, 4)
	for i := 0; i < 4; i++ {
		part1[i] = chars[int(b[i])%len(chars)]
		part2[i] = chars[int(b[i+4])%len(chars)]
	}
	return fmt.Sprintf("CNET-%s-%s", string(part1), string(part2))
}
