package web

import (
	_ "embed"
	"net/http"
)

//go:embed workspace.js
var workspaceScript []byte

func workspaceScriptHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(workspaceScript)
}
