package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// SanitizeForTerminal sanitizes a string for terminal display by:
// 1. Replacing problematic Unicode characters (especially emojis) with safe alternatives
// 2. Removing control characters
// 3. Ensuring the string is safe for display in Windows terminals
func SanitizeForTerminal(s string) string {
	if s == "" {
		return s
	}

	// Replace common problematic emoji strings with text equivalents
	// Note: Using string replacements instead of rune map for multi-byte emoji
	emojis := map[string]string{
		"📡": "[SAT]",  // Satellite dish emoji
		"📻": "[RAD]",  // Radio emoji
		"🔥": "[FIRE]", // Fire emoji
		"⚡": "[BOLT]", // Lightning bolt
		"🚀": "[ROCK]", // Rocket
		"🌐": "[GLOB]", // Globe
		"📶": "[SIG]",  // Signal strength
		"🔋": "[BAT]",  // Battery
		"💻": "[COMP]", // Computer
		"📱": "[MOB]",  // Mobile phone
		"🎯": "[TARG]", // Target
		"🔗": "[LINK]", // Link
		"⭐": "[STAR]", // Star
		"🏠": "[HOME]", // House
		"🚗": "[CAR]",  // Car
		"✈️": "[PLAN]", // Airplane
		"🛰️": "[SAT2]", // Satellite
		"🔌": "[PLUG]", // Electric plug
		"🌍": "[EARTH]", // Earth globe
	}

	// First pass: replace known problematic emoji strings
	result := s
	for emoji, replacement := range emojis {
		result = strings.ReplaceAll(result, emoji, replacement)
	}

	// Second pass: filter other problematic characters
	var finalResult strings.Builder
	for _, r := range result {
		if isProblematicForTerminal(r) {
			// Skip problematic characters (don't include them at all)
			continue
		} else {
			finalResult.WriteRune(r)
		}
	}

	sanitized := finalResult.String()

	// Remove any remaining control characters and non-printable characters
	sanitized = regexp.MustCompile(`[\x00-\x1F\x7F-\x9F]`).ReplaceAllString(sanitized, "")

	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)

	return sanitized
}

// isProblematicForTerminal checks if a rune might cause display issues in terminals
func isProblematicForTerminal(r rune) bool {
	// Control characters
	if r < 32 || (r >= 127 && r < 160) {
		return true
	}

	// High Unicode ranges that often contain emojis and problematic characters
	// Emoji blocks in Unicode
	if (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
		(r >= 0x1F300 && r <= 0x1F5FF) || // Miscellaneous Symbols and Pictographs
		(r >= 0x1F680 && r <= 0x1F6FF) || // Transport and Map Symbols
		(r >= 0x1F700 && r <= 0x1F77F) || // Alchemical Symbols
		(r >= 0x1F780 && r <= 0x1F7FF) || // Geometric Shapes Extended
		(r >= 0x1F800 && r <= 0x1F8FF) || // Supplemental Arrows-C
		(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols and Pictographs
		(r >= 0x1FA00 && r <= 0x1FA6F) || // Chess Symbols
		(r >= 0x1FA70 && r <= 0x1FAFF) || // Symbols and Pictographs Extended-A
		(r >= 0x2600 && r <= 0x26FF) ||   // Miscellaneous Symbols
		(r >= 0x2700 && r <= 0x27BF) ||   // Dingbats
		(r >= 0xFE00 && r <= 0xFE0F) ||   // Variation Selectors
		(r >= 0x200D && r <= 0x200D) {    // Zero Width Joiner
		return true
	}

	// Check for combining marks that might cause display issues
	if unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me) {
		return true
	}

	return false
}

// TruncateForDisplay truncates a string to fit within the specified width,
// adding "..." if truncated, and ensuring proper display in terminals
func TruncateForDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	
	s = SanitizeForTerminal(s)
	
	// Use rune count instead of byte length for proper Unicode handling
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	
	return string(runes[:maxWidth-3]) + "..."
}
