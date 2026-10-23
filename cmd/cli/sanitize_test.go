package main

import (
	"testing"
)

func TestSanitizeTerminalString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world, study session at 3pm!",
			expected: "Hello world, study session at 3pm!",
		},
		{
			name:     "ANSI CSI color and style codes",
			input:    "\x1b[31;1mDangerous Red\x1b[0m Text",
			expected: "Dangerous Red Text",
		},
		{
			name:     "Screen clearing injection",
			input:    "\x1b[2J\x1b[HClear and reset terminal",
			expected: "Clear and reset terminal",
		},
		{
			name:     "OSC clipboard / window title injection",
			input:    "\x1b]52;c;ZXhwbG9pdA==\x07Pwned Title",
			expected: "Pwned Title",
		},
		{
			name:     "Non-printable control characters",
			input:    "Warning\a\b\x00\x03\x04Beep",
			expected: "WarningBeep",
		},
		{
			name:     "Preserves valid newlines and tabs",
			input:    "Line 1\n\tIndented Line 2",
			expected: "Line 1\n\tIndented Line 2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeTerminalString(tc.input)
			if got != tc.expected {
				t.Fatalf("SanitizeTerminalString mismatch:\n got:  %q\n want: %q", got, tc.expected)
			}
		})
	}
}
