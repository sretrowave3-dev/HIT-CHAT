<#
.SYNOPSIS
  CampusNet End-to-End Bidirectional E2EE Integration Test Runner
.DESCRIPTION
  Runs the standalone E2E integration test suite covering:
  - WebSocket hub connection
  - Authenticated device tokens
  - CLI-to-Web HKDF/AES-GCM encryption & decryption
  - Ephemeral store persistence & authoritative TTL
  - Delivery lifecycle ACKs (DELIVERED & READ)
  - Authorized IDOR-guarded Burn-on-Read
  - Web-to-CLI reply encryption & decryption
#>

$ErrorActionPreference = "Stop"

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host " [CampusNet] Running End-to-End E2EE Integration Suite      " -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

go run ./cmd/e2e
if ($LASTEXITCODE -ne 0) {
    Write-Host "`n[ERROR] E2E Integration Suite Failed (exit code: $LASTEXITCODE)" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "`n[PASS] E2E Integration Suite Passed Successfully!" -ForegroundColor Green
