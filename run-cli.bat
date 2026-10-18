@echo off
title CampusNet Secure Ephemeral Terminal
color 0A
cd /d "%~dp0"
echo ================================================================
echo           Launching CampusNet Interactive TUI Client...         
echo ================================================================
bin\campusnet-cli.exe
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] CLI exited with code %ERRORLEVEL%
    pause
)
