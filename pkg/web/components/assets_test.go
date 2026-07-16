package components

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetsAndHandler(t *testing.T) {
	for _, name := range []string{"components.css", "components.js"} {
		if _, err := fs.Stat(Assets(), name); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	server := httptest.NewServer(AssetHandlerWithOptions(AssetOptions{CacheControl: "no-cache"}))
	defer server.Close()
	resp, err := http.Get(server.URL + "/components.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/css") || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("unexpected response: %s", resp.Status)
	}
}
