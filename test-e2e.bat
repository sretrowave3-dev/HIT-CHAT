@echo off
REM CampusNet E2E Bidirectional E2EE Integration Test Runner
echo ============================================================
echo  [CampusNet] Running End-to-End E2EE Integration Suite
echo ============================================================

go run ./cmd/e2e
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] E2E Integration Suite Failed with code %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)

echo.
echo [PASS] E2E Integration Suite Passed Successfully!
