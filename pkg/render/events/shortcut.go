package events

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Shortcut is a platform-neutral key chord. Key uses canonical names such as
// S, Escape, Delete, or F5.
type Shortcut struct {
	Key       string
	Modifiers Modifiers
}

func ParseShortcut(input string) (Shortcut, error) {
	parts := strings.Split(input, "+")
	var shortcut Shortcut
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return Shortcut{}, fmt.Errorf("invalid shortcut %q", input)
		}
		switch strings.ToLower(part) {
		case "ctrl", "control":
			if shortcut.Modifiers.Control {
				return Shortcut{}, fmt.Errorf("duplicate Control modifier")
			}
			shortcut.Modifiers.Control = true
		case "shift":
			if shortcut.Modifiers.Shift {
				return Shortcut{}, fmt.Errorf("duplicate Shift modifier")
			}
			shortcut.Modifiers.Shift = true
		case "alt", "option":
			if shortcut.Modifiers.Alt {
				return Shortcut{}, fmt.Errorf("duplicate Alt modifier")
			}
			shortcut.Modifiers.Alt = true
		case "meta", "cmd", "command", "win", "windows":
			if shortcut.Modifiers.Meta {
				return Shortcut{}, fmt.Errorf("duplicate Meta modifier")
			}
			shortcut.Modifiers.Meta = true
		default:
			if shortcut.Key != "" {
				return Shortcut{}, fmt.Errorf("shortcut %q has multiple keys", input)
			}
			key, err := canonicalKey(part)
			if err != nil {
				return Shortcut{}, err
			}
			shortcut.Key = key
		}
	}
	if shortcut.Key == "" {
		return Shortcut{}, fmt.Errorf("shortcut %q has no key", input)
	}
	return shortcut, nil
}

func NormalizeShortcut(input string) (string, error) {
	shortcut, err := ParseShortcut(input)
	if err != nil {
		return "", err
	}
	return shortcut.String(), nil
}

func (s Shortcut) String() string {
	parts := make([]string, 0, 5)
	if s.Modifiers.Control {
		parts = append(parts, "Ctrl")
	}
	if s.Modifiers.Alt {
		parts = append(parts, "Alt")
	}
	if s.Modifiers.Shift {
		parts = append(parts, "Shift")
	}
	if s.Modifiers.Meta {
		parts = append(parts, "Meta")
	}
	return strings.Join(append(parts, s.Key), "+")
}

func canonicalKey(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	runes := []rune(trimmed)
	if len(runes) == 1 && (unicode.IsLetter(runes[0]) || unicode.IsDigit(runes[0])) {
		return strings.ToUpper(trimmed), nil
	}
	switch strings.ToLower(strings.ReplaceAll(trimmed, " ", "")) {
	case "esc", "escape":
		return "Escape", nil
	case "enter", "return":
		return "Enter", nil
	case "space", "spacebar":
		return "Space", nil
	case "tab":
		return "Tab", nil
	case "backspace":
		return "Backspace", nil
	case "delete", "del":
		return "Delete", nil
	case "home":
		return "Home", nil
	case "end":
		return "End", nil
	case "pageup":
		return "PageUp", nil
	case "pagedown":
		return "PageDown", nil
	case "left", "arrowleft":
		return "Left", nil
	case "up", "arrowup":
		return "Up", nil
	case "right", "arrowright":
		return "Right", nil
	case "down", "arrowdown":
		return "Down", nil
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "f") {
		if number, err := strconv.Atoi(lower[1:]); err == nil && number >= 1 && number <= 24 {
			return fmt.Sprintf("F%d", number), nil
		}
	}
	return "", fmt.Errorf("unsupported shortcut key %q", input)
}

// ShortcutFromVirtualKey converts the stable Win32-compatible key values used
// by protocol v2 into the portable shortcut representation. Platform adapters
// are responsible only for transmitting the numeric key and modifier flags.
func ShortcutFromVirtualKey(key uint32, modifiers Modifiers) (Shortcut, bool) {
	name := ""
	switch {
	case key >= 'A' && key <= 'Z', key >= '0' && key <= '9':
		name = string(rune(key))
	case key >= 0x70 && key <= 0x87:
		name = fmt.Sprintf("F%d", key-0x70+1)
	default:
		name = map[uint32]string{0x1B: "Escape", 0x0D: "Enter", 0x20: "Space", 0x09: "Tab", 0x08: "Backspace", 0x2E: "Delete", 0x24: "Home", 0x23: "End", 0x21: "PageUp", 0x22: "PageDown", 0x25: "Left", 0x26: "Up", 0x27: "Right", 0x28: "Down"}[key]
	}
	return Shortcut{Key: name, Modifiers: modifiers}, name != ""
}
