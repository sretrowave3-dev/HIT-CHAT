<<<<<<< HEAD
# CampusNet: High-Security Ephemeral Communications Network

CampusNet is a high-security, campus-focused ephemeral communications network engineered with zero-knowledge end-to-end encryption (E2EE), client-side Double Ratchet and Extended Triple Diffie-Hellman (X3DH) key agreement, forward secrecy, sub-second burn-on-read ephemerality, and privacy-preserving campus access point (BSSID) location zones.

---

## Key Security & Architectural Invariants

1. **Zero-Knowledge Blind Relay**: Plaintext message bodies and private cryptographic keys **never** touch the server or unencrypted storage. The server operates strictly as a blind relay and public-key bundle registry.
2. **Sub-Second Burn-on-Read**: When an encrypted envelope is delivered and acknowledged (`ACK_READ`), an atomic Redis script unlinks and cryptographically zeroes the message buffer in memory.
3. **Double Ratchet with Forward Secrecy**: Every conversation utilizes symmetric KDF chain ratchets and Diffie-Hellman ratchets. Out-of-order messages are handled through a bounded skipped-key cache.
4. **BSSID Privacy Zero-Bypass**: Campus Wi-Fi AP BSSIDs are hashed using HMAC-SHA256 with campus salt. When the privacy toggle is disabled (`location_enabled == false`), telemetry is completely scrubbed and null vectors are dispatched to peers.

---

## Directory Layout

```
hit-chat/
├── memory-roadmap.md                 # Context ledger & operational execution roadmap
├── Makefile                           # Unified build/run automation
├── docker-compose.yml                 # Local Redis 7.2 & PostGIS 16 services
├── go.mod                             # Go module definition
├── api/
│   ├── proto/v1/
│   │   ├── envelope.proto             # Cryptographic wire envelope & stream framing
│   │   ├── auth.proto                 # RFC 8628 device flow & bundle registry
│   │   └── geo.proto                  # BSSID resolution & geofencing
│   └── openapi/
│       └── openapi.yaml               # OpenAPI 3.0 REST specification
├── cmd/
│   ├── server/
│   │   └── main.go                    # Main backend gateway entrypoint
│   └── cli/
│       ├── main.go                    # CLI router (TUI, send, listen, pair)
│       ├── tui.go                     # Interactive split-screen Bubble Tea TUI
│       └── headless.go                # Non-interactive POSIX streaming pipelines
├── internal/
│   ├── auth/                          # RFC 8628 device flow, JWT, bundle registry
│   ├── crypto/                        # X3DH, Double Ratchet, Curve25519, AEAD, Keyring
│   ├── ephemeral/                     # Millisecond TTL, Redis & Memory burn stores
│   ├── geo/                           # HMAC BSSID hashing, privacy filter, Wi-Fi sniffers
│   └── transport/                     # WebSocket hub, REST routing, CORS middleware
├── pkg/
│   └── protocol/                      # Wire envelopes, message opcodes, errors
├── web/                               # React + Vite + Tailwind CSS SPA
└── desktop/                           # Tauri v2 desktop executable configuration
```

---

## Quickstart

### 1. Backend Server
```bash
# Run with in-memory ephemeral store
go run ./cmd/server

# Or run with Redis container
docker compose up -d redis
go run ./cmd/server --redis localhost:6379
```

### 2. Terminal CLI Client
```bash
# Launch interactive split-screen TUI
go run ./cmd/cli

# Send ephemeral message with 5s burn timer
go run ./cmd/cli send --to @alice --ttl 5000 "Meet at Robotics Lab 4"

# Non-interactive POSIX pipe
echo "Secret coordinates" | go run ./cmd/cli send --to @bob

# Stream incoming messages in JSON format
go run ./cmd/cli listen --format json
```

### 3. Web Client
```bash
cd web
npm install
npm run dev
# Open http://localhost:5173
```

### 4. Desktop Client (Tauri)
```bash
cd desktop/src-tauri
cargo tauri dev
```
=======
# HIT-CHAT
>>>>>>> 9ebd07873726e0682b3df3279d211ec3bfc3685b
