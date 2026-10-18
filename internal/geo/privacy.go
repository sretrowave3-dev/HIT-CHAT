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

	if req.BSSIDHash != "" {
		if zone, found := f.resolver.ResolveZone(req.BSSIDHash); found {
			zoneCode = zone.ZoneCode
			displayZone = zone.DisplayName
			buildingName = zone.BuildingName
			floorLevel = zone.FloorLevel
		}
	}

	return &protocol.PresencePayload{
		UserHandle:      userHandle,
		BSSIDHash:       req.BSSIDHash,
		ZoneCode:        zoneCode,
		DisplayZone:     displayZone,
		BuildingName:    buildingName,
		FloorLevel:      floorLevel,
		LocationEnabled: true,
		Latitude:        req.Latitude,
		Longitude:       req.Longitude,
	}
}
