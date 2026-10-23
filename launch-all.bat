@echo off
title CampusNet All-in-One Launcher
color 0B
cd /d "%~dp0"

echo ===================================================================
echo               CAMPUSNET ALL-IN-ONE SYSTEM LAUNCHER                 
echo        High-Security Ephemeral Network (Gateway + Web + CLI)       
echo ===================================================================
echo.
echo [1/3] Starting Ephemeral Gateway Server on http://localhost:8080...
start "CampusNet Server" bin\campusnet-server.exe

timeout /t 2 /nobreak >nul

echo [2/3] Opening Web Portal in default web browser...
start http://localhost:8080

echo [3/3] Launching Secure Terminal TUI Client...
start "CampusNet Terminal CLI" bin\campusnet-cli.exe

echo.
echo ===================================================================
echo [SUCCESS] CampusNet is fully running!
echo - Web Client:  http://localhost:8080
echo - Gateway API: http://localhost:8080/api/v1/health
echo - Terminal:    Active in separate window
echo ===================================================================
echo.
pause
