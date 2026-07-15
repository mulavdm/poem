//go:build android

package render

import "os"

// resolveDefaultFontPath returns the Android system UI font. Roboto has
// shipped at this path on every Android release the engine targets (API 26+);
// the existence check guards exotic builds, falling back to the atlas
// builder's embedded basicfont path when empty.
func resolveDefaultFontPath() string {
	const roboto = "/system/fonts/Roboto-Regular.ttf"
	if _, err := os.Stat(roboto); err == nil {
		return roboto
	}
	return ""
}
