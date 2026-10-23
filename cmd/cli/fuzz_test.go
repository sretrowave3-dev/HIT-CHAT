package main

import (
	"strings"
	"testing"
)

// FuzzSanitizeTerminal tests that terminal string sanitization withstands arbitrary
// fuzz inputs, never panics, and reliably strips dangerous escape sequence indicators.
func FuzzSanitizeTerminal(f *testing.F) {
	seeds := []string{
		"Hello world",
		"\x1b[31mRed Alert\x1b[0m",
		"\x1b[2J\x1b[HCleared screen",
		"\x1b]52;c;ZXhwbG9pdA==\x07Clipboard attack",
		"\x00\x07\x08\x0b\x0c\x0e\x0fBinary noise",
		"\nValid newlines\tValid tabs\r",
		strings.Repeat("A\x1b[", 50),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		sanitized := SanitizeTerminalString(input)

		// Verification: CSI and OSC escape intros must NEVER survive in sanitized output
		if strings.Contains(sanitized, "\x1b[") {
			t.Fatalf("sanitizer failed: CSI sequence escaped filter: %q", sanitized)
		}
		if strings.Contains(sanitized, "\x1b]") {
			t.Fatalf("sanitizer failed: OSC sequence escaped filter: %q", sanitized)
		}
	})
}
