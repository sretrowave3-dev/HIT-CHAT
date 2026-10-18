package geo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
)

const (
	DefaultCampusSalt = "CampusNet_Salt_SecureCampus_v1"
)

// CampusZone represents a physical room or building zone.
type CampusZone struct {
	ZoneCode     string `json:"zone_code"`
	DisplayName  string `json:"display_name"`
	BuildingName string `json:"building_name"`
	FloorLevel   int    `json:"floor_level"`
}

// BSSIDResolver resolves Wi-Fi access point hashes to campus zones.
type BSSIDResolver struct {
	mu         sync.RWMutex
	salt       []byte
	hashToZone map[string]CampusZone
}

// NewBSSIDResolver creates a resolver seeded with default campus APs.
func NewBSSIDResolver(campusSalt string) *BSSIDResolver {
	if campusSalt == "" {
		campusSalt = DefaultCampusSalt
	}

	r := &BSSIDResolver{
		salt:       []byte(campusSalt),
		hashToZone: make(map[string]CampusZone),
	}

	// Seed reference campus zones
	r.RegisterZone("LIB_FL2_SILENT", "Library 2nd Floor - Silent Study", "Main Library", 2)
	r.RegisterZone("ENG_LAB_4", "Engineering Hall - Robotics Lab 4", "Turing Engineering Complex", 1)
	r.RegisterZone("STUDENT_UNION", "Student Union - Coffee & Commons", "Campus Center", 1)
	r.RegisterZone("CAMPUS_QUAD", "Central Quadrangle Lawn", "Outdoor Common", 0)

	// Pre-hash known reference campus BSSIDs for demonstration
	r.AssociateBSSID("00:14:22:01:23:45", "LIB_FL2_SILENT")
	r.AssociateBSSID("00:14:22:67:89:ab", "ENG_LAB_4")
	r.AssociateBSSID("00:14:22:cd:ef:01", "STUDENT_UNION")

	return r
}

// NormalizeBSSID normalizes MAC address strings to standard lower-case colon-separated format.
func NormalizeBSSID(bssid string) string {
	cleaned := strings.TrimSpace(strings.ToLower(bssid))
	cleaned = strings.ReplaceAll(cleaned, "-", ":")
	return cleaned
}

// HashBSSID computes an HMAC-SHA256 hex string from a raw BSSID using the campus secret salt.
// RAW BSSIDs ARE NEVER BROADCAST OR STORED ON SERVER.
func (r *BSSIDResolver) HashBSSID(rawBSSID string) string {
	norm := NormalizeBSSID(rawBSSID)
	mac := hmac.New(sha256.New, r.salt)
	mac.Write([]byte(norm))
	return hex.EncodeToString(mac.Sum(nil))
}

// RegisterZone adds a campus location zone definition.
func (r *BSSIDResolver) RegisterZone(code, displayName, building string, floor int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hashToZone[code] = CampusZone{
		ZoneCode:     code,
		DisplayName:  displayName,
		BuildingName: building,
		FloorLevel:   floor,
	}
}

// AssociateBSSID binds a known physical access point MAC to a campus zone.
func (r *BSSIDResolver) AssociateBSSID(rawBSSID, zoneCode string) string {
	bssidHash := r.HashBSSID(rawBSSID)
	r.mu.Lock()
	defer r.mu.Unlock()

	if zone, ok := r.hashToZone[zoneCode]; ok {
		r.hashToZone[bssidHash] = zone
	}
	return bssidHash
}

// ResolveZone maps a BSSID hash to its designated physical campus zone.
func (r *BSSIDResolver) ResolveZone(bssidHash string) (*CampusZone, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	zone, exists := r.hashToZone[bssidHash]
	if !exists {
		return nil, false
	}
	return &zone, true
}
