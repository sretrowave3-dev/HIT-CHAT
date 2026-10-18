# CampusNet Windows PowerShell Helper Script
param(
    [Parameter(Position=0)]
    [ValidateSet("build", "build-server", "build-cli", "test", "run-server", "run-cli", "docker-up", "docker-down", "clean")]
    [string]$Target = "build"
)

$ErrorActionPreference = "Stop"

switch ($Target) {
    "build" {
        Write-Host "==> Building CampusNet Server & CLI..." -ForegroundColor Cyan
        if (-not (Test-Path "bin")) { New-Item -ItemType Directory -Path "bin" | Out-Null }
        go build -o bin/campusnet-server.exe ./cmd/server
        go build -o bin/campusnet-cli.exe ./cmd/cli
        Write-Host "Build complete! Artifacts in bin/" -ForegroundColor Green
    }
    "build-server" {
        Write-Host "==> Building Server..." -ForegroundColor Cyan
        if (-not (Test-Path "bin")) { New-Item -ItemType Directory -Path "bin" | Out-Null }
        go build -o bin/campusnet-server.exe ./cmd/server
    }
    "build-cli" {
        Write-Host "==> Building CLI..." -ForegroundColor Cyan
        if (-not (Test-Path "bin")) { New-Item -ItemType Directory -Path "bin" | Out-Null }
        go build -o bin/campusnet-cli.exe ./cmd/cli
    }
    "test" {
        Write-Host "==> Running Test Suite..." -ForegroundColor Cyan
        go test -v ./pkg/... ./internal/...
    }
    "run-server" {
        Write-Host "==> Running CampusNet Gateway Server..." -ForegroundColor Cyan
        go run ./cmd/server
    }
    "run-cli" {
        Write-Host "==> Launching CampusNet Interactive TUI..." -ForegroundColor Cyan
        go run ./cmd/cli
    }
    "docker-up" {
        Write-Host "==> Starting Docker Compose Services..." -ForegroundColor Cyan
        docker compose up -d
    }
    "docker-down" {
        Write-Host "==> Stopping Docker Compose Services..." -ForegroundColor Cyan
        docker compose down
    }
    "clean" {
        Write-Host "==> Cleaning bin directory..." -ForegroundColor Cyan
        if (Test-Path "bin") { Remove-Item -Recurse -Force "bin" }
    }
}
