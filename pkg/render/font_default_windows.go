//go:build windows

package render

import (
	"os"
	"path/filepath"
)

func resolveDefaultFontPath() string {
	windowsDir := os.Getenv("WINDIR")
	if windowsDir == "" {
		windowsDir = `C:\Windows`
	}
	for _, name := range []string{"segoeui.ttf", "arial.ttf"} {
		candidate := filepath.Join(windowsDir, "Fonts", name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
