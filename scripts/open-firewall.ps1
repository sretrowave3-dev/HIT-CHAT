# Windows Defender Firewall Rule: CampusNet Inbound TCP 8080, 8081
# Run as Administrator if prompted

$ruleName = "CampusNet Local Dev Gateway"
$ports = @(8080, 8081)

Write-Host "Configuring Windows Firewall for CampusNet on ports 8080 and 8081..." -ForegroundColor Yellow

$existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "Firewall rule '$ruleName' already exists. Ensuring it is enabled and unrestricted..." -ForegroundColor Green
    Set-NetFirewallRule -DisplayName $ruleName -Profile Any -Action Allow -Enabled True
} else {
    try {
        New-NetFirewallRule -DisplayName $ruleName `
                            -Direction Inbound `
                            -LocalPort 8080, 8081 `
                            -Protocol TCP `
                            -Action Allow `
                            -Profile Any `
                            -Description "Allows incoming LAN connections to CampusNet E2EE Gateway and HTTP Helper"
        Write-Host "Successfully added firewall rule '$ruleName' on ports 8080 and 8081." -ForegroundColor Green
    } catch {
        Write-Host "Failed to add firewall rule. Please right-click PowerShell and 'Run as Administrator'." -ForegroundColor Red
        exit 1
    }
}
