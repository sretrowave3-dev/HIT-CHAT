@echo off
setlocal
echo =================================================================
echo   CampusNet LAN Gateway with Local HTTPS & Dev Identity Switcher
echo =================================================================
echo.
echo 1. Ensuring production frontend assets are built...
cd web
call npm.cmd run build
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Web build failed. Aborting.
    pause
    exit /b 1
)
cd ..

echo.
echo 2. Launching CampusNet Gateway with --lan and --dev-auth...
echo    (Local CA and Server TLS cert will be generated under .campusnet-tls\)
echo.
go run -tags devauth ./cmd/server --lan --dev-auth
pause
