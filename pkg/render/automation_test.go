package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"

	"go_native_gpu_gui/pkg/render/components"
	"go_native_gpu_gui/pkg/render/protocol"
)

func TestBuildAutomationSnapshotIncludesChildren(t *testing.T) {
	globalState = &ApplicationState{
		Pages:       make(map[string][]Component),
		CurrentPage: PageDashboard,
	}
	globalState.Pages[PageDashboard] = []Component{
		&FlexBox{
			CompID:         "root",
			Rect:           image.Rect(0, 0, 200, 80),
			Direction:      Vertical,
			JustifyContent: JustifyStart,
			AlignItems:     AlignStretch,
			Children: []Component{
				&components.Button{CompID: "btn_a", Rect: image.Rect(0, 0, 100, 40), Label: "A"},
			},
		},
	}

	nodes, flat := buildAutomationSnapshot()
	if len(nodes) != 1 {
		t.Fatalf("expected one root node, got %d", len(nodes))
	}
	if len(nodes[0].Children) != 1 || nodes[0].Children[0].ID != "btn_a" {
		t.Fatalf("unexpected child snapshot: %+v", nodes[0].Children)
	}
	if len(flat) < 2 {
		t.Fatalf("expected flattened nodes to include child, got %d", len(flat))
	}
}

func TestAutomationSetTextUpdatesState(t *testing.T) {
	globalState = &ApplicationState{
		Pages:           make(map[string][]Component),
		CurrentPage:     PageDashboard,
		TextInputValues: make(map[string]string),
	}
	input := &components.TextInput{CompID: "txt_name", Rect: image.Rect(10, 10, 120, 40)}
	globalState.Pages[PageDashboard] = []Component{input}

	if err := automationSetText("txt_name", "hello"); err != nil {
		t.Fatal(err)
	}
	if got := globalState.TextInputValues["txt_name"]; got != "hello" {
		t.Fatalf("unexpected stored text: %q", got)
	}
	if globalState.FocusedID != "txt_name" {
		t.Fatalf("expected focused id to be txt_name, got %q", globalState.FocusedID)
	}
}

func TestAutomationClickComponentInvokesButton(t *testing.T) {
	globalState = &ApplicationState{
		Pages:       make(map[string][]Component),
		CurrentPage: PageDashboard,
	}
	clicked := false
	button := &components.Button{
		CompID:    "btn_test",
		Rect:      image.Rect(0, 0, 100, 40),
		Label:     "Click",
		OnClick:   func(state *ApplicationState) { clicked = true },
		BaseColor: color.RGBA{10, 10, 10, 255},
	}
	globalState.Pages[PageDashboard] = []Component{button}

	if err := automationClickComponent("btn_test"); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatalf("expected button click callback to fire")
	}
}

func TestRenderFrameToImageDrawsPixels(t *testing.T) {
	frame := protocol.RenderFrame{
		Width:  20,
		Height: 20,
		Commands: []protocol.DrawCommand{
			{
				Type: protocol.DrawCommandTypeFillRect,
				X1:   0,
				Y1:   0,
				X2:   10,
				Y2:   10,
				R:    255,
				A:    255,
			},
		},
	}
	img := renderFrameToImage(frame)
	if got := img.RGBAAt(5, 5); got.R != 255 {
		t.Fatalf("expected red pixel, got %+v", got)
	}
}

func TestAutomationHTTPStateEndpoint(t *testing.T) {
	globalState = &ApplicationState{
		Pages:        make(map[string][]Component),
		CurrentPage:  PageDashboard,
		FocusedID:    "focus_a",
		HoveredID:    "hover_a",
		WindowWidth:  640,
		WindowHeight: 480,
	}

	req := httptest.NewRequest(http.MethodGet, "/state", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp AutomationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.OK || resp.CurrentPage != PageDashboard || resp.FocusedID != "focus_a" {
		t.Fatalf("unexpected state response: %+v", resp)
	}
}

func TestAutomationHTTPComponentsEndpoint(t *testing.T) {
	globalState = &ApplicationState{
		Pages:       make(map[string][]Component),
		CurrentPage: PageDashboard,
	}
	globalState.Pages[PageDashboard] = []Component{
		&FlexBox{
			CompID: "root",
			Rect:   image.Rect(0, 0, 240, 80),
			Children: []Component{
				&components.Button{CompID: "btn_http", Rect: image.Rect(0, 0, 100, 40), Label: "HTTP"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/components", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp AutomationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Nodes) != 1 || len(resp.Flat) < 2 {
		t.Fatalf("unexpected component snapshot: %+v", resp)
	}
}

func TestAutomationHTTPSetTextUpdatesTextInput(t *testing.T) {
	globalState = &ApplicationState{
		Pages:           make(map[string][]Component),
		CurrentPage:     PageDashboard,
		TextInputValues: make(map[string]string),
	}
	globalState.Pages[PageDashboard] = []Component{
		&components.TextInput{CompID: "txt_http", Rect: image.Rect(0, 0, 140, 40)},
	}

	body := bytes.NewBufferString(`{"id":"txt_http","value":"updated"}`)
	req := httptest.NewRequest(http.MethodPost, "/set-text", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := globalState.TextInputValues["txt_http"]; got != "updated" {
		t.Fatalf("expected updated text, got %q", got)
	}
}

func TestAutomationHTTPClickTriggersButton(t *testing.T) {
	globalState = &ApplicationState{
		Pages:       make(map[string][]Component),
		CurrentPage: PageDashboard,
	}
	clicked := false
	globalState.Pages[PageDashboard] = []Component{
		&components.Button{
			CompID:    "btn_http_click",
			Rect:      image.Rect(0, 0, 120, 40),
			Label:     "Click",
			OnClick:   func(state *ApplicationState) { clicked = true },
			BaseColor: color.RGBA{10, 10, 10, 255},
		},
	}

	body := bytes.NewBufferString(`{"id":"btn_http_click"}`)
	req := httptest.NewRequest(http.MethodPost, "/click", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !clicked {
		t.Fatalf("expected click callback to fire")
	}
}

func TestAutomationHTTPFrameEndpointReturnsPNG(t *testing.T) {
	globalLastFrame = protocol.RenderFrame{
		Width:  16,
		Height: 16,
		Commands: []protocol.DrawCommand{
			{
				Type: protocol.DrawCommandTypeFillRect,
				X1:   0,
				Y1:   0,
				X2:   16,
				Y2:   16,
				G:    255,
				A:    255,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/frame", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png, got %q", got)
	}
	body := rec.Body.Bytes()
	if len(body) < 8 || string(body[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("response is not a png payload")
	}
}

func TestAutomationHTTPPrepareWindowUsesNativeHelper(t *testing.T) {
	previous := nativeDebugRequest
	t.Cleanup(func() {
		nativeDebugRequest = previous
	})

	var captured protocol.NativeDebugRequest
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		captured = req
		return protocol.NativeDebugResponse{
			DPI:              144,
			WindowVisible:    true,
			WindowMinimized:  false,
			WindowForeground: true,
			ClientWidth:      1200,
			ClientHeight:     800,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/prepare-window", bytes.NewBufferString(`{"bring_to_foreground":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !captured.RestoreWindow || !captured.ClampToWorkArea || captured.BringToForeground {
		t.Fatalf("unexpected native debug request: %+v", captured)
	}

	var state NativeAutomationState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to decode native state response: %v body=%s", err, rec.Body.String())
	}
	if !state.WindowVisible || !state.WindowForeground || state.ClientWidth != 1200 {
		t.Fatalf("unexpected native state response: %+v", state)
	}
}

func TestAutomationHTTPPrepareWindowRejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/prepare-window", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp AutomationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.OK || resp.Error == "" {
		t.Fatalf("expected error response, got %+v", resp)
	}
}

func TestAutomationHTTPPrepareWindowPropagatesNativeErrors(t *testing.T) {
	previous := nativeDebugRequest
	t.Cleanup(func() {
		nativeDebugRequest = previous
	})
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		return protocol.NativeDebugResponse{}, fmt.Errorf("native boom")
	}

	req := httptest.NewRequest(http.MethodPost, "/prepare-window", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestInspectFramePNGPrefersDesktopWhenForegrounded(t *testing.T) {
	previous := nativeDebugRequest
	t.Cleanup(func() {
		nativeDebugRequest = previous
	})

	call := 0
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		call++
		switch call {
		case 1:
			if !req.RestoreWindow || !req.ClampToWorkArea || !req.BringToForeground {
				t.Fatalf("unexpected prepare request: %+v", req)
			}
			return protocol.NativeDebugResponse{
				WindowVisible:    true,
				WindowMinimized:  false,
				WindowForeground: true,
			}, nil
		case 2:
			if !req.CaptureDesktopFrame {
				t.Fatalf("expected desktop capture request, got %+v", req)
			}
			return protocol.NativeDebugResponse{
				FrameWidth:  1,
				FrameHeight: 1,
				FrameRGBA:   []byte{255, 0, 0, 255},
			}, nil
		default:
			t.Fatalf("unexpected extra native request: %+v", req)
			return protocol.NativeDebugResponse{}, nil
		}
	}

	pngBytes, source, state, err := inspectFramePNG(InspectFrameRequest{
		PrepareWindow:     true,
		RestoreWindow:     true,
		ClampToWorkArea:   true,
		BringToForeground: true,
		PreferDesktop:     true,
		FallbackToSelf:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if source != "desktop" {
		t.Fatalf("expected desktop source, got %q", source)
	}
	if !state.WindowForeground {
		t.Fatalf("expected foreground state, got %+v", state)
	}
	if len(pngBytes) < 8 || string(pngBytes[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG payload")
	}
}

func TestInspectFramePNGFallsBackToSelfWhenNotForegrounded(t *testing.T) {
	previous := nativeDebugRequest
	t.Cleanup(func() {
		nativeDebugRequest = previous
	})

	call := 0
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		call++
		switch call {
		case 1:
			return protocol.NativeDebugResponse{
				WindowVisible:    true,
				WindowMinimized:  false,
				WindowForeground: false,
			}, nil
		case 2:
			if !req.CaptureFrame {
				t.Fatalf("expected self-frame capture request, got %+v", req)
			}
			return protocol.NativeDebugResponse{
				FrameWidth:  1,
				FrameHeight: 1,
				FrameRGBA:   []byte{0, 255, 0, 255},
			}, nil
		default:
			t.Fatalf("unexpected extra native request: %+v", req)
			return protocol.NativeDebugResponse{}, nil
		}
	}

	_, source, state, err := inspectFramePNG(InspectFrameRequest{
		PrepareWindow:     true,
		RestoreWindow:     true,
		ClampToWorkArea:   true,
		BringToForeground: true,
		PreferDesktop:     true,
		FallbackToSelf:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if source != "self" {
		t.Fatalf("expected self source, got %q", source)
	}
	if state.WindowForeground {
		t.Fatalf("expected non-foreground state, got %+v", state)
	}
}

func TestAutomationHTTPInspectFrameReturnsPNGAndSourceHeader(t *testing.T) {
	previous := nativeDebugRequest
	t.Cleanup(func() {
		nativeDebugRequest = previous
	})

	call := 0
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		call++
		switch call {
		case 1:
			return protocol.NativeDebugResponse{
				WindowVisible:    true,
				WindowMinimized:  false,
				WindowForeground: false,
			}, nil
		case 2:
			return protocol.NativeDebugResponse{
				FrameWidth:  1,
				FrameHeight: 1,
				FrameRGBA:   []byte{0, 0, 255, 255},
			}, nil
		default:
			t.Fatalf("unexpected extra native request: %+v", req)
			return protocol.NativeDebugResponse{}, nil
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/inspect-frame", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png, got %q", got)
	}
	if got := rec.Header().Get("X-POEM-Frame-Source"); got != "self" {
		t.Fatalf("expected self frame source, got %q", got)
	}
	body := rec.Body.Bytes()
	if len(body) < 8 || string(body[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("response is not a png payload")
	}
}
