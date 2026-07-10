//go:build windows

package render

import (
	"os"
	"path/filepath"
)

func resolveDefaultFallbackFontPaths() []string {
	root := os.Getenv("WINDIR")
	if root == "" {
		root = `C:\Windows`
	}
	candidates := []string{"malgun.ttf", "seguisym.ttf", "seguiemj.ttf", "arial.ttf"}
	paths := make([]string, 0, len(candidates))
	for _, name := range candidates {
		path := filepath.Join(root, "Fonts", name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			paths = append(paths, path)
		}
	}
	return paths
}
