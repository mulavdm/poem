package protocol

import "testing"

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

func TestNativeDebugRoundTrip(t *testing.T) {
	reqPayload, err := EncodeNativeDebugRequest(NativeDebugRequest{
		CaptureFrame:          true,
		CapturePresentedFrame: true,
		CaptureDesktopFrame:   true,
		RestoreWindow:         true,
		ClampToWorkArea:       true,
		BringToForeground:     true,
		MaximizeWindow:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := DecodeNativeDebugRequest(reqPayload)
	if err != nil {
		t.Fatal(err)
	}
	if !req.CaptureFrame || !req.CapturePresentedFrame || !req.CaptureDesktopFrame || !req.RestoreWindow || !req.ClampToWorkArea || !req.BringToForeground || !req.MaximizeWindow {
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
