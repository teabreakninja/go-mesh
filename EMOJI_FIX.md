# Emoji Sanitization Fix for Windows Terminal

## Problem

The TUI (Terminal User Interface) was experiencing display corruption on Windows terminals when usernames containing emoji characters (like 📡) were displayed. This was particularly problematic with PowerShell and Console Host, where Unicode emoji characters can cause layout issues, text wrapping problems, and terminal display corruption.

## Root Cause

The issue occurred because:
1. Emoji characters are multi-byte Unicode characters that can have inconsistent width handling across different terminals
2. Some Windows terminals don't properly handle wide characters or combining marks
3. The display table calculations didn't account for the varying visual width of emoji characters
4. Emoji can cause cursor positioning issues and text overflow

## Solution

We implemented a comprehensive string sanitization system consisting of:

### 1. String Sanitization Utility (`internal/utils/strings.go`)

- **`SanitizeForTerminal(s string) string`**: Replaces problematic Unicode characters (especially emojis) with safe text alternatives
- **`TruncateForDisplay(s string, maxWidth int) string`**: Safely truncates strings while ensuring proper terminal display
- **`isProblematicForTerminal(r rune) bool`**: Identifies Unicode characters that might cause display issues

### 2. Emoji Replacement Map

Common problematic emojis are replaced with safe bracketed text:
- 📡 → `[SAT]` (Satellite)  
- 📻 → `[RAD]` (Radio)
- 🔥 → `[FIRE]` (Fire)
- ⚡ → `[BOLT]` (Lightning)
- 🚀 → `[ROCK]` (Rocket)
- And many more...

### 3. Integration Points

Updated the following components to use sanitization:

#### NodeDB (`internal/meshtastic/nodedb.go`)
- `GetNodeName()` - Sanitizes long and short names before returning
- `GetNodeShortName()` - Sanitizes and properly truncates names

#### UI Model (`internal/ui/model.go`)
- `updatePacketTable()` - Sanitizes NodeInfo display data
- Column display names - Uses `TruncateForDisplay()` for proper width handling

## Features

### Safe Character Replacement
- Replaces known problematic emoji with readable text alternatives
- Removes or replaces other problematic Unicode characters (control chars, combining marks, etc.)
- Preserves regular Unicode characters that display properly

### Proper Text Truncation
- Uses rune count instead of byte length for accurate Unicode handling  
- Maintains visual consistency in table columns
- Adds "..." indicator when text is truncated

### Terminal Compatibility
- Tested to work with PowerShell, Command Prompt, and modern terminals
- Handles both legacy and modern terminal capabilities
- Prevents display corruption and layout issues

## Testing

Comprehensive unit tests verify:
- Emoji replacement accuracy
- Control character filtering
- Proper truncation behavior  
- Edge cases (empty strings, very long text, etc.)

Run tests with:
```bash
go test ./internal/utils/
```

## Usage

The sanitization is automatically applied whenever usernames are displayed in the TUI. No user configuration is required.

### Before Fix:
```
From     | To       | Data
---------|----------|------------------
User📡   | Station🔥| NodeInfo...
```
*(Could cause display corruption)*

### After Fix:
```
From       | To         | Data
-----------|------------|------------------
User[SAT]  | Stn[FIRE] | NodeInfo...
```
*(Clean, consistent display)*

## Future Enhancements

- Add configuration option to customize emoji replacement text
- Support for additional problematic Unicode ranges
- Terminal capability detection for adaptive behavior
- User preference for emoji handling (strip vs replace vs pass-through)
