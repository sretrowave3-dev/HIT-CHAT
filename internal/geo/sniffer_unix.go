//go:build !windows

package geo

import (
	"os/exec"
	"regexp"
	"strings"
)

var (
	unixBSSIDRegex = regexp.MustCompile(`([0-9a-fA-F:]{17})`)
)

// SniffCurrentBSSID retrieves active Wi-Fi AP BSSID on Linux and macOS.
func SniffCurrentBSSID() (bssid string, ssid string, err error) {
	// Try nmcli first (Linux)
	cmd := exec.Command("nmcli", "-t", "-f", "BSSID,SSID", "dev", "wifi")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, ":") {
				parts := strings.Split(line, ":")
				if len(parts) >= 6 {
					return NormalizeBSSID(strings.Join(parts[:6], ":")), "", nil
				}
			}
		}
	}

	// Try iwgetid (Linux)
	cmd = exec.Command("iwgetid", "-a", "-r")
	output, err = cmd.Output()
	if err == nil {
		matches := unixBSSIDRegex.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			return NormalizeBSSID(matches[1]), "", nil
		}
	}

	return "", "", nil
}
