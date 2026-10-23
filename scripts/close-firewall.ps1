# Windows Defender Firewall Rule Cleanup: CampusNet Inbound TCP 8080
# Run as Administrator if prompted

$ruleName = "CampusNet Local Dev Gateway"

Write-Host "Removing Windows Firewall rule for CampusNet..." -ForegroundColor Yellow

$existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
if ($existing) {
    try {
        Remove-NetFirewallRule -DisplayName $ruleName
        Write-Host "Successfully removed firewall rule '$ruleName'." -ForegroundColor Green
    } catch {
        Write-Host "Failed to remove firewall rule. Please right-click PowerShell and 'Run as Administrator'." -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "Firewall rule '$ruleName' was not found (already clean)." -ForegroundColor Gray
}
