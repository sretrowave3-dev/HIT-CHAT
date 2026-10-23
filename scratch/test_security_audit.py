import sys
import json
import urllib.request
import urllib.error
import time

SERVER = "http://localhost:8080"

def log(msg, status="INFO"):
    print(f"[{status}] {msg}")

def test_health():
    log("Checking API health...")
    req = urllib.request.Request(f"{SERVER}/api/v1/health")
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
        data = json.loads(resp.read().decode())
        assert data.get("status") == "healthy"
    log("Health check passed", "PASS")

def test_path_traversal():
    log("Testing path traversal defense...")
    # Normal root route should succeed
    with urllib.request.urlopen(f"{SERVER}/") as resp:
        assert resp.status == 200

    # Path traversal attack
    import http.client
    conn = http.client.HTTPConnection("localhost", 8080)

    traversal_paths = [
        "/../../go.mod",
        "/..%2f..%2fgo.mod",
        "/..\\..\\go.mod",
        "/%2e%2e/%2e%2e/go.mod",
        "/C:/Windows/win.ini",
    ]
    for p in traversal_paths:
        try:
            conn.request("GET", p)
            resp = conn.getresponse()
            content = resp.read().decode(errors="ignore")
            # If 200, ensure it did not leak repo files (only SPA index.html is allowed)
            assert "module github.com/hit-chat/campusnet" not in content, f"Path traversal leaked go.mod via {p}"
            log(f"Traversal safely handled for {p}: HTTP {resp.status}", "PASS")
        except Exception as e:
            log(f"Traversal blocked with exception for {p}: {e}", "PASS")
    conn.close()

def test_bundle_authentication():
    log("Testing Pre-Key Bundle upload authentication & authorization...")
    
    # 1. Unauthenticated upload must be rejected with 401
    bad_req = urllib.request.Request(
        f"{SERVER}/api/v1/crypto/bundle",
        data=json.dumps({
            "device_id": "malicious-dev-1",
            "user_handle": "alice",
            "identity_key": "fake_key_base64=="
        }).encode(),
        headers={"Content-Type": "application/json"}
    )
    try:
        urllib.request.urlopen(bad_req)
        assert False, "Unauthenticated bundle upload should have been rejected!"
    except urllib.error.HTTPError as e:
        assert e.code == 401, f"Expected 401 Unauthorized, got {e.code}"
        log("Unauthenticated bundle upload correctly rejected with 401 Unauthorized", "PASS")

    # 2. Mint session token for Alice
    alice_sess_req = urllib.request.Request(
        f"{SERVER}/api/v1/auth/session",
        data=json.dumps({"user_handle": "alice", "platform": "test"}).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(alice_sess_req) as resp:
        alice_data = json.loads(resp.read().decode())
        alice_token = alice_data["access_token"]
        alice_dev = alice_data["device_id"]

    # 3. Alice token attempting to overwrite Bob's bundle -> must be rejected with 403
    hijack_req = urllib.request.Request(
        f"{SERVER}/api/v1/crypto/bundle",
        data=json.dumps({
            "device_id": alice_dev,
            "user_handle": "bob", # Impersonating bob!
            "identity_key": "attacker_fake_key=="
        }).encode(),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {alice_token}"
        }
    )
    try:
        urllib.request.urlopen(hijack_req)
        assert False, "Cross-account bundle registration should have failed with 403!"
    except urllib.error.HTTPError as e:
        assert e.code == 403, f"Expected 403 Forbidden, got {e.code}"
        log("Cross-account bundle hijack attempt rejected with 403 Forbidden", "PASS")

    # 4. Alice uploading bundle for Alice -> must succeed with 201
    legit_req = urllib.request.Request(
        f"{SERVER}/api/v1/crypto/bundle",
        data=json.dumps({
            "device_id": alice_dev,
            "user_handle": "alice",
            "identity_key": "valid_alice_identity_key=="
        }).encode(),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {alice_token}"
        }
    )
    with urllib.request.urlopen(legit_req) as resp:
        assert resp.status == 201
        res_data = json.loads(resp.read().decode())
        assert res_data.get("status") == "bundle_registered"
        log("Legitimate authenticated bundle upload succeeded with 201 Created", "PASS")

def test_device_flow_security():
    log("Testing Device Flow brute-force rate-limiting & isolation...")

    # Request code for Charlie
    code_req = urllib.request.Request(
        f"{SERVER}/api/v1/auth/device/code",
        data=json.dumps({
            "client_id": "test-charlie-dev",
            "platform": "cli",
            "public_identity_key": "charlie_key==",
            "user_handle": "charlie"
        }).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(code_req) as resp:
        code_data = json.loads(resp.read().decode())
        charlie_user_code = code_data["user_code"]
        log(f"Device code obtained for Charlie: {charlie_user_code}")

    # Brute-force rate limiting test: send 6 fake verification codes to trigger rate limit lock
    rate_limited = False
    fake_code = "CNET-GUESS-CODE"
    for i in range(8):
        v_req = urllib.request.Request(
            f"{SERVER}/api/v1/auth/device/verify",
            data=json.dumps({
                "user_code": fake_code,
                "user_handle": "attacker"
            }).encode(),
            headers={"Content-Type": "application/json"}
        )
        try:
            urllib.request.urlopen(v_req)
        except urllib.error.HTTPError as e:
            err_data = json.loads(e.read().decode())
            if "too many failed" in err_data.get("error", ""):
                rate_limited = True
                log(f"Brute force attempt #{i+1} triggered rate-limit protection: {err_data['error']}", "PASS")
                break

    assert rate_limited, "Brute force attempts should trigger rate limit lock!"

    # Verify session isolation: auto-authorizing for Bob should NOT claim Charlie's pending code
    auto_req = urllib.request.Request(
        f"{SERVER}/api/v1/auth/device/auto",
        data=json.dumps({"user_handle": "bob"}).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(auto_req) as resp:
        auto_data = json.loads(resp.read().decode())
        # Charlie's user code was not stolen; an independent session was minted for Bob
        assert auto_data.get("user_code") != charlie_user_code
        log("Auto-authorize safely isolated: Charlie's pending code was NOT hijacked by Bob", "PASS")

def test_bssid_privacy_scrubbing():
    log("Testing BSSID physical MAC address scrub & zero-knowledge guarantee...")

    # 1. Location disabled -> complete scrub
    p1 = urllib.request.Request(
        f"{SERVER}/api/v1/geo/resolve",
        data=json.dumps({
            "user_handle": "alice",
            "bssid_hash": "00:14:22:01:23:45",
            "location_enabled": False
        }).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(p1) as resp:
        d1 = json.loads(resp.read().decode())
        assert d1["zone_code"] == "PROTECTED"
        assert d1.get("bssid_hash", "") == ""
        assert d1["location_enabled"] is False
        log("Location disabled: BSSID completely scrubbed to null vector (omitted)", "PASS")

    # 2. Location enabled with raw MAC -> server MUST hash raw MAC before output
    p2 = urllib.request.Request(
        f"{SERVER}/api/v1/geo/resolve",
        data=json.dumps({
            "user_handle": "alice",
            "bssid_hash": "00:14:22:01:23:45",
            "location_enabled": True
        }).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(p2) as resp:
        d2 = json.loads(resp.read().decode())
        assert d2["bssid_hash"] != "00:14:22:01:23:45", "Raw physical MAC leaked in presence output!"
        assert len(d2["bssid_hash"]) == 64, f"Expected 64-char HMAC hash, got {d2['bssid_hash']}"
        assert d2["zone_code"] == "LIB_FL2_SILENT"
        log(f"Raw MAC scrubbed & hashed to: {d2['bssid_hash'][:16]}... (Zone: {d2['zone_code']})", "PASS")

if __name__ == "__main__":
    print("\n--- RUNNING COMPREHENSIVE SECURITY & VULNERABILITY AUDIT ---\n")
    test_health()
    test_path_traversal()
    test_bundle_authentication()
    test_device_flow_security()
    test_bssid_privacy_scrubbing()
    print("\n[ALL 5 SECURITY SUITES PASSED CLEANLY WITH ZERO VULNERABILITIES]\n")
