# CampusNet: Production Dev-Auth Removal Checklist

This document details the exact, verifiable procedure to remove the Development Identity Switcher before compiling, containerizing, or releasing CampusNet into production.

---

## 1. Architectural Guardrails (Already in Place)

CampusNet was built with defense-in-depth isolation for dev features:
1. **Go Build Tags (`//go:build devauth`)**:
   The `POST /api/v1/dev/login` endpoint is compiled **only** when `-tags devauth` is explicitly provided to `go build` or `go run`. Standard production builds compile `internal/transport/dev_auth_stub.go` instead, where the endpoint does not exist and returns `404 Not Found`.
2. **Flag Enforcement (`--dev-auth`)**:
   Even if compiled with the tag, the server refuses any dev login unless the `--dev-auth` CLI flag is explicitly passed at runtime.
3. **Private Network Restriction**:
   The dev endpoint checks `isPrivateNetwork(req)` and immediately drops connections from non-private or public IP ranges with `403 Forbidden`.
4. **Vite Environment Guards**:
   The frontend `DevIdentityPicker` renders only if `import.meta.env.VITE_DEV_AUTH === "true"` or during local Vite dev mode (`import.meta.env.DEV`).

---

## 2. Pre-Release Removal Checklist

Follow these steps for a completely clean production release:

### Step 1: Remove Server Dev Auth Source File
Delete the build-tagged devauth implementation:
```bash
rm internal/transport/dev_auth.go
```
Verify that `internal/transport/dev_auth_stub.go` is retained so that `r.registerDevAuthRoutes()` compiles cleanly as a no-op:
```bash
git status internal/transport/
```

### Step 2: Remove Web Dev Picker Component
Delete the dev identity switcher component:
```bash
rm web/src/components/DevIdentityPicker.tsx
```
In `web/src/App.tsx`:
- Remove `import { DevIdentityPicker } from './components/DevIdentityPicker.tsx';`
- Remove `<DevIdentityPicker ... />` from the JSX tree.
- Remove `handleSwitchIdentity` callback.

### Step 3: Ensure Clean Environment Variables
Verify your production `.env` and deployment configuration:
- `VITE_DEV_AUTH` must NOT be set to `"true"` (omit it or set to `"false"`).
- `CAMPUSNET_ENV=production`
- `CAMPUSNET_AS` must NOT be set in developer shells.

### Step 4: Verification Grep
Run the following search commands across the entire codebase to confirm zero residual dev-auth tokens:
```bash
# Check Go codebase
git grep -i "devauth"
git grep -i "dev_auth"

# Check Web codebase
git grep -i "VITE_DEV_AUTH"
git grep -i "DevIdentityPicker"
```
Both commands should return zero matches in your production branch.

### Step 5: Run Full Production Test Suite
Verify that all tests and production builds pass:
```bash
# 1. Compile without devauth tag (Release mode)
go test ./... -race
go build -o bin/campusnet-server ./cmd/server
go build -o bin/campusnet-cli ./cmd/cli

# 2. Build web bundle
cd web
npm run test
npm run build
cd ..
```

---

## 3. Threat Verification

Attempting to access `POST /api/v1/dev/login` in the release binary:
```bash
curl -X POST http://localhost:8080/api/v1/dev/login -d '{"handle":"alice"}'
# Response must be: 404 Not Found
```
Attempting to pass `--as` on CLI without user credentials:
```bash
# Local keyring for that handle will not have a valid signed token, failing cleanly.
```
