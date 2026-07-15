//go:build !windows && !android

package render

func resolveDefaultFallbackFontPaths() []string { return nil }
