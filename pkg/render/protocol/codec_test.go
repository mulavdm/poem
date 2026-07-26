package protocol

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func TestFontAtlasUsesInitPayloadWithDedicatedMessageType(t *testing.T) {
	message := InitEngine{AtlasWidth: 2, AtlasHeight: 2, AtlasPixels: make([]byte, 16), Chars: []CharInfo{{R: '日', Advance: 14}}}
	initial, err := EncodeInitEngine(message)
	if err != nil {
		t.Fatal(err)
	}
	update, err := EncodeFontAtlas(message)
	if err != nil {
		t.Fatal(err)
	}
	initialType, initialBody, err := DecodeEnvelope(initial)
	if err != nil {
		t.Fatal(err)
	}
	updateType, updateBody, err := DecodeEnvelope(update)
	if err != nil {
		t.Fatal(err)
	}
	if initialType != MessageInitEngine || updateType != MessageFontAtlas || !bytes.Equal(initialBody, updateBody) {
		t.Fatalf("unexpected atlas envelope types %v/%v or mismatched bodies", initialType, updateType)
	}
}

func TestRenderFrameRoundTripWithImageBytes(t *testing.T) {
	encoded, err := EncodeRenderFrame(RenderFrame{
		Width:  800,
		Height: 600,
		Cursor: 2,
		Commands: []DrawCommand{
			{
				Type:  DrawCommandTypeDrawImage,
				X1:    10,
				Y1:    20,
				X2:    110,
				Y2:    220,
				W:     32,
				H:     16,
				Bytes: []byte{1, 2, 3, 4},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	frame, err := DecodeRenderFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Commands) != 1 {
		t.Fatalf("expected one command, got %d", len(frame.Commands))
	}
	if frame.Commands[0].Type != DrawCommandTypeDrawImage {
		t.Fatalf("unexpected command type: %v", frame.Commands[0].Type)
	}
	if string(frame.Commands[0].Bytes) != string([]byte{1, 2, 3, 4}) {
		t.Fatalf("unexpected image bytes: %v", frame.Commands[0].Bytes)
	}
}

func TestRenderFrameRoundTripWithMapScenePlacement(t *testing.T) {
	want := DrawCommand{Type: DrawCommandTypeDrawMapScene, X1: 4, Y1: 8, X2: 404, Y2: 308, W: 256, H: 128, Text: "main-map", Bytes: []byte{9, 8, 7}}
	encoded, err := EncodeRenderFrame(RenderFrame{Width: 800, Height: 600, Commands: []DrawCommand{want}})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := DecodeRenderFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Commands) != 1 || !reflect.DeepEqual(frame.Commands[0], want) {
		t.Fatalf("map command = %+v, want %+v", frame.Commands, want)
	}
}

func TestRenderFrameRoundTripPreservesUnicodeText(t *testing.T) {
	want := "POEM 日本語 ✓"
	encoded, err := EncodeRenderFrame(RenderFrame{Width: 320, Height: 200, Commands: []DrawCommand{{Type: DrawCommandTypeDrawText, Text: want}}})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := DecodeRenderFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Commands) != 1 || frame.Commands[0].Text != want {
		t.Fatalf("unicode text = %q", frame.Commands[0].Text)
	}
}

func TestCompositionEventRoundTrip(t *testing.T) {
	want := EventBatch{Events: []Event{{Type: EventTypeCompositionStart}, {Type: EventTypeCompositionUpdate, Text: "にほ"}, {Type: EventTypeCompositionEnd, Text: "日本"}}}
	encoded, err := EncodeEventBatch(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeEventBatch(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composition events=%#v want=%#v", got, want)
	}
}

func TestGestureEventRoundTrip(t *testing.T) {
	want := EventBatch{Events: []Event{
		{Type: EventTypePanGesture, X: 10, Y: 20, DeltaX: -3, DeltaY: 7, Phase: GesturePhaseUpdate},
		{Type: EventTypePinchGesture, X: 30, Y: 40, DeltaX: 2, DeltaY: -1, Scale: 1.25, Phase: GesturePhaseEnd},
	}}
	encoded, err := EncodeEventBatch(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeEventBatch(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gesture events=%#v want=%#v", got, want)
	}
}

func TestSemanticActionEventRoundTrip(t *testing.T) {
	want := EventBatch{Events: []Event{{Type: EventTypeSemanticAction, Target: "editor/title", Action: "set-value", Value: "日本語"}}}
	encoded, err := EncodeEventBatch(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeEventBatch(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("semantic action events=%#v want=%#v", got, want)
	}
}

func TestRealtimeViewportEventRoundTrip(t *testing.T) {
	want := EventBatch{Events: []Event{{Type: EventTypeRealtimeViewport, Target: "scene_viewport", Bytes: []byte{0, 1, 2, 255}}}}
	encoded, err := EncodeEventBatch(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeEventBatch(encoded)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("realtime event=%#v err=%v", got, err)
	}
	oversized := EventBatch{Events: []Event{{Type: EventTypeRealtimeViewport, Bytes: make([]byte, (1<<20)+1)}}}
	if _, err := EncodeEventBatch(oversized); err == nil {
		t.Fatal("oversized realtime payload accepted")
	}
}

func TestCompositionEventRejectsOversizedText(t *testing.T) {
	_, err := EncodeEventBatch(EventBatch{Events: []Event{{Type: EventTypeCompositionUpdate, Text: strings.Repeat("x", (1<<20)+1)}}})
	if err == nil {
		t.Fatal("oversized composition event was accepted")
	}
}

func TestDecodeRejectsUnboundedEventAndCommandCounts(t *testing.T) {
	eventPayload := wrapEnvelope(MessageEventBatch, []byte{0xff, 0xff, 0xff, 0xff})
	if _, err := DecodeEventBatch(eventPayload); err == nil {
		t.Fatal("oversized event count was accepted")
	}
	commandPayload := wrapEnvelope(MessageRenderFrame, append([]byte{
		0, 0, 0, 0, // width
		0, 0, 0, 0, // height
		0, // cursor
	}, []byte{0xff, 0xff, 0xff, 0xff}...))
	if _, err := DecodeRenderFrame(commandPayload); err == nil {
		t.Fatal("oversized draw command count was accepted")
	}
}

func TestNativeDebugRoundTrip(t *testing.T) {
	reqPayload, err := EncodeNativeDebugRequest(NativeDebugRequest{
		CaptureFrame:          true,
		CapturePresentedFrame: true,
		CaptureDesktopFrame:   true,
		RestoreWindow:         true,
		ClampToWorkArea:       true,
		BringToForeground:     true,
		MaximizeWindow:        true,
		ResetPerf:             true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := DecodeNativeDebugRequest(reqPayload)
	if err != nil {
		t.Fatal(err)
	}
	if !req.CaptureFrame || !req.CapturePresentedFrame || !req.CaptureDesktopFrame || !req.RestoreWindow || !req.ClampToWorkArea || !req.BringToForeground || !req.MaximizeWindow || !req.ResetPerf {
		t.Fatalf("expected both native capture flags: %+v", req)
	}

	respPayload, err := EncodeNativeDebugResponse(NativeDebugResponse{
		DPI:              144,
		WindowVisible:    true,
		WindowMinimized:  false,
		WindowForeground: true,
		WindowLeft:       10,
		WindowTop:        20,
		WindowRight:      1010,
		WindowBottom:     620,
		ClientWidth:      900,
		ClientHeight:     500,
		BackbufferWidth:  1800,
		BackbufferHeight: 1000,
		FrameWidth:       2,
		FrameHeight:      1,
		FrameRGBA:        []byte{255, 0, 0, 255, 0, 255, 0, 255},
		PerfPhases: []NativePerfPhase{
			{Name: "apply_map_scene", Count: 5, MeanMS: 1.5, P50MS: 1.25, P95MS: 3, P99MS: 3.5, MaxMS: 4},
			{Name: "present", Count: 120, MeanMS: 2.5, P50MS: 2, P95MS: 4.17, P99MS: 6, MaxMS: 9.5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := DecodeNativeDebugResponse(respPayload)
	if err != nil {
		t.Fatal(err)
	}
	if resp.DPI != 144 || !resp.WindowVisible || !resp.WindowForeground || resp.ClientWidth != 900 || len(resp.FrameRGBA) != 8 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.PerfPhases) != 2 {
		t.Fatalf("expected 2 native perf phases, got %d", len(resp.PerfPhases))
	}
	// Phase order is part of the contract: the host sorts channels by name so
	// a consumer can diff two snapshots positionally.
	if resp.PerfPhases[0].Name != "apply_map_scene" || resp.PerfPhases[1].Name != "present" {
		t.Fatalf("phase order not preserved: %+v", resp.PerfPhases)
	}
	present := resp.PerfPhases[1]
	if present.Count != 120 || present.P95MS != 4.17 || present.MaxMS != 9.5 {
		t.Fatalf("native perf phase lost precision: %+v", present)
	}
}

// A malformed phase count must be rejected rather than drive a huge allocation.
func TestNativeDebugResponseRejectsOversizedPhaseCount(t *testing.T) {
	payload, err := EncodeNativeDebugResponse(NativeDebugResponse{})
	if err != nil {
		t.Fatal(err)
	}
	// The phase count is the final uint32 of the body.
	payload[len(payload)-4] = 0xFF
	payload[len(payload)-3] = 0xFF
	payload[len(payload)-2] = 0xFF
	payload[len(payload)-1] = 0x7F
	if _, err := DecodeNativeDebugResponse(payload); err == nil {
		t.Fatal("expected an error for an oversized native perf phase count")
	}
}

func TestNativeDialogRoundTrip(t *testing.T) {
	reqPayload, err := EncodeNativeDialogRequest(NativeDialogRequest{
		Kind:       "directory",
		Title:      "Choose Workspace",
		InitialDir: `D:\Projects`,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := DecodeNativeDialogRequest(reqPayload)
	if err != nil {
		t.Fatal(err)
	}
	if req.Kind != "directory" || req.Title != "Choose Workspace" || req.InitialDir != `D:\Projects` {
		t.Fatalf("unexpected dialog request: %+v", req)
	}

	respPayload, err := EncodeNativeDialogResponse(NativeDialogResponse{
		Path: `D:\Projects\GenEngine`,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := DecodeNativeDialogResponse(respPayload)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Canceled || resp.Path != `D:\Projects\GenEngine` || resp.Error != "" {
		t.Fatalf("unexpected dialog response: %+v", resp)
	}
}

func TestSemanticTreeRoundTrip(t *testing.T) {
	want := SemanticTree{Revision: 7, Nodes: []SemanticNode{
		{Parent: -1, ID: "root", Role: "application", Name: "Demo"},
		{Parent: 0, ID: "save", Role: "button", Name: "Save", AccessKey: "Alt+S", X1: 10, Y1: 20, X2: 90, Y2: 56, State: 2, LabeledBy: []string{"save-label"}, DescribedBy: []string{"save-help"}, Controls: []string{"notes"}, FlowsTo: []string{"zoom"}, Actions: []string{"focus", "invoke"}},
		{Parent: 0, ID: "zoom", Role: "slider", Name: "Zoom", Value: "125", HasRange: true, RangeMin: 25, RangeMax: 400, SmallChange: 5, LargeChange: 25, Actions: []string{"set-value"}},
		{Parent: 0, ID: "notes", Role: "text-field", Name: "Notes", Value: "日本語", HasText: true, SelectionStart: 1, SelectionEnd: 3, Multiline: true, Actions: []string{"set-selection"}},
		{Parent: 0, ID: "table", Role: "table", HasCollection: true, SelectionRequired: false, HasGrid: true, GridRows: 2, GridColumns: 3},
		{Parent: 4, ID: "table/r1/c2", Role: "cell", HasGridItem: true, GridRow: 0, GridColumn: 1, GridRowSpan: 1, GridColumnSpan: 1},
		{Parent: 0, ID: "scroll", Role: "group", HasScroll: true, HScrollPercent: -1, VScrollable: true, VScrollPercent: 40, HViewSize: 100, VViewSize: 25},
	}}
	payload, err := EncodeSemanticTree(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSemanticTree(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("semantic tree mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestSemanticRelationshipLimits(t *testing.T) {
	tooMany := make([]string, 257)
	if _, err := EncodeSemanticTree(SemanticTree{Nodes: []SemanticNode{{ID: "field", LabeledBy: tooMany}}}); err == nil {
		t.Fatal("oversized relationship list was encoded")
	}

	const target = "relationship-target-unique"
	payload, err := EncodeSemanticTree(SemanticTree{Nodes: []SemanticNode{{ID: "field", LabeledBy: []string{target}}}})
	if err != nil {
		t.Fatal(err)
	}
	targetOffset := bytes.Index(payload, []byte(target))
	if targetOffset < 8 {
		t.Fatal("relationship target not found in encoded payload")
	}
	// The relationship count precedes the target string's own length prefix.
	binary.LittleEndian.PutUint32(payload[targetOffset-8:targetOffset-4], 257)
	if _, err := DecodeSemanticTree(payload); err == nil || !strings.Contains(err.Error(), "relationship count") {
		t.Fatalf("malformed relationship count error=%v", err)
	}
}
