package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mulavdm/poem/pkg/app"
)

type testState struct{ Count int }

func testApp() app.App[testState] {
	return app.App[testState]{
		Init: testState{},
		View: func(state testState) app.Node {
			return app.Container(app.Vertical, 0,
				app.Text("count="+strconv.Itoa(state.Count)),
				app.Button("increment", app.Msg{Name: "increment"}),
			)
		},
		Update: func(state testState, msg app.Msg) (testState, app.Cmd) {
			if msg.Name == "increment" {
				state.Count++
			}
			return state, app.Cmd{}
		},
	}
}

type asyncState struct {
	Count   int
	Loading bool
}

func asyncApp(started chan<- struct{}, release <-chan struct{}, cancelled chan<- struct{}) app.App[asyncState] {
	return app.App[asyncState]{
		Init: asyncState{},
		View: func(state asyncState) app.Node {
			label := "count=" + strconv.Itoa(state.Count)
			if state.Loading {
				label = "loading"
			}
			return app.Container(app.Vertical, 0, app.Text(label), app.Button("start", app.Msg{Name: "start"}))
		},
		Update: func(state asyncState, msg app.Msg) (asyncState, app.Cmd) {
			switch msg.Name {
			case "start":
				state.Loading = true
				return state, app.Cmd{Name: "work.done", Run: func(ctx context.Context) (any, error) {
					if started != nil {
						started <- struct{}{}
					}
					select {
					case <-release:
						return 41, nil
					case <-ctx.Done():
						if cancelled != nil {
							cancelled <- struct{}{}
						}
						return nil, ctx.Err()
					}
				}}
			case "work.done":
				state.Loading = false
				if msg.Err == nil {
					state.Count = msg.Value.(int)
				}
			}
			return state, app.Cmd{}
		},
	}
}

func csrfFromBody(body string) string {
	start := strings.Index(body, `name="trellis_csrf" value="`) + len(`name="trellis_csrf" value="`)
	end := strings.Index(body[start:], `"`)
	return body[start : start+end]
}

func TestHandlerRequiresCSRFAndAllowsDeclaredEvent(t *testing.T) {
	handler, _, err := NewHandler(testApp(), "test", Options{SigningKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	get, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(get.Body)
	get.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `name="trellis_csrf"`) {
		t.Fatal("page did not contain csrf field")
	}
	csrfStart := strings.Index(string(body), `name="trellis_csrf" value="`) + len(`name="trellis_csrf" value="`)
	csrfEnd := strings.Index(string(body)[csrfStart:], `"`)
	csrf := string(body)[csrfStart : csrfStart+csrfEnd]
	cookie := get.Cookies()[0]

	form := "trellis_csrf=" + csrf + "&trellis_msg=increment"
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d", response.StatusCode)
	}

	bad, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader("trellis_csrf=bad&trellis_msg=increment"))
	bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	bad.AddCookie(cookie)
	badResponse, err := client.Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	badResponse.Body.Close()
	if badResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("bad csrf status = %d", badResponse.StatusCode)
	}
}

func TestDeclaredActionPayloadIsTransportedAndForgedPayloadIsRejected(t *testing.T) {
	var received app.Msg
	application := app.App[string]{
		View: func(string) app.Node {
			return app.ActionNode{Semantic: app.Semantic{ID: "settings", Name: "Settings", Enabled: true}, Label: "Settings", Invoke: app.Msg{Name: "panel", Payload: "settings"}}
		},
		Update: func(state string, message app.Msg) (string, app.Cmd) {
			received = message
			return message.Payload, app.Cmd{}
		},
	}
	handler, _, err := NewHandler(application, "test", Options{SigningKey: []byte(strings.Repeat("p", 32))})
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
	cookie := home.Cookies()[0]
	post := func(action string) int {
		form := url.Values{csrfFieldName: {csrfFromBody(string(body))}, msgFieldName: {action}}
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(cookie)
		response, postErr := client.Do(request)
		if postErr != nil {
			t.Fatal(postErr)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	if status := post(actionSubmitValue(app.Msg{Name: "panel", Payload: "settings"})); status != http.StatusSeeOther || received.Name != "panel" || received.Payload != "settings" {
		t.Fatalf("status=%d received=%+v", status, received)
	}
	if status := post("panel"); status != http.StatusBadRequest {
		t.Fatalf("forged payload-free action status=%d", status)
	}
}

func TestConfiguredSignerRejectsShortKey(t *testing.T) {
	if _, _, err := NewHandler(testApp(), "test", Options{SigningKey: []byte("short")}); err == nil {
		t.Fatal("expected short signing key to fail")
	}
}

func TestStartRunsOnceForNewSessionAndStylesLoadUnderCSP(t *testing.T) {
	var starts atomic.Int32
	application := testApp()
	application.Start = func(state testState) (testState, app.Cmd) { starts.Add(1); state.Count = 7; return state, app.Cmd{} }
	handler, _, err := NewHandler(application, "start", Options{SigningKey: []byte(strings.Repeat("s", 32))})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	first, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(first.Body)
	first.Body.Close()
	if starts.Load() != 1 || !strings.Contains(string(body), "count=7") {
		t.Fatalf("start=%d body=%s", starts.Load(), body)
	}
	if strings.Contains(string(body), "<style") || !strings.Contains(string(body), `href="/__poem/design.css"`) {
		t.Fatalf("page still relies on inline styling: %s", body)
	}
	if csp := first.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "style-src 'self'") || strings.Contains(csp, "unsafe-inline") {
		t.Fatalf("CSP=%q", csp)
	}
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	request.AddCookie(first.Cookies()[0])
	second, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if starts.Load() != 1 {
		t.Fatalf("start reran: %d", starts.Load())
	}
	css, err := server.Client().Get(server.URL + "/__poem/design.css")
	if err != nil {
		t.Fatal(err)
	}
	cssBody, _ := io.ReadAll(css.Body)
	css.Body.Close()
	if css.StatusCode != http.StatusOK || !strings.Contains(string(cssBody), "--poem-primary") {
		t.Fatalf("design CSS unavailable: %d %s", css.StatusCode, cssBody)
	}
}

func TestImageViewportFieldCommitValidatesAndPersists(t *testing.T) {
	type viewportState struct {
		Transform app.ImageTransform
		Point     app.ViewportPoint
		Marker    string
	}
	application := app.App[viewportState]{
		View: func(state viewportState) app.Node {
			image := app.Image([]byte("\x89PNG\r\n\x1a\n"), "map")
			node := app.ImageViewport(image, app.Msg{Name: "viewport"})
			node.Transform = state.Transform
			node.OnActivate = app.Msg{Name: "point"}
			node.Markers = []app.ImageMarker{{ID: "place-1", X: 0.25, Y: 0.75, Label: "Place", OnActivate: app.Msg{Name: "marker"}}}
			return node
		},
		Update: func(state viewportState, msg app.Msg) (viewportState, app.Cmd) {
			if msg.Name == "viewport" {
				if transform, ok := msg.ImageTransform(); ok {
					state.Transform = transform
				}
			}
			if msg.Name == "point" {
				state.Point, _ = msg.ViewportPoint()
			}
			if msg.Name == "marker" {
				state.Marker = msg.Payload
			}
			return state, app.Cmd{}
		},
	}
	handler, _, err := NewHandler(application, "viewport", Options{SigningKey: []byte(strings.Repeat("v", 32))})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	get, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	cookie := get.Cookies()[0]
	csrf := csrfFromBody(string(body))
	post := func(field, value string) int {
		form := url.Values{csrfFieldName: {csrf}, commitFieldName: {field}, commitValueName: {value}}
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/__field", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if status := post(fieldPrefix+rootPath, app.ImageTransformPayload(app.ImageTransform{OffsetX: 0.2, Scale: 2})); status != http.StatusSeeOther {
		t.Fatalf("valid status = %d", status)
	}
	if status := post("forged", app.ImageTransformPayload(app.ImageTransform{Scale: 2})); status != http.StatusBadRequest {
		t.Fatalf("forged status = %d", status)
	}
	if status := post(fieldPrefix+rootPath, `{"x":9,"y":0,"scale":1}`); status != http.StatusBadRequest {
		t.Fatalf("invalid transform status = %d", status)
	}
	if status := post(fieldPrefix+rootPath, `{"x":0,"y":0,"scale":9}`); status != http.StatusBadRequest {
		t.Fatalf("out-of-range scale status = %d", status)
	}
	if status := post(fieldPrefix+rootPath+"/activate", app.ViewportPointPayload(app.ViewportPoint{X: 0.4, Y: 0.6})); status != http.StatusSeeOther {
		t.Fatalf("point status = %d", status)
	}
	if status := post(fieldPrefix+rootPath+"/marker-0", "place-1"); status != http.StatusSeeOther {
		t.Fatalf("marker status = %d", status)
	}
	if status := post(fieldPrefix+rootPath+"/marker-0", "forged"); status != http.StatusBadRequest {
		t.Fatalf("forged marker status = %d", status)
	}
}

func TestFileStateStoreSurvivesHandlerRecreation(t *testing.T) {
	dir := t.TempDir()
	key := []byte(strings.Repeat("r", 32))
	store, err := NewFileStateStore(dir, testState{})
	if err != nil {
		t.Fatal(err)
	}
	handler, _, err := NewHandlerWithStore(testApp(), "test", Options{SigningKey: key}, store)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	get, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	csrfStart := strings.Index(string(body), `name="trellis_csrf" value="`) + len(`name="trellis_csrf" value="`)
	csrfEnd := strings.Index(string(body)[csrfStart:], `"`)
	csrf := string(body)[csrfStart : csrfStart+csrfEnd]
	cookie := get.Cookies()[0]
	form := "trellis_csrf=" + csrf + "&trellis_msg=increment"
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	server.Close()

	storeAfterRestart, err := NewFileStateStore(dir, testState{})
	if err != nil {
		t.Fatal(err)
	}
	handlerAfterRestart, _, err := NewHandlerWithStore(testApp(), "test", Options{SigningKey: key}, storeAfterRestart)
	if err != nil {
		t.Fatal(err)
	}
	serverAfterRestart := httptest.NewServer(handlerAfterRestart)
	defer serverAfterRestart.Close()
	restartRequest, _ := http.NewRequest(http.MethodGet, serverAfterRestart.URL+"/", nil)
	restartRequest.AddCookie(cookie)
	restartResponse, err := serverAfterRestart.Client().Do(restartRequest)
	if err != nil {
		t.Fatal(err)
	}
	restartBody, _ := io.ReadAll(restartResponse.Body)
	restartResponse.Body.Close()
	if !strings.Contains(string(restartBody), "count=1") {
		t.Fatalf("persisted state missing after handler recreation: %s", restartBody)
	}
}

func TestHandlerSerializesConcurrentSessionUpdates(t *testing.T) {
	handler, _, err := NewHandler(testApp(), "test", Options{SigningKey: []byte(strings.Repeat("c", 32))})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	get, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	csrfStart := strings.Index(string(body), `name="trellis_csrf" value="`) + len(`name="trellis_csrf" value="`)
	csrfEnd := strings.Index(string(body)[csrfStart:], `"`)
	csrf := string(body)[csrfStart : csrfStart+csrfEnd]
	cookie := get.Cookies()[0]

	const updates = 16
	results := make(chan error, updates)
	for i := 0; i < updates; i++ {
		go func() {
			request, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader("trellis_csrf="+csrf+"&trellis_msg=increment"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.AddCookie(cookie)
			response, requestErr := client.Do(request)
			if requestErr == nil {
				response.Body.Close()
				if response.StatusCode != http.StatusSeeOther {
					requestErr = fmt.Errorf("status = %d", response.StatusCode)
				}
			}
			results <- requestErr
		}()
	}
	for i := 0; i < updates; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	finalRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	finalRequest.AddCookie(cookie)
	finalResponse, err := client.Do(finalRequest)
	if err != nil {
		t.Fatal(err)
	}
	finalBody, _ := io.ReadAll(finalResponse.Body)
	finalResponse.Body.Close()
	if !strings.Contains(string(finalBody), "count=16") {
		t.Fatalf("concurrent updates lost: %s", finalBody)
	}
}

func TestAsyncCommandRedirectsToRefreshingLoadingPageThenPersistsResult(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	handler, _, err := NewHandler(asyncApp(started, release, nil), "async", Options{SigningKey: []byte(strings.Repeat("a", 32))})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	get, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	cookie := get.Cookies()[0]
	csrf := csrfFromBody(string(body))

	forged, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader("trellis_csrf="+csrf+"&trellis_msg=work.done"))
	forged.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	forged.AddCookie(cookie)
	forgedResponse, err := client.Do(forged)
	if err != nil {
		t.Fatal(err)
	}
	forgedResponse.Body.Close()
	if forgedResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("forged completion status = %d", forgedResponse.StatusCode)
	}

	request, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader("trellis_csrf="+csrf+"&trellis_msg=start"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d", response.StatusCode)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("command did not start")
	}

	loadingRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	loadingRequest.AddCookie(cookie)
	loadingResponse, err := client.Do(loadingRequest)
	if err != nil {
		t.Fatal(err)
	}
	loadingBody, _ := io.ReadAll(loadingResponse.Body)
	loadingResponse.Body.Close()
	if !strings.Contains(string(loadingBody), "loading") || !strings.Contains(string(loadingBody), `http-equiv="refresh"`) {
		t.Fatalf("pending page missing loading refresh: %s", loadingBody)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		finalRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
		finalRequest.AddCookie(cookie)
		finalResponse, getErr := client.Do(finalRequest)
		if getErr != nil {
			t.Fatal(getErr)
		}
		finalBody, _ := io.ReadAll(finalResponse.Body)
		finalResponse.Body.Close()
		if strings.Contains(string(finalBody), "count=41") {
			if strings.Contains(string(finalBody), `http-equiv="refresh"`) {
				t.Fatal("completed page still refreshes")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("completion not persisted: %s", finalBody)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSessionEvictionCancelsCommand(t *testing.T) {
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)
	release := make(chan struct{})
	handler, _, err := NewHandler(asyncApp(started, release, cancelled), "async", Options{SigningKey: []byte(strings.Repeat("e", 32)), MaxSessions: 1})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	first := server.Client()
	first.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	get, _ := first.Get(server.URL + "/")
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader("trellis_csrf="+csrfFromBody(string(body))+"&trellis_msg=start"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(get.Cookies()[0])
	posted, err := first.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	posted.Body.Close()
	<-started

	second := server.Client()
	second.Jar = nil
	secondGet, err := second.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	secondGet.Body.Close()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("eviction did not cancel command")
	}
}

type failingStore struct{ saves atomic.Int32 }

func (s *failingStore) Load(context.Context, string) (asyncState, bool, error) {
	return asyncState{}, false, nil
}
func (s *failingStore) Save(context.Context, string, asyncState) error {
	if s.saves.Add(1) > 1 {
		return fmt.Errorf("save failed")
	}
	return nil
}
func (*failingStore) Delete(context.Context, string) error { return nil }

func TestCommandDoesNotStartWhenLoadingStateCannotBeSaved(t *testing.T) {
	started := make(chan struct{}, 1)
	store := &failingStore{}
	handler, _, err := NewHandlerWithStore(asyncApp(started, make(chan struct{}), nil), "async", Options{SigningKey: []byte(strings.Repeat("f", 32))}, store)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	get, _ := client.Get(server.URL + "/")
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/__event", strings.NewReader("trellis_csrf="+csrfFromBody(string(body))+"&trellis_msg=start"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(get.Cookies()[0])
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.StatusCode)
	}
	select {
	case <-started:
		t.Fatal("command started despite persistence failure")
	case <-time.After(50 * time.Millisecond):
	}
}
