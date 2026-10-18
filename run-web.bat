@echo off
title CampusNet Web Portal (Vite Dev Server)
color 0D
cd /d "%~dp0web"
echo ================================================================
echo           Starting CampusNet React Web Portal...                
echo ================================================================
npm run dev
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Web server exited with code %ERRORLEVEL%
    pause
)
