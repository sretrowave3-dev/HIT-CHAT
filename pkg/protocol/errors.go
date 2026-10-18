package protocol

import "errors"

var (
	ErrDeviceCodeExpired   = errors.New("device code has expired")
	ErrDevicePending       = errors.New("authorization pending: user has not approved the device code")
	ErrDeviceDenied        = errors.New("authorization denied by user")
	ErrPreKeyExhausted     = errors.New("one-time pre-keys exhausted for device")
	ErrBundleNotFound      = errors.New("pre-key bundle not found for target device")
	ErrBurnedOrUnlinked    = errors.New("message envelope has been burned or unlinked")
	ErrPrivacyRestricted   = errors.New("location data dropped due to active privacy switch")
	ErrUnauthorized        = errors.New("unauthorized request or invalid device token")
)
