//go:build !windows && !android

package render

func resolveDefaultFontPath() string { return "" }
