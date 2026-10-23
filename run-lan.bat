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
echo 2. Compiling server binary with devauth tag into bin\campusnet-server.exe...
go build -tags devauth -o bin\campusnet-server.exe .\cmd\server
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Server build failed. Aborting.
    pause
    exit /b 1
)

echo.
echo 3. Launching bin\campusnet-server.exe with --lan and --dev-auth...
bin\campusnet-server.exe --lan --dev-auth
pause
