package events

import "testing"

func TestNormalizeShortcut(t *testing.T) {
	for input, want := range map[string]string{
		"shift+ctrl+s": "Ctrl+Shift+S",
		"Option+F5":    "Alt+F5",
		"control+del":  "Ctrl+Delete",
	} {
		got, err := NormalizeShortcut(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeShortcut(%q)=%q,%v want %q", input, got, err, want)
		}
	}
}

func TestParseShortcutRejectsAmbiguousChords(t *testing.T) {
	for _, input := range []string{"Ctrl", "Ctrl+S+P", "Ctrl+Ctrl+S", "Ctrl+?"} {
		if _, err := ParseShortcut(input); err == nil {
			t.Fatalf("invalid shortcut %q accepted", input)
		}
	}
}

func TestShortcutFromVirtualKey(t *testing.T) {
	shortcut, ok := ShortcutFromVirtualKey('K', Modifiers{Control: true, Alt: true})
	if !ok || shortcut.String() != "Ctrl+Alt+K" {
		t.Fatalf("shortcut=%+v ok=%v", shortcut, ok)
	}
}
