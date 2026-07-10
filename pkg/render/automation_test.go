package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_native_gpu_gui/pkg/render/components"
	"go_native_gpu_gui/pkg/render/protocol"
	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
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

func TestBuildAutomationSnapshotIncludesSemanticDescendants(t *testing.T) {
	globalState = &ApplicationState{
		Pages:       make(map[string][]Component),
		CurrentPage: PageDashboard,
	}
	menu := components.NewMenu("file", []components.MenuItem{
		{ID: "open", Label: "Open"},
		{ID: "disabled", Label: "Disabled", Disabled: true},
	})
	menu.Rect = image.Rect(10, 10, 210, 90)
	globalState.Pages[PageDashboard] = []Component{menu}

	nodes, flat := buildAutomationSnapshot()
	if len(nodes) != 1 || len(nodes[0].Children) != 2 {
		t.Fatalf("expected semantic menu items in hierarchy, got %+v", nodes)
	}
	if nodes[0].Children[0].ID != "file/open" || nodes[0].Children[0].Role != "menu-item" {
		t.Fatalf("unexpected first semantic child: %+v", nodes[0].Children[0])
	}
	if !nodes[0].Children[1].Disabled {
		t.Fatalf("expected disabled semantic state: %+v", nodes[0].Children[1])
	}
	found := false
	for _, node := range flat {
		if node.ID == "file/open" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected semantic descendant in flat automation snapshot: %+v", flat)
	}
}

func TestBuildAutomationSnapshotUsesPortableChildTraversal(t *testing.T) {
	globalState = &ApplicationState{Pages: make(map[string][]Component), CurrentPage: PageDashboard}
	content := components.NewLabel("settings.appearance.content", "Theme settings")
	accordion := components.NewAccordion("settings", []components.AccordionItem{{ID: "appearance", Title: "Appearance", Content: content}}, map[string]bool{"appearance": true}, nil)
	accordion.Rect = image.Rect(0, 0, 320, 100)
	accordion.Measure(image.Pt(320, 100), globalState)
	globalState.Pages[PageDashboard] = []Component{accordion}

	_, flat := buildAutomationSnapshot()
	foundHeader, foundContent := false, false
	for _, node := range flat {
		switch node.ID {
		case "settings/appearance":
			foundHeader = true
		case "settings.appearance.content":
			foundContent = true
		}
	}
	if !foundHeader || !foundContent {
		t.Fatalf("portable traversal missing header=%v content=%v: %+v", foundHeader, foundContent, flat)
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

func TestAutomationPressKeySupportsCollectionNavigation(t *testing.T) {
	globalState = &ApplicationState{
		Pages:           make(map[string][]Component),
		CurrentPage:     PageDashboard,
		FocusedID:       "table",
		TransientState:  renderstate.NewStore(),
		ScrollPositions: make(map[string]int),
		ScrollCurrent:   make(map[string]float64),
	}
	selected := "a"
	table := components.NewDataTable("table", []components.TableColumn{{Key: "name", Title: "Name"}}, []components.TableRow{
		{ID: "a", Values: map[string]string{"name": "Alpha"}},
		{ID: "b", Values: map[string]string{"name": "Beta"}},
		{ID: "c", Values: map[string]string{"name": "Gamma"}},
	})
	table.Rect = image.Rect(0, 0, 240, 70)
	table.SelectedID = selected
	table.OnSelect = func(id string, _ *ApplicationState) {
		selected = id
		table.SelectedID = id
	}
	globalState.Pages[PageDashboard] = []Component{table}

	for _, step := range []struct {
		key, want string
	}{{"Down", "b"}, {"End", "c"}, {"Home", "a"}, {"Page Down", "b"}, {"ArrowUp", "a"}} {
		if err := automationPressKey(step.key); err != nil {
			t.Fatalf("press %q: %v", step.key, err)
		}
		if selected != step.want {
			t.Fatalf("press %q selected %q, want %q", step.key, selected, step.want)
		}
	}
}

func TestAutomationPressKeyRoutesToFocusedOverlay(t *testing.T) {
	globalState = &ApplicationState{
		Pages:       make(map[string][]Component),
		CurrentPage: PageDashboard,
	}
	invoked := ""
	menu := components.NewMenu("menu", []components.MenuItem{
		{ID: "new", Label: "New"},
		{ID: "open", Label: "Open", OnInvoke: func(*ApplicationState) { invoked = "open" }},
	})
	menu.OnDismiss = func(state *ApplicationState) { state.CloseOverlay("menu") }
	globalState.FocusedID = "launcher"
	globalState.OpenFocusedOverlay("menu", menu, true)
	if err := automationPressKey("Down"); err != nil {
		t.Fatal(err)
	}
	if err := automationPressKey("Enter"); err != nil {
		t.Fatal(err)
	}
	if invoked != "open" || len(globalState.Overlays.Snapshot()) != 0 || globalState.FocusedID != "launcher" {
		t.Fatalf("overlay key route invoked=%q overlays=%d focus=%q", invoked, len(globalState.Overlays.Snapshot()), globalState.FocusedID)
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

func TestAutomationSnapshotDoesNotExposeMaskedText(t *testing.T) {
	globalState = &ApplicationState{Pages: make(map[string][]Component), CurrentPage: PageDashboard, TextInputValues: make(map[string]string), TransientState: renderstate.NewStore()}
	input := &components.TextInput{CompID: "password", Text: "secret", Masked: true, Rect: image.Rect(0, 0, 180, 40)}
	input.CursorIndex = len([]rune(input.Text))
	globalState.Pages[PageDashboard] = []Component{input}
	_, flat := buildAutomationSnapshot()
	if len(flat) != 1 || flat[0].Text != "" || flat[0].Value != "" || flat[0].SelectionStart != 0 || flat[0].SelectionEnd != 0 {
		t.Fatalf("masked automation node exposed data: %+v", flat)
	}
}

func TestAutomationHTTPSelectTextPublishesRuneRange(t *testing.T) {
	globalState = &ApplicationState{
		Pages:           make(map[string][]Component),
		CurrentPage:     PageDashboard,
		TextInputValues: make(map[string]string),
		TransientState:  renderstate.NewStore(),
	}
	input := &components.TextInput{CompID: "txt_select", Text: "A日本語Z", Rect: image.Rect(0, 0, 180, 40)}
	globalState.Pages[PageDashboard] = []Component{input}

	body := bytes.NewBufferString(`{"id":"txt_select","start":1,"end":4}`)
	req := httptest.NewRequest(http.MethodPost, "/select-text", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	start, end := input.Selection(globalState)
	if start != 1 || end != 4 {
		t.Fatalf("selection=%d:%d", start, end)
	}
	_, flat := buildAutomationSnapshot()
	if len(flat) != 1 || flat[0].SelectionStart != 1 || flat[0].SelectionEnd != 4 {
		t.Fatalf("automation selection snapshot: %+v", flat)
	}
}

func TestAutomationHTTPCompositionCommitsUnicode(t *testing.T) {
	globalState = &ApplicationState{Pages: make(map[string][]Component), CurrentPage: PageDashboard, TextInputValues: make(map[string]string), TransientState: renderstate.NewStore()}
	input := &components.TextInput{CompID: "txt_ime", Text: "A-Z", Rect: image.Rect(0, 0, 180, 40)}
	globalState.Pages[PageDashboard] = []Component{input}
	input.SetSelection(1, 2, globalState)
	handler := newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{}))
	for _, request := range []struct{ path, body string }{
		{"/composition-start", `{"id":"txt_ime"}`},
		{"/composition-update", `{"id":"txt_ime","value":"日本"}`},
		{"/composition-end", `{"id":"txt_ime","value":"日本"}`},
	} {
		req := httptest.NewRequest(http.MethodPost, request.path, bytes.NewBufferString(request.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", request.path, rec.Code, rec.Body.String())
		}
	}
	if input.Text != "A日本Z" {
		t.Fatalf("composition committed %q", input.Text)
	}
}

func TestAutomationTextAreaSelectionAndComposition(t *testing.T) {
	globalState = &ApplicationState{Pages: make(map[string][]Component), CurrentPage: PageDashboard, TextInputValues: make(map[string]string), TransientState: renderstate.NewStore()}
	area := &components.TextArea{CompID: "notes", Text: "one two", Rect: image.Rect(0, 0, 240, 120)}
	globalState.Pages[PageDashboard] = []Component{area}
	handler := newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{}))
	for _, request := range []struct{ path, body string }{
		{"/select-text", `{"id":"notes","start":4,"end":7}`},
		{"/composition-start", `{"id":"notes"}`},
		{"/composition-update", `{"id":"notes","value":"日本"}`},
		{"/composition-end", `{"id":"notes","value":"日本"}`},
	} {
		req := httptest.NewRequest(http.MethodPost, request.path, bytes.NewBufferString(request.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", request.path, rec.Code, rec.Body.String())
		}
	}
	if area.Text != "one 日本" {
		t.Fatalf("composition committed %q", area.Text)
	}
	_, flat := buildAutomationSnapshot()
	if len(flat) != 1 || flat[0].SelectionStart != 6 || flat[0].SelectionEnd != 6 {
		t.Fatalf("textarea automation snapshot: %+v", flat)
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

func TestAutomationHTTPClickInvokesSemanticTabTarget(t *testing.T) {
	globalState = &ApplicationState{Pages: make(map[string][]Component), CurrentPage: PageDashboard}
	selected := ""
	tabs := components.NewTabs("views", []components.TabItem{{ID: "details", Label: "Details"}, {ID: "history", Label: "History"}}, "details", func(id string, _ *ApplicationState) { selected = id })
	tabs.Rect = image.Rect(0, 0, 240, 36)
	globalState.Pages[PageDashboard] = []Component{tabs}
	body := bytes.NewBufferString(`{"id":"views/history"}`)
	req := httptest.NewRequest(http.MethodPost, "/click", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if selected != "history" {
		t.Fatalf("selected tab %q", selected)
	}
}

func TestAutomationClickUsesDisclosureSemanticAction(t *testing.T) {
	globalState = &ApplicationState{
		Pages:          make(map[string][]Component),
		CurrentPage:    PageDashboard,
		TransientState: renderstate.NewStore(),
	}
	accordion := components.NewAccordion("settings", []components.AccordionItem{{ID: "advanced", Title: "Advanced"}}, map[string]bool{}, nil)
	accordion.Rect = image.Rect(0, 0, 240, 36)
	globalState.Pages[PageDashboard] = []Component{accordion}
	if err := automationClickComponent("settings/advanced"); err != nil {
		t.Fatal(err)
	}
	if !accordion.Expanded["advanced"] {
		t.Fatal("semantic click did not choose the advertised expand action")
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

func TestAutomationHTTPPrepareWindowPassesMaximizeFlag(t *testing.T) {
	previous := nativeDebugRequest
	t.Cleanup(func() {
		nativeDebugRequest = previous
	})

	var captured protocol.NativeDebugRequest
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		captured = req
		return protocol.NativeDebugResponse{WindowVisible: true}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/prepare-window", bytes.NewBufferString(`{"maximize_window":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !captured.MaximizeWindow {
		t.Fatalf("expected maximize flag in native request, got %+v", captured)
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

func TestAutomationHTTPPerfStateEndpoint(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })
	globalPerfTracker.recordFrame(10*time.Millisecond, 5*time.Millisecond, 2*time.Millisecond, time.Millisecond, 18*time.Millisecond, 42)

	req := httptest.NewRequest(http.MethodGet, "/perf/state", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var state PerfState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if state.Frames.FrameCount != 1 || state.Frames.LastTotalMS <= 0 {
		t.Fatalf("unexpected perf state: %+v", state)
	}
}

func TestAutomationHTTPPerfResetEndpoint(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })
	globalPerfTracker.recordAutomationAction("click", 20*time.Millisecond, "")

	req := httptest.NewRequest(http.MethodPost, "/perf/reset", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var state PerfState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if state.Automation.ActionCount != 0 || len(state.Events) != 0 {
		t.Fatalf("expected reset tracker, got %+v", state)
	}
}

func TestAutomationHTTPMeasureActionReportsLatency(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })
	globalState = &ApplicationState{
		Pages:       make(map[string][]Component),
		CurrentPage: PageDashboard,
	}
	clicked := false
	globalState.Pages[PageDashboard] = []Component{
		&components.Button{
			CompID:    "btn_measure",
			Rect:      image.Rect(0, 0, 100, 40),
			Label:     "Measure",
			OnClick:   func(state *ApplicationState) { clicked = true; state.StatusText = "clicked" },
			BaseColor: color.RGBA{10, 10, 10, 255},
		},
	}

	body := bytes.NewBufferString(`{"command":"click","id":"btn_measure","condition":{"focused_id":"btn_measure"}}`)
	req := httptest.NewRequest(http.MethodPost, "/perf/measure-action", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !clicked {
		t.Fatalf("expected click callback to fire")
	}
	var resp PerfMeasureActionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	if !resp.OK || !resp.ConditionMet || resp.ElapsedMS < 0 {
		t.Fatalf("unexpected measure response: %+v", resp)
	}
	if resp.Perf.Automation.ActionCount == 0 {
		t.Fatalf("expected automation perf entries, got %+v", resp.Perf)
	}
}

func TestAutomationHTTPMeasureNativeActionReportsLatency(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })

	previous := nativeDebugRequest
	t.Cleanup(func() {
		nativeDebugRequest = previous
	})

	call := 0
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		call++
		if call == 1 {
			if !req.MaximizeWindow || !req.BringToForeground {
				t.Fatalf("unexpected native request: %+v", req)
			}
			return protocol.NativeDebugResponse{
				WindowVisible:    true,
				WindowForeground: false,
				WindowMinimized:  false,
				ClientWidth:      900,
				ClientHeight:     700,
			}, nil
		}
		return protocol.NativeDebugResponse{
			WindowVisible:    true,
			WindowForeground: true,
			WindowMinimized:  false,
			ClientWidth:      1600,
			ClientHeight:     1000,
			BackbufferWidth:  1600,
			BackbufferHeight: 1000,
		}, nil
	}

	body := bytes.NewBufferString(`{"maximize_window":true,"bring_to_foreground":true,"condition":{"window_foreground":true,"window_not_minimized":true,"client_width_at_least":1200}}`)
	req := httptest.NewRequest(http.MethodPost, "/perf/measure-native-action", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp PerfMeasureNativeActionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	if !resp.OK || !resp.ConditionMet || !resp.NativeState.WindowForeground || resp.NativeState.ClientWidth < 1200 {
		t.Fatalf("unexpected native measure response: %+v", resp)
	}
	if resp.Perf.Automation.ActionCount == 0 {
		t.Fatalf("expected automation perf entries, got %+v", resp.Perf)
	}
}

func TestAutomationPressKeyDispatchesShortcutAndMnemonic(t *testing.T) {
	shortcutCalls := 0
	mnemonicCalls := 0
	button := components.NewButton("publish", "Publish", func(*ApplicationState) { mnemonicCalls++ })
	button.Mnemonic = 'P'
	globalState = &ApplicationState{CurrentPage: PageDashboard, Pages: map[string][]Component{PageDashboard: {button}}}
	if err := globalState.RegisterShortcut("Ctrl+Shift+Q", func(*ApplicationState) { shortcutCalls++ }); err != nil {
		t.Fatal(err)
	}
	if err := automationPressKey("shift+control+q"); err != nil || shortcutCalls != 1 {
		t.Fatalf("shortcut err=%v calls=%d", err, shortcutCalls)
	}
	if err := automationPressKey("Alt+P"); err != nil || mnemonicCalls != 1 || globalState.FocusedID != "publish" {
		t.Fatalf("mnemonic err=%v calls=%d focus=%q", err, mnemonicCalls, globalState.FocusedID)
	}
}

func TestSemanticAutomationSelectorTargetsUniqueNode(t *testing.T) {
	clicked := ""
	publish := components.NewButton("publish", "Publish", func(*ApplicationState) { clicked = "publish" })
	publish.SetBounds(image.Rect(0, 0, 100, 40))
	publish.Selected = true
	cancel := components.NewButton("cancel", "Cancel", func(*ApplicationState) { clicked = "cancel" })
	globalState = &ApplicationState{CurrentPage: PageDashboard, Pages: map[string][]Component{PageDashboard: {publish, cancel}}}

	resp := handleAutomationRequest(AutomationRequest{Command: "click", Selector: &AutomationSelector{
		Role: "button", Name: "publish", States: map[string]bool{"enabled": true, "selected": true},
	}}, AutomationConfig{})
	if !resp.OK || clicked != "publish" || globalState.FocusedID != "publish" {
		t.Fatalf("response=%+v clicked=%q focus=%q", resp, clicked, globalState.FocusedID)
	}
}

func TestSemanticAutomationSelectorRejectsAmbiguityAndInvalidInput(t *testing.T) {
	globalState = &ApplicationState{CurrentPage: PageDashboard, Pages: map[string][]Component{PageDashboard: {
		components.NewButton("save-one", "Save", nil), components.NewButton("save-two", "Save", nil),
	}}}
	for _, req := range []AutomationRequest{
		{Command: "click", Selector: &AutomationSelector{Role: "button", Name: "Save"}},
		{Command: "click", Selector: &AutomationSelector{Role: "button", States: map[string]bool{"sparkling": true}}},
		{Command: "click", ID: "save-one", Selector: &AutomationSelector{Name: "Save"}},
		{Command: "click", Selector: &AutomationSelector{}},
	} {
		if resp := handleAutomationRequest(req, AutomationConfig{}); resp.OK || resp.Error == "" {
			t.Fatalf("invalid selector accepted: req=%+v response=%+v", req, resp)
		}
	}
}

func TestSemanticAutomationSelectorFocusesDescendant(t *testing.T) {
	tabs := components.NewTabs("tabs", []components.TabItem{{ID: "first", Label: "First"}, {ID: "second", Label: "Second"}}, "first", nil)
	globalState = &ApplicationState{CurrentPage: PageDashboard, Pages: map[string][]Component{PageDashboard: {tabs}}}
	resp := handleAutomationRequest(AutomationRequest{Command: "focus", Selector: &AutomationSelector{Role: "tab", Name: "Second"}}, AutomationConfig{})
	second, found := types.BuildSemanticsTree(globalState).Find("tabs/second")
	if !resp.OK || globalState.FocusedID != "tabs" || !found || !second.State.Focused {
		t.Fatalf("response=%+v focus=%q semantic=%+v found=%v", resp, globalState.FocusedID, second, found)
	}
}

func TestAutomationHTTPSelectorAndRequestLimit(t *testing.T) {
	clicked := false
	publish := components.NewButton("publish", "Publish", func(*ApplicationState) { clicked = true })
	publish.SetBounds(image.Rect(0, 0, 100, 40))
	globalState = &ApplicationState{CurrentPage: PageDashboard, Pages: map[string][]Component{PageDashboard: {publish}}}
	handler := newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{}))
	req := httptest.NewRequest(http.MethodPost, "/click", bytes.NewBufferString(`{"selector":{"role":"button","name":"Publish","states":{"enabled":true}}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !clicked {
		t.Fatalf("selector status=%d clicked=%v body=%s", rec.Code, clicked, rec.Body.String())
	}
	var selectorResp AutomationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &selectorResp); err != nil || selectorResp.TargetID != "publish" {
		t.Fatalf("selector response=%+v err=%v", selectorResp, err)
	}

	large := bytes.NewBufferString(`{"id":"` + strings.Repeat("a", maxAutomationRequestBody+1) + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/click", large)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized request status=%d body=%s", rec.Code, rec.Body.String())
	}
}
