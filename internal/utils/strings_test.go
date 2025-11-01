package utils

import (
	"testing"
)

func TestSanitizeForTerminal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "satellite emoji",
			input:    "User📡",
			expected: "User[SAT]",
		},
		{
			name:     "radio emoji",
			input:    "📻Station",
			expected: "[RAD]Station",
		},
		{
			name:     "fire emoji",
			input:    "Fire🔥User",
			expected: "Fire[FIRE]User",
		},
		{
			name:     "multiple emojis",
			input:    "📡🚀User🔥",
			expected: "[SAT][ROCK]User[FIRE]",
		},
		{
			name:     "regular text",
			input:    "NormalUser",
			expected: "NormalUser",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "control characters",
			input:    "User\x01\x7F",
			expected: "User",
		},
		{
			name:     "mixed content",
			input:    "📡 Satellite User 🔥",
			expected: "[SAT] Satellite User [FIRE]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForTerminal(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeForTerminal(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "Short",
			maxWidth: 10,
			expected: "Short",
		},
		{
			name:     "truncation needed",
			input:    "VeryLongUsername",
			maxWidth: 8,
			expected: "VeryL...",
		},
		{
			name:     "with emoji",
			input:    "User📡Name",
			maxWidth: 14, // "User[SAT]Name" is 14 characters
			expected: "User[SAT]Name",
		},
		{
			name:     "emoji truncation",
			input:    "User📡Name",
			maxWidth: 8,
			expected: "User[...", // "User[SAT]Name" truncated to 8 chars
		},
		{
			name:     "max width too small",
			input:    "Test",
			maxWidth: 2,
			expected: "Te",
		},
		{
			name:     "zero width",
			input:    "Test",
			maxWidth: 0,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateForDisplay(tt.input, tt.maxWidth)
			if result != tt.expected {
				t.Errorf("TruncateForDisplay(%q, %d) = %q, expected %q", tt.input, tt.maxWidth, result, tt.expected)
			}
		})
	}
}

func TestIsProblematicForTerminal(t *testing.T) {
	tests := []struct {
		name     string
		input    rune
		expected bool
	}{
		{
			name:     "regular ASCII",
			input:    'A',
			expected: false,
		},
		{
			name:     "control character",
			input:    '\x01',
			expected: true,
		},
		{
			name:     "emoji range",
			input:    0x1F600, // 😀
			expected: true,
		},
		{
			name:     "regular unicode",
			input:    'ü',
			expected: false,
		},
		{
			name:     "zero width joiner",
			input:    0x200D,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isProblematicForTerminal(tt.input)
			if result != tt.expected {
				t.Errorf("isProblematicForTerminal(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}
