//go:build android

package render

import "os"

// resolveDefaultFallbackFontPaths returns Android system fonts that widen
// glyph coverage beyond the primary face, mirroring the Windows fallback
// chain's intent (CJK + symbols). Only fonts actually present are returned.
func resolveDefaultFallbackFontPaths() []string {
	candidates := []string{
		"/system/fonts/NotoSansCJK-Regular.ttc",
		"/system/fonts/NotoSansSymbols-Regular-Subsetted.ttf",
		"/system/fonts/DroidSansFallback.ttf",
	}
	var present []string
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			present = append(present, path)
		}
	}
	return present
}
