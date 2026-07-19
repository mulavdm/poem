package web

import (
	_ "embed"
	"net/http"
)

//go:embed image_viewport.js
var imageViewportScript []byte

func imageViewportScriptHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(imageViewportScript)
}
