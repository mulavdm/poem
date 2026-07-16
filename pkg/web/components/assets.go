package components

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var assetFiles embed.FS

// AssetOptions configures component asset responses.
type AssetOptions struct {
	// CacheControl is written to successful asset responses when non-empty.
	// Use no-cache during development and immutable caching only with versioned URLs.
	CacheControl string
}

// Assets returns the embedded component CSS and JavaScript filesystem.
func Assets() fs.FS {
	assets, err := fs.Sub(assetFiles, "static")
	if err != nil {
		panic(err)
	}
	return assets
}

// AssetHandler returns an HTTP handler for embedded component assets.
// It uses no cache directive by default; applications may use AssetHandlerWithOptions.
func AssetHandler() http.Handler {
	return AssetHandlerWithOptions(AssetOptions{})
}

// AssetHandlerWithOptions returns an HTTP handler for embedded assets using options.
func AssetHandlerWithOptions(options AssetOptions) http.Handler {
	files := http.FileServer(http.FS(Assets()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if options.CacheControl != "" {
			w.Header().Set("Cache-Control", options.CacheControl)
		}
		files.ServeHTTP(w, r)
	})
}
