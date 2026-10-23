package geo

import (
	"github.com/hit-chat/campusnet/pkg/protocol"
)

// ZeroBypassPrivacyFilter ensures client location privacy is strictly enforced.
// If location_enabled == false, telemetry is stripped and peers receive only null vectors.
type ZeroBypassPrivacyFilter struct {
	resolver *BSSIDResolver
}

func NewPrivacyFilter(resolver *BSSIDResolver) *ZeroBypassPrivacyFilter {
	return &ZeroBypassPrivacyFilter{
		resolver: resolver,
	}
}

// SanitizePresence processes incoming presence telemetry and applies strict privacy enforcement.
func (f *ZeroBypassPrivacyFilter) SanitizePresence(userHandle string, req *protocol.PresencePayload) *protocol.PresencePayload {
	// If privacy switch is active (location disabled), immediately scrub all telemetry
	if req == nil || !req.LocationEnabled {
		return &protocol.PresencePayload{
			UserHandle:      userHandle,
			BSSIDHash:       "",
			ZoneCode:        "PROTECTED",
			DisplayZone:     "Privacy Protected (Location Concealed)",
			BuildingName:    "Unknown",
			FloorLevel:      0,
			LocationEnabled: false,
			Latitude:        nil,
			Longitude:       nil,
		}
	}

	// Location is explicitly enabled by user
	zoneCode := req.ZoneCode
	displayZone := req.DisplayZone
	buildingName := req.BuildingName
	floorLevel := req.FloorLevel
	bHash := req.BSSIDHash

	if bHash != "" {
		// Zero-Knowledge Privacy Invariant: raw physical MACs are NEVER leaked or broadcast.
		// If client passed a raw MAC (e.g. colon/dash separated or short string) rather than HMAC-SHA256,
		// hash it immediately using the campus salt.
		isHex64 := len(bHash) == 64
		if isHex64 {
			for _, c := range bHash {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					isHex64 = false
					break
				}
			}
		}
		if !isHex64 {
			bHash = f.resolver.HashBSSID(bHash)
		}

		if zone, found := f.resolver.ResolveZone(bHash); found {
			zoneCode = zone.ZoneCode
			displayZone = zone.DisplayName
			buildingName = zone.BuildingName
			floorLevel = zone.FloorLevel
		}
	}

	return &protocol.PresencePayload{
		UserHandle:      userHandle,
		BSSIDHash:       bHash,
		ZoneCode:        zoneCode,
		DisplayZone:     displayZone,
		BuildingName:    buildingName,
		FloorLevel:      floorLevel,
		LocationEnabled: true,
		Latitude:        req.Latitude,
		Longitude:       req.Longitude,
	}
}
