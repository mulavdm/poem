package web

import (
	_ "embed"
	"net/http"
)

//go:embed map-viewport.js
var mapViewportScript []byte

func mapViewportScriptHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(mapViewportScript)
}
