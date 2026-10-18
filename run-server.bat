@echo off
title CampusNet Ephemeral Gateway Server
color 0B
cd /d "%~dp0"
echo ================================================================
echo           Starting CampusNet Ephemeral Gateway Server...        
echo ================================================================
bin\campusnet-server.exe
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Server exited with code %ERRORLEVEL%
    pause
)
