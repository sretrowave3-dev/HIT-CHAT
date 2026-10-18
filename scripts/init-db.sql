-- CampusNet Spatial & Cryptographic Directory Schema
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "postgis";

-- Campus Zones Table (Polygonal Geofences)
CREATE TABLE IF NOT EXISTS campus_zones (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    zone_code VARCHAR(64) UNIQUE NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    building_name VARCHAR(128) NOT NULL,
    floor_level INT DEFAULT 1,
    polygon GEOMETRY(Polygon, 4326),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Campus Wi-Fi Access Point Hashes (HMAC-SHA256 of BSSIDs)
CREATE TABLE IF NOT EXISTS access_point_hashes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bssid_hash VARCHAR(64) UNIQUE NOT NULL, -- SHA-256 / HMAC hash of BSSID
    zone_code VARCHAR(64) REFERENCES campus_zones(zone_code) ON DELETE CASCADE,
    description VARCHAR(255),
    signal_threshold INT DEFAULT -80,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Registered Devices & Users (Device Authorization Flow)
CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id VARCHAR(128) UNIQUE NOT NULL,
    user_handle VARCHAR(64) NOT NULL,
    device_name VARCHAR(128),
    platform VARCHAR(32), -- 'cli', 'web', 'desktop'
    public_identity_key TEXT NOT NULL, -- Base64 X25519 / Ed25519
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Pre-Key Bundles for X3DH Handshake (Public Keys Only)
CREATE TABLE IF NOT EXISTS prekey_bundles (
    device_id VARCHAR(128) PRIMARY KEY REFERENCES devices(device_id) ON DELETE CASCADE,
    signed_prekey TEXT NOT NULL,
    signed_prekey_signature TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- One-Time Prekeys (OTK Pool)
CREATE TABLE IF NOT EXISTS one_time_prekeys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id VARCHAR(128) REFERENCES devices(device_id) ON DELETE CASCADE,
    key_id INT NOT NULL,
    public_key TEXT NOT NULL,
    consumed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(device_id, key_id)
);

-- Spatial Indexes
CREATE INDEX IF NOT EXISTS idx_campus_zones_geom ON campus_zones USING GIST (polygon);
CREATE INDEX IF NOT EXISTS idx_ap_bssid_hash ON access_point_hashes(bssid_hash);

-- Seed Initial Campus Zones
INSERT INTO campus_zones (zone_code, display_name, building_name, floor_level)
VALUES 
    ('LIB_FL2_SILENT', 'Library 2nd Floor - Silent Study', 'Main Library', 2),
    ('ENG_LAB_4', 'Engineering Hall - Robotics Lab 4', 'Turing Engineering Complex', 1),
    ('CAMPUS_QUAD', 'Central Quadrangle Lawn', 'Outdoor Common', 0),
    ('STUDENT_UNION', 'Student Union - Coffee & Commons', 'Campus Center', 1)
ON CONFLICT (zone_code) DO NOTHING;
