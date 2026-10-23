# Windows Defender Firewall Rule: CampusNet Inbound TCP 8080
# Run as Administrator if prompted

$ruleName = "CampusNet Local Dev Gateway"
$port = 8080

Write-Host "Configuring Windows Firewall for CampusNet on port $port..." -ForegroundColor Yellow

$existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "Firewall rule '$ruleName' already exists. Ensuring it is enabled..." -ForegroundColor Green
    Enable-NetFirewallRule -DisplayName $ruleName
} else {
    try {
        New-NetFirewallRule -DisplayName $ruleName `
                            -Direction Inbound `
                            -LocalPort $port `
                            -Protocol TCP `
                            -Action Allow `
                            -Profile Private,Domain `
                            -Description "Allows incoming LAN connections to CampusNet E2EE Gateway"
        Write-Host "Successfully added firewall rule '$ruleName' on port $port." -ForegroundColor Green
    } catch {
        Write-Host "Failed to add firewall rule. Please right-click PowerShell and 'Run as Administrator'." -ForegroundColor Red
        exit 1
    }
}
