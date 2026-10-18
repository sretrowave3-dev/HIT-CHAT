package geo

import (
	"testing"

	"github.com/hit-chat/campusnet/pkg/protocol"
)

func TestBSSIDHashingAndResolution(t *testing.T) {
	resolver := NewBSSIDResolver("campus_test_salt")

	rawBSSID := "00:14:22:01:23:45"
	zoneCode := "LIB_FL2_SILENT"

	bssidHash := resolver.AssociateBSSID(rawBSSID, zoneCode)
	if len(bssidHash) != 64 {
		t.Fatalf("expected 64-char hex SHA256 hash, got %d chars", len(bssidHash))
	}

	zone, found := resolver.ResolveZone(bssidHash)
	if !found {
		t.Fatalf("failed to resolve zone for hash %s", bssidHash)
	}

	if zone.ZoneCode != zoneCode {
		t.Fatalf("zone code mismatch: got %s, want %s", zone.ZoneCode, zoneCode)
	}
}

func TestZeroBypassPrivacyFilter(t *testing.T) {
	resolver := NewBSSIDResolver("campus_test_salt")
	filter := NewPrivacyFilter(resolver)

	lat := 37.7749
	lon := -122.4194

	// Case 1: Location Privacy ACTIVE (location_enabled = false)
	reqProtected := &protocol.PresencePayload{
		UserHandle:      "student_alice",
		BSSIDHash:       "00112233445566778899aabbccddeeff",
		LocationEnabled: false,
		Latitude:        &lat,
		Longitude:       &lon,
	}

	sanitized := filter.SanitizePresence("student_alice", reqProtected)
	if sanitized.LocationEnabled {
		t.Fatalf("expected location_enabled to remain false")
	}
	if sanitized.BSSIDHash != "" {
		t.Fatalf("expected BSSID hash to be scrubbed, got: %s", sanitized.BSSIDHash)
	}
	if sanitized.Latitude != nil || sanitized.Longitude != nil {
		t.Fatalf("expected GPS coordinates to be stripped to null vectors")
	}
	if sanitized.ZoneCode != "PROTECTED" {
		t.Fatalf("expected zone code PROTECTED, got %s", sanitized.ZoneCode)
	}

	// Case 2: Location Privacy INACTIVE (location_enabled = true)
	bssidHash := resolver.AssociateBSSID("00:14:22:01:23:45", "LIB_FL2_SILENT")
	reqPublic := &protocol.PresencePayload{
		UserHandle:      "student_bob",
		BSSIDHash:       bssidHash,
		LocationEnabled: true,
		Latitude:        &lat,
		Longitude:       &lon,
	}

	publicPresence := filter.SanitizePresence("student_bob", reqPublic)
	if !publicPresence.LocationEnabled {
		t.Fatalf("expected location_enabled to be true")
	}
	if publicPresence.ZoneCode != "LIB_FL2_SILENT" {
		t.Fatalf("expected resolved zone LIB_FL2_SILENT, got %s", publicPresence.ZoneCode)
	}
	if publicPresence.Latitude == nil || *publicPresence.Latitude != lat {
		t.Fatalf("expected latitude to be preserved when location_enabled is true")
	}
}
