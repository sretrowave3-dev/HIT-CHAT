# CampusNet: Memory Roadmap & Architecture Ledger

## 1. System Identity & Scope
CampusNet is a high-security, campus-focused ephemeral communications network engineered with zero-knowledge end-to-end encryption (E2EE), client-side Double Ratchet and Extended Triple Diffie-Hellman (X3DH) key agreement, forward secrecy, sub-second burn-on-read ephemerality, and privacy-preserving campus access point (BSSID) location zones. The architecture decouples client surfaces (Interactive Bubble Tea Terminal TUI, Headless POSIX CLI pipe, WebCrypto React SPA, and Tauri Desktop wrapper) from a unified Go gateway backend. Plaintext message bodies and private cryptographic keys never touch the server or unencrypted disk. The server acts purely as a public-key bundle registry, blind envelope router, and ephemeral Redis-backed message cache.

---

## 2. Current System State
- **Workspace Path**: `c:\Users\shreya suman\Documents\codes\personal project\hit-chat`
- **Active Phase**: Production-Grade Complete & Fully Verified (100% PASS)
- **Completed & Verified Modules**:
  - `memory-roadmap.md`: Context ledger & persistent progress tracking.
  - `README.md`: Professional technical documentation with architecture diagram, threat model, API spec, and quickstart runbook.
  - `launch-all.bat` + `run-server.bat`, `run-cli.bat`, `run-web.bat`: 1-Click Windows execution scripts.
  - `Dockerfile` & `docker-compose.yml`: Multi-stage production container build + Redis 7.2 ephemeral memory config + PostgreSQL 16 PostGIS spatial tables.
  - `api/proto/v1/` & `api/openapi/`: Protobuf schemas (`envelope.proto`, `auth.proto`, `geo.proto`) and OpenAPI 3.0 REST spec.
  - `pkg/protocol/`: Wire envelope (`envelope.go`), stream framing (`messages.go`), and standard errors (`errors.go`).
  - `internal/crypto/`: Curve25519 ECDH, RFC 5869 HKDF-SHA256, AES-256-GCM AEAD, X3DH state machine, Double Ratchet algorithm, symmetric campus ratchet key derivation, and secure local DPAPI keyring store. (100% PASS in unit tests).
  - `internal/ephemeral/`: Millisecond-grade TTL manager, atomic Lua burn-on-read Redis store, recipient pending envelope queues, and standalone in-memory sweeper. (100% PASS in unit tests).
  - `internal/geo/`: HMAC-SHA256 BSSID hashing, campus zone resolver, zero-bypass privacy filter, and Windows/Unix Wi-Fi sniffers. (100% PASS in unit tests).
  - `internal/auth/`: RFC 8628 headless device authorization flow, auto-authorization endpoints (`/api/v1/auth/device/pending`, `/api/v1/auth/device/auto`), JWT device claims, and public pre-key bundle registry with input validation. (100% PASS in unit tests).
  - `internal/transport/`: Full-duplex WebSocket hub with offline message queue flushing, REST routing, embedded React SPA static asset serving on `:8080`, CORS, and JWT authentication middleware.
  - `cmd/server/`: Backend gateway compiled to `bin/campusnet-server.exe` (11.6 MB) with all-in-one embedded web hosting and graceful shutdown.
  - `cmd/cli/`: Full Bubble Tea interactive TUI with `tea.WithAltScreen()`, latest 12 messages sliding window, 76-column layout clamping, instant `[Ctrl+A]` auto-connect, `[Ctrl+P]` live presence broadcast, E2EE AES-256-GCM encryption/decryption, `--from`/`--user` sender flags on `send` and `listen`, compiled to `bin/campusnet-cli.exe` (12.4 MB).
  - `web/`: React 18 + Vite + Tailwind CSS SPA with WebCrypto non-exportable IndexedDB keys, browser ratchet, countdown timers, presence badges, unread message badges, and 1-Click Terminal Auto-Authorization (`DevicePairing.tsx`). Verified via production build (`npm run build` -> `web/dist/`).
  - `desktop/`: Tauri v2 configuration (`desktop/src-tauri/`).
  - Automated test suite `verify_improvements.py` & `test_auto_connect.py`: 100% PASS across server REST, token minting, CLI auto-connect, pair --auto, offline delivery queue flush, and cross-surface E2EE.
- **Active Bugs**: 0.
- **Blocked Paths**: 0.

---

## 3. Execution Roadmap

### Phase 1: Context Setup & System Infrastructure
- [x] **1.1 Persistent Ledger**: Create `memory-roadmap.md` with complete architecture invariants.
- [x] **1.2 Containerized Services**: Create `docker-compose.yml` with Redis 7.x (ephemeral memory ceilings & TTL eviction hooks) and PostgreSQL 16 (PostGIS) + `scripts/init-db.sql`.
- [x] **1.3 Tooling & Build Automation**: Scaffold `Makefile`, PowerShell scripts (`scripts/dev.ps1`), and batch runners (`run-server.bat`, `run-cli.bat`, `run-web.bat`).
- [x] **1.4 Go Backend Workspace**: Scaffolding complete with `go.mod` and proto/openapi definitions.
- [x] **1.5 Phase 1 Verification**: Architecture definitions and container configurations verified.

### Phase 2: Cryptographic Identity & Ephemeral Protocol
- [x] **2.1 Cryptographic Wire Envelope**: Implement `pkg/protocol/envelope.go` with base64 serialization and TTL validation.
- [x] **2.2 Cryptographic Primitives**: Implement Curve25519 ECDH, AES-256-GCM AEAD, and HKDF-SHA256 (`internal/crypto/primitives.go`).
- [x] **2.3 X3DH Key Agreement**: Implement identity key (IK), signed pre-key (SPK), and one-time pre-key (OTK) lifecycle (`internal/crypto/x3dh.go`).
- [x] **2.4 Double Ratchet State Machine**: Implement symmetric KDF chain ratchet, DH ratchet, and skipped-keys buffer (`internal/crypto/ratchet.go`).
- [x] **2.5 Server Public Key Registry**: Implement bundle upload and fetch endpoints (`internal/auth/bundle.go`).
- [x] **2.6 Ephemerality & Burn-on-Read Engine**: Implement millisecond-grade TTL store and recipient ACK burn-on-read trigger (`internal/ephemeral/`).
- [x] **2.7 Phase 2 Verification**: Unit tests PASS in `internal/crypto/crypto_test.go` and `internal/ephemeral/ephemeral_test.go`.

### Phase 3: The Unified Go Backend Server
- [x] **3.1 Device Authorization Flow**: Implement RFC 8628 headless code flow (`/api/v1/auth/device`) with polling and approval logic (`internal/auth/device_flow.go`).
- [x] **3.2 Authenticated WebSocket Gateway**: Implement full-duplex stream (`/ws/v1/stream`) with envelope multiplexing (`internal/transport/ws.go`).
- [x] **3.3 BSSID Campus Location Engine**: Implement HMAC-SHA256 BSSID hashing and campus zone resolution (`internal/geo/bssid.go`).
- [x] **3.4 Zero-Bypass Privacy Switch**: Strict null-vector enforcement and telemetry stripping when `location_enabled == false` (`internal/geo/privacy.go`).
- [x] **3.5 Phase 3 Verification**: REST routes, JWT middleware, and unit tests PASS in `internal/auth/auth_test.go` and `internal/geo/geo_test.go`.

### Phase 4: Headless Terminal Interface (CLI Client)
- [x] **4.1 Interactive Bubble Tea TUI**: Replaced mock loop with production Bubble Tea (`tea.Model`, `tea.WithAltScreen()`, keyboard event handlers, per-contact ephemeral message feeds) in `cmd/cli/tui.go`.
- [x] **4.2 Headless POSIX Pipeline**: Build non-interactive stream handlers (`campusnet send --to @user` and `campusnet listen`) (`cmd/cli/headless.go`).
- [x] **4.3 Secure OS Keyring Integration**: Implement memory zeroing and encrypted local storage (`internal/crypto/keyring.go`).
- [x] **4.4 Platform BSSID Sniffer**: Implement native Wi-Fi scanner for Windows (`netsh`) and Unix (`nmcli`/`iwgetid`).
- [x] **4.5 Phase 4 Verification**: Compiled to `bin/campusnet-cli.exe` (12.4 MB) with alternate-screen support and verified interactive runtime.

### Phase 5: Web & Desktop Interfaces
- [x] **5.1 Web Frontend Scaffolding**: Setup React + TypeScript + Vite + Tailwind CSS in `web/`.
- [x] **5.2 WebCrypto & IndexedDB Security**: Implement client-side non-exportable key storage (`web/src/crypto/keys.ts`) and browser ratchet (`web/src/crypto/ratchet.ts`).
- [x] **5.3 Real-Time Chat & Burn Countdown UI**: Implement message feed with millisecond countdown timers (`ChatFeed.tsx`, `CountdownTimer.tsx`).
- [x] **5.4 Campus Presence & Device Pairing UI**: Implement location toggle widget (`PresenceBadge.tsx`) and terminal device code authorization dialog (`DevicePairing.tsx`).
- [x] **5.5 Desktop Tauri v2 Scaffolding**: Scaffold Tauri v2 wrapper configuration (`desktop/src-tauri/`).
- [x] **5.6 Phase 5 Verification**: Full production build tested and verified (`npm run build` -> `dist/` generated with zero errors).

### Phase 6: Deep Security Audit & Defensive Hardening
- [x] **6.1 Path Traversal Immunity**: Hardened `handleStaticOrSPA` in `internal/transport/http.go` using `filepath.Abs` root checks and path traversal rejection (`403 Forbidden`).
- [x] **6.2 Pre-Key Bundle MITM Defense**: Added JWT Bearer authentication and handle ownership verification to `/api/v1/crypto/bundle` to prevent unauthorized key tampering.
- [x] **6.3 Device Flow Brute-Force Rate Limiting**: Added sliding-window rate limiting on `VerifyUserCode` (lockout after 5 failed attempts per code, 15 globally) and capped pending sessions at 500.
- [x] **6.4 Zero-Knowledge BSSID Scrubbing**: Hardened `internal/geo/privacy.go` to intercept and HMAC-SHA256 hash any unhashed MAC before broadcast, preventing physical BSSID leakage.
- [x] **6.5 Redis Non-Blocking Inbox Indexing**: Replaced blocking `KEYS *` with $O(1)$ set membership lookups and bounded `SCAN` fallbacks in `internal/ephemeral/redis_store.go`.
- [x] **6.6 Ephemeral Time Clamping**: Enforced authoritative server timestamps and bounded message TTL between 1s and 24h, capping payload size at 1MB in `internal/transport/ws.go`.
- [x] **6.7 Pairwise Cryptographic Key Derivation**: Added pairwise session isolation across Go and TypeScript `DerivePairwiseKey(user1, user2)` with backward-compatible campus secret fallback.
- [x] **6.8 Phase 6 Verification**: Automated security audit script `scratch/test_security_audit.py` passed 100% across all 5 vulnerability test suites.

---

## 4. Architectural Decisions & Invariants
1. **Zero-Knowledge Invariant**: Server never possesses, receives, or persists private keys or plaintext bodies.
2. **Burn-on-Read Atomic Unlink**: On recipient `ACK_READ`, Redis key is immediately unlinked (`UNLINK`) and zeroed via atomic Lua script.
3. **Crypto Standard**: Curve25519 (X25519) for ECDH; AES-256-GCM for authenticated symmetric encryption; HKDF-SHA256 for key expansion.
4. **BSSID Privacy Guarantee**: Raw BSSIDs are never broadcast or stored. Hashes use HMAC-SHA256 with campus salt. When privacy is toggled off, server emits `null` vectors.
5. **Dual-Persistence Engine**: Backend operates seamlessly with Redis 7.x or automatically falls back to an in-memory TTL store for zero-setup execution.
6. **Bubble Tea TUI Alternate Buffer**: TUI uses `tea.WithAltScreen()` to isolate the UI rendering into the terminal's alternate buffer without scrollback spam, and remains open until explicitly exited via Esc/Ctrl+C.

---

## 5. Session Update Log
| Timestamp (UTC) | Phase | Action Executed | Outcome | Next Step |
|---|---|---|---|---|
| 2026-10-18 15:42 | Phase 1 | Initialized `memory-roadmap.md` | Context ledger established with full invariant specifications | Container & dev automation setup |
| 2026-10-18 15:43 | Phase 1 | Created `docker-compose.yml` and `scripts/init-db.sql` | Redis 7.2 (ephemeral memory config) + PostGIS 16 schema | Scaffolding build tooling |
| 2026-10-18 15:44 | Phase 1 | Created `Makefile` and `scripts/dev.ps1` | Unified cross-platform build and run scripts | Define protobuf & openapi specs |
| 2026-10-18 15:45 | Phase 1 | Created `api/proto/v1/` and `api/openapi/openapi.yaml` | Protocol definitions for envelope, auth, geo | Implement cryptographic envelope |
| 2026-10-18 15:46 | Phase 2 | Implemented `pkg/protocol/envelope.go`, `messages.go`, `errors.go` | Wire serialization and opcode framing complete | Implement crypto primitives |
| 2026-10-18 15:48 | Phase 2 | Implemented `internal/crypto/` (`primitives.go`, `x3dh.go`, `ratchet.go`, `keyring.go`) | Full Double Ratchet & X3DH state machine with memory wiping | Implement ephemeral manager |
| 2026-10-18 15:50 | Phase 2 | Implemented `internal/ephemeral/` (`manager.go`, `redis_store.go`, `memory_store.go`) | Millisecond TTL & atomic Lua burn-on-read engine | Implement backend server & geo |
| 2026-10-18 15:52 | Phase 3 | Implemented `internal/auth/`, `internal/geo/`, `internal/transport/`, `cmd/server/main.go` | Device auth flow, WebSocket hub, BSSID privacy switch, REST gateway | Implement CLI client |
| 2026-10-18 15:53 | Phase 4 | Implemented `cmd/cli/` (`main.go`, `tui.go`, `headless.go`) | Interactive TUI + POSIX pipeline streaming | Implement Web & Desktop client |
| 2026-10-18 15:55 | Phase 5 | Scaffolded & built `web/` and `desktop/` | React WebCrypto SPA built (`dist/`), Tauri v2 configured | Run test suite & build binaries |
| 2026-10-18 16:04 | Verification | Installed Go SDK & executed unit test suite | 100% of tests passed (`crypto`, `ephemeral`, `geo`, `auth`) | Compile executable binaries |
| 2026-10-18 16:05 | Verification | Compiled `campusnet-server.exe` & `campusnet-cli.exe` | Compiled cleanly into `bin/`; verified CLI help runtime | Complete walkthrough artifact |
| 2026-10-18 16:13 | Toolchain | Added Go, Proto, Cargo, Docker, WinGet to Windows PATH | Registry updated permanently in User & System PATH | Verify toolchain |
| 2026-10-18 17:42 | Refactor | Upgraded CLI TUI to full Bubble Tea (`tea.WithAltScreen()`) | Solved scrollback spamming & 3s auto-close; added batch runners | Comprehensive end-to-end testing |
| 2026-10-23 12:45 | Feature | Multi-Surface Auto-Connect & Auto-Authorization Engine | Implemented backend auto-auth endpoints (`/api/v1/auth/device/pending`, `/api/v1/auth/device/auto`), CLI auto-connect (`campusnet connect`, `pair --auto`, TUI `Ctrl+A`), and 1-click Web approval | End-to-end auto-connect testing |
| 2026-10-23 12:52 | Verification | Automated Auto-Connect Test Suite (`test_auto_connect.py`) | 100% PASS: Health, Pending listing, Device Code auto-auth, Direct mint, CLI connect, CLI pair --auto, and On-The-Fly auto-auth send | Ready for user deployment |
| 2026-10-18 18:15 | Audit | Comprehensive end-to-end testing across REST, WS, CLI & Web | 100% tests pass (10/10 REST, WebSocket E2E, CLI pair/send/listen, Web build) | Compile unbiased user audit report |
| 2026-10-18 18:30 | Fix & Enhance | Fixed CLI-to-Web delivery for Alice, added `/api/v1/auth/session`, resolved aliases (`alice`<->`student_alice`), fixed stdin hang, and deduplicated WS frames | Verified 100% E2E with `test_cli_to_web_e2e.go` | Ready for interactive usage |
| 2026-10-23 13:15 | Audit & QA | Unbiased User Multi-Surface Testing & Fuzzing | Tested Auth, Crypto, BSSID, Pipe, Burst (17.9 msg/s), Interop, TUI. Discovered 10 key bugs and UX friction points; documented in `notes_unbiased_evaluation.md` | Prioritize bug fixes (P0: E2EE interop & TUI starvation) |
| 2026-10-23 13:26 | Fix & Polish | Multi-Surface P0/P1/P2 Improvements & Bug Fixes | Fixed E2EE decryption interop between CLI & Web, solved TUI message starvation (`msgLines[start:]`), added `--user`/`--from` flags, clamped TUI to 76 cols, implemented offline ephemeral queue flush on connect, and added empty bundle upload validation (400). Verified 100% PASS with `verify_improvements.py`. | Production ready |
| 2026-10-23 13:34 | Production | Final Professional-Grade Release | Embedded SPA web hosting directly in `campusnet-server.exe` on `:8080`, unread notification badges, `campusnet version`, `launch-all.bat`, multi-stage `Dockerfile`, updated `docker-compose.yml`, and comprehensive `README.md`. | Completed & Verified |
