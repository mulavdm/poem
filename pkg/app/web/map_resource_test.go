package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/app"
)

func TestVectorMapAssetsAndLoadingMarkup(t *testing.T) {
	application := app.App[int]{
		View: func(int) app.Node {
			return app.MapViewportNode{
				Semantic: app.Semantic{ID: "map", Name: "Map", Enabled: true},
				Source:   app.MapSource{ID: "streets", ProviderID: "engine", MinZoom: 0, MaxZoom: 18},
				Camera:   app.MapCamera{Latitude: 52, Longitude: 4, Zoom: 10}, MaxZoom: 18, MaxPitch: 70,
			}
		},
		Update: func(state int, _ app.Msg) (int, app.Cmd) { return state, app.Cmd{} },
	}
	handler, _, err := NewHandler(application, "Map", Options{SigningKey: []byte("01234567890123456789012345678901")})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/__poem/map-worker.js", contentType: "text/javascript; charset=utf-8", contains: "poemCartographyBuild"},
		{path: "/__poem/wasm_exec.js", contentType: "text/javascript; charset=utf-8", contains: "globalThis.Go = class"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != test.contentType || !strings.Contains(response.Body.String(), test.contains) {
			t.Fatalf("asset %s: status=%d type=%q", test.path, response.Code, response.Header().Get("Content-Type"))
		}
		if response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Fatalf("asset %s cache=%q", test.path, response.Header().Get("Cache-Control"))
		}
	}
	wasm := httptest.NewRecorder()
	handler.ServeHTTP(wasm, httptest.NewRequest(http.MethodGet, "/__poem/cartography.wasm", nil))
	if wasm.Code != http.StatusOK || wasm.Header().Get("Content-Type") != "application/wasm" || len(wasm.Body.Bytes()) < 4 || string(wasm.Body.Bytes()[:4]) != "\x00asm" {
		t.Fatalf("WASM asset invalid: status=%d type=%q bytes=%d", wasm.Code, wasm.Header().Get("Content-Type"), wasm.Body.Len())
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	body, err := io.ReadAll(page.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	markup := string(body)
	for _, want := range []string{`data-poem-map-viewport`, `data-vector-status="loading"`, `aria-busy="true"`, `aria-keyshortcuts="ArrowUp ArrowDown ArrowLeft ArrowRight + - Q E PageUp PageDown Home"`, `src="/__poem/map-viewport.js?v=`} {
		if !strings.Contains(markup, want) {
			t.Fatalf("map page missing %q", want)
		}
	}
}

func TestMapResourceBrokerValidatesCurrentSourceAndProvider(t *testing.T) {
	tile := []byte{0x1a, 0x00}
	cachePolicy := app.MapCachePersistentOnlineOnly
	application := app.App[int]{
		Init: 1,
		View: func(int) app.Node {
			return app.MapViewportNode{
				Semantic: app.Semantic{ID: "map", Name: "Map", Enabled: true},
				Source:   app.MapSource{ID: "streets", ProviderID: "engine", Snapshot: "s1", MinZoom: 0, MaxZoom: 18},
				Camera:   app.MapCamera{Latitude: 52, Longitude: 4, Zoom: 10}, MinZoom: 0, MaxZoom: 18, MaxPitch: 70,
				CachePolicy: cachePolicy,
			}
		},
		Update: func(state int, _ app.Msg) (int, app.Cmd) { return state, app.Cmd{} },
		MapResources: func(int) []app.MapResourceProvider {
			return []app.MapResourceProvider{{ID: "engine", Fetch: func(_ context.Context, request app.MapResourceRequest) (app.MapResource, error) {
				if request.SourceID != "streets" || request.Snapshot != "s1" || request.Z != 3 || request.X != 4 || request.Y != 2 {
					t.Fatalf("request = %+v", request)
				}
				return app.MapResource{Bytes: tile, ContentType: "application/vnd.mapbox-vector-tile", ETag: `"tile"`, Immutable: true}, nil
			}}}
		},
	}
	handler, _, err := NewHandler(application, "Map", Options{SigningKey: []byte("01234567890123456789012345678901")})
	if err != nil {
		t.Fatal(err)
	}
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := home.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session cookie")
	}
	request := httptest.NewRequest(http.MethodGet, "/__poem/map-resource?provider=engine&source=streets&snapshot=s1&kind=0&z=3&x=4&y=2", nil)
	request.AddCookie(cookies[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != string(tile) {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" {
		t.Fatalf("cache = %q", response.Header().Get("Cache-Control"))
	}
	cachePolicy = app.MapCacheMemoryOnly
	memoryResponse := httptest.NewRecorder()
	handler.ServeHTTP(memoryResponse, request.Clone(request.Context()))
	if memoryResponse.Code != http.StatusOK || memoryResponse.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("memory cache: status=%d cache=%q", memoryResponse.Code, memoryResponse.Header().Get("Cache-Control"))
	}

	forged := httptest.NewRequest(http.MethodGet, "/__poem/map-resource?provider=engine&source=forged&snapshot=s1&kind=0&z=3&x=4&y=2", nil)
	forged.AddCookie(cookies[0])
	forgedResponse := httptest.NewRecorder()
	handler.ServeHTTP(forgedResponse, forged)
	if forgedResponse.Code != http.StatusBadRequest {
		t.Fatalf("forged status = %d", forgedResponse.Code)
	}
}

func TestMapFeatureCommitRequiresDeclaredInteractiveFeature(t *testing.T) {
	application := app.App[string]{
		View: func(selected string) app.Node {
			return app.MapViewportNode{
				Semantic: app.Semantic{ID: "map", Name: "Map", Enabled: true},
				Source:   app.MapSource{ID: "streets", ProviderID: "engine", MinZoom: 0, MaxZoom: 18},
				Camera:   app.MapCamera{Latitude: 52, Longitude: 4, Zoom: 10}, MaxZoom: 18, MaxPitch: 70,
				Features:  []app.MapFeature{{ID: "poi", Name: "POI-" + selected, Geometry: app.MapGeometryPoint, Role: app.MapFeaturePOI, Positions: []app.MapPosition{{Latitude: 52, Longitude: 4}}, OnActivate: app.Msg{Name: "select"}}},
				OnFeature: app.Msg{Name: "select-vector"},
			}
		},
		Update: func(state string, msg app.Msg) (string, app.Cmd) {
			if msg.Name == "select" {
				state = msg.Payload
			} else if msg.Name == "select-vector" {
				if activation, ok := msg.MapFeatureActivation(); ok {
					state = activation.ID
				}
			}
			return state, app.Cmd{}
		},
	}
	handler, _, err := NewHandler(application, "Map", Options{SigningKey: []byte("01234567890123456789012345678901")})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	home, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(home.Body)
	home.Body.Close()
	if !strings.Contains(string(body), `data-active-features="WzBd"`) {
		t.Fatalf("interactive feature indexes missing: %s", body)
	}
	post := func(field, value string) int {
		form := url.Values{csrfFieldName: {csrfFromBody(string(body))}, commitFieldName: {fieldPrefix + rootPath + field}, commitValueName: {value}}
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/__field", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(home.Cookies()[0])
		response, requestErr := client.Do(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if status := post("/feature-0", "forged"); status != http.StatusBadRequest {
		t.Fatalf("forged status=%d", status)
	}
	if status := post("/feature-0", "poi"); status != http.StatusSeeOther {
		t.Fatalf("valid status=%d", status)
	}
	activation := app.MapFeatureActivationPayload(app.MapFeatureActivation{ID: "14-1-2-poi-7", Name: "Museum", Category: "leisure-tourism", SourceLayer: "poi", Latitude: 52.1, Longitude: 4.2})
	if status := post("/vector-feature", activation); status != http.StatusSeeOther {
		t.Fatalf("vector feature status=%d", status)
	}
	finalRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	finalRequest.AddCookie(home.Cookies()[0])
	finalResponse, err := client.Do(finalRequest)
	if err != nil {
		t.Fatal(err)
	}
	finalBody, _ := io.ReadAll(finalResponse.Body)
	finalResponse.Body.Close()
	if !strings.Contains(string(finalBody), "POI-14-1-2-poi-7") {
		t.Fatalf("selection not persisted: %s", finalBody)
	}
}
