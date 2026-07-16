package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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
		Update: func(state testState, msg app.Msg) testState {
			if msg.Name == "increment" {
				state.Count++
			}
			return state
		},
	}
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

func TestConfiguredSignerRejectsShortKey(t *testing.T) {
	if _, _, err := NewHandler(testApp(), "test", Options{SigningKey: []byte("short")}); err == nil {
		t.Fatal("expected short signing key to fail")
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
