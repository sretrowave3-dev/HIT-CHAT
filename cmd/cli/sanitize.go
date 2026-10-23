package main

import (
	"regexp"
	"strings"
)

var ansiEscapeRegex = regexp.MustCompile(`\x1b(\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\))`)

// SanitizeTerminalString cleans incoming untrusted text to defend against terminal escape-sequence injection.
// It strips VT100/ANSI escape codes and filters out dangerous non-printable ASCII control characters.
func SanitizeTerminalString(input string) string {
	// 1. Strip ANSI escape sequences (CSI and OSC sequences)
	cleaned := ansiEscapeRegex.ReplaceAllString(input, "")

	// 2. Filter non-printable control characters (ASCII 0x00-0x1F except tab \t and newline \n)
	var sb strings.Builder
	sb.Grow(len(cleaned))

	for _, r := range cleaned {
		if r == '\n' || r == '\t' || r == '\r' {
			sb.WriteRune(r)
			continue
		}
		// Strip C0 control characters and DEL (0x7F)
		if (r >= 0 && r < 0x20) || r == 0x7F {
			continue
		}
		sb.WriteRune(r)
	}

	return sb.String()
}
