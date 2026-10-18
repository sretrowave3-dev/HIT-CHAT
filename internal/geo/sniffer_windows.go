//go:build windows

package geo

import (
	"os/exec"
	"regexp"
	"strings"
)

var (
	bssidRegex = regexp.MustCompile(`(?i)BSSID\s*:\s*([0-9a-fA-F:]{17}|[0-9a-fA-F-]{17})`)
	ssidRegex  = regexp.MustCompile(`(?i)SSID\s*:\s*(.+)`)
)

// SniffCurrentBSSID executes netsh wlan show interfaces to retrieve the active Wi-Fi AP BSSID.
func SniffCurrentBSSID() (bssid string, ssid string, err error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "BSSID") {
			matches := bssidRegex.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				bssid = NormalizeBSSID(matches[1])
			}
		} else if strings.HasPrefix(strings.ToUpper(trimmed), "SSID") && !strings.Contains(strings.ToUpper(trimmed), "BSSID") {
			matches := ssidRegex.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				ssid = strings.TrimSpace(matches[1])
			}
		}
	}

	return bssid, ssid, nil
}
