package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/mulavdm/poem/pkg/render/components"
	"github.com/mulavdm/poem/pkg/render/events"
	"github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/protocol"
	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

var nativeDebugRequest = requestNativeDebugWithOptions

const maxAutomationRequestBody = 1 << 20

type AutomationConfig struct {
	Enabled    bool
	Mode       string
	Host       string
	Port       int
	PipeName   string
	CaptureDir string
	Verbose    bool
}

type AutomationNode struct {
	ID             string           `json:"id"`
	Type           string           `json:"type"`
	Bounds         AutomationBounds `json:"bounds"`
	Text           string           `json:"text,omitempty"`
	Visible        bool             `json:"visible"`
	Focusable      bool             `json:"focusable"`
	Focused        bool             `json:"focused"`
	Role           string           `json:"role,omitempty"`
	Name           string           `json:"name,omitempty"`
	Value          string           `json:"value,omitempty"`
	Description    string           `json:"description,omitempty"`
	Disabled       bool             `json:"disabled,omitempty"`
	Selected       bool             `json:"selected,omitempty"`
	Checked        bool             `json:"checked,omitempty"`
	Expanded       bool             `json:"expanded,omitempty"`
	ReadOnly       bool             `json:"read_only,omitempty"`
	Invalid        bool             `json:"invalid,omitempty"`
	Actions        []string         `json:"actions,omitempty"`
	SelectionStart int              `json:"selection_start"`
	SelectionEnd   int              `json:"selection_end"`
	Children       []AutomationNode `json:"children,omitempty"`
}

type AutomationBounds struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type AutomationRequest struct {
	Command  string              `json:"command"`
	ID       string              `json:"id,omitempty"`
	Selector *AutomationSelector `json:"selector,omitempty"`
	Value    string              `json:"value,omitempty"`
	Path     string              `json:"path,omitempty"`
	Key      string              `json:"key,omitempty"`
	Start    int                 `json:"start,omitempty"`
	End      int                 `json:"end,omitempty"`
	Delta    int                 `json:"delta,omitempty"`
	DeltaX   int                 `json:"delta_x,omitempty"`
	DeltaY   int                 `json:"delta_y,omitempty"`
	Scale    float64             `json:"scale,omitempty"`
	Button   int                 `json:"button,omitempty"`
	Phase    string              `json:"phase,omitempty"`
}

// AutomationSelector addresses exactly one node in the platform-neutral
// semantic tree. State names are validated and support both true and false so
// callers can distinguish enabled from disabled controls.
type AutomationSelector struct {
	Role   string          `json:"role,omitempty"`
	Name   string          `json:"name,omitempty"`
	States map[string]bool `json:"states,omitempty"`
}

type AutomationResponse struct {
	OK                   bool             `json:"ok"`
	Error                string           `json:"error,omitempty"`
	CurrentPage          string           `json:"current_page,omitempty"`
	FocusedID            string           `json:"focused_id,omitempty"`
	HoveredID            string           `json:"hovered_id,omitempty"`
	TargetID             string           `json:"target_id,omitempty"`
	WindowWidth          int              `json:"window_width,omitempty"`
	WindowHeight         int              `json:"window_height,omitempty"`
	PhysicalWindowWidth  int              `json:"physical_window_width,omitempty"`
	PhysicalWindowHeight int              `json:"physical_window_height,omitempty"`
	Nodes                []AutomationNode `json:"nodes,omitempty"`
	Flat                 []AutomationNode `json:"flat,omitempty"`
	CapturePath          string           `json:"capture_path,omitempty"`
}

type NativeAutomationState struct {
	DPI int32 `json:"dpi"`

	WindowVisible    bool `json:"window_visible"`
	WindowMinimized  bool `json:"window_minimized"`
	WindowForeground bool `json:"window_foreground"`

	WindowLeft   int32 `json:"window_left"`
	WindowTop    int32 `json:"window_top"`
	WindowRight  int32 `json:"window_right"`
	WindowBottom int32 `json:"window_bottom"`

	ClientWidth  int32 `json:"client_width"`
	ClientHeight int32 `json:"client_height"`

	WorkLeft   int32 `json:"work_left"`
	WorkTop    int32 `json:"work_top"`
	WorkRight  int32 `json:"work_right"`
	WorkBottom int32 `json:"work_bottom"`

	BackbufferWidth  int32 `json:"backbuffer_width"`
	BackbufferHeight int32 `json:"backbuffer_height"`

	FrameWidth  int32 `json:"frame_width,omitempty"`
	FrameHeight int32 `json:"frame_height,omitempty"`
}

type NativeWindowControlRequest struct {
	RestoreWindow     bool `json:"restore_window"`
	ClampToWorkArea   bool `json:"clamp_to_work_area"`
	BringToForeground bool `json:"bring_to_foreground"`
	MaximizeWindow    bool `json:"maximize_window"`
}

// ResizeWindowRequest resizes the app window's client area (not the outer
// window rect) to Width x Height. Both must be positive.
type ResizeWindowRequest struct {
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}

type InspectFrameRequest struct {
	PrepareWindow     bool `json:"prepare_window"`
	RestoreWindow     bool `json:"restore_window"`
	ClampToWorkArea   bool `json:"clamp_to_work_area"`
	BringToForeground bool `json:"bring_to_foreground"`
	PreferDesktop     bool `json:"prefer_desktop"`
	FallbackToSelf    bool `json:"fallback_to_self"`
}

type PerfCondition struct {
	FocusedID           string `json:"focused_id,omitempty"`
	CurrentPage         string `json:"current_page,omitempty"`
	ComponentID         string `json:"component_id,omitempty"`
	ContainsText        string `json:"contains_text,omitempty"`
	WindowWidthAtLeast  int    `json:"window_width_at_least,omitempty"`
	WindowHeightAtLeast int    `json:"window_height_at_least,omitempty"`
	MinFrameCountDelta  uint64 `json:"min_frame_count_delta,omitempty"`
}

type PerfMeasureActionRequest struct {
	Command        string              `json:"command"`
	ID             string              `json:"id,omitempty"`
	Value          string              `json:"value,omitempty"`
	Path           string              `json:"path,omitempty"`
	Key            string              `json:"key,omitempty"`
	Selector       *AutomationSelector `json:"selector,omitempty"`
	TimeoutMS      int                 `json:"timeout_ms,omitempty"`
	PollIntervalMS int                 `json:"poll_interval_ms,omitempty"`
	Condition      PerfCondition       `json:"condition"`
}

type PerfMeasureActionResponse struct {
	OK           bool               `json:"ok"`
	Error        string             `json:"error,omitempty"`
	Command      string             `json:"command"`
	ElapsedMS    float64            `json:"elapsed_ms"`
	ConditionMet bool               `json:"condition_met"`
	FrameDelta   uint64             `json:"frame_delta"`
	Perf         PerfState          `json:"perf"`
	FinalState   AutomationResponse `json:"final_state"`
}

type NativePerfCondition struct {
	WindowVisible           bool   `json:"window_visible,omitempty"`
	WindowForeground        bool   `json:"window_foreground,omitempty"`
	WindowNotMinimized      bool   `json:"window_not_minimized,omitempty"`
	ClientWidthAtLeast      int    `json:"client_width_at_least,omitempty"`
	ClientHeightAtLeast     int    `json:"client_height_at_least,omitempty"`
	BackbufferWidthAtLeast  int    `json:"backbuffer_width_at_least,omitempty"`
	BackbufferHeightAtLeast int    `json:"backbuffer_height_at_least,omitempty"`
	WindowWidthAtLeast      int    `json:"window_width_at_least,omitempty"`
	WindowHeightAtLeast     int    `json:"window_height_at_least,omitempty"`
	MinFrameCountDelta      uint64 `json:"min_frame_count_delta,omitempty"`
	MinEventBatchCountDelta uint64 `json:"min_event_batch_count_delta,omitempty"`
}

type PerfMeasureNativeActionRequest struct {
	RestoreWindow     bool                `json:"restore_window"`
	ClampToWorkArea   bool                `json:"clamp_to_work_area"`
	BringToForeground bool                `json:"bring_to_foreground"`
	MaximizeWindow    bool                `json:"maximize_window"`
	TimeoutMS         int                 `json:"timeout_ms,omitempty"`
	PollIntervalMS    int                 `json:"poll_interval_ms,omitempty"`
	Condition         NativePerfCondition `json:"condition"`
}

type PerfMeasureNativeActionResponse struct {
	OK           bool                  `json:"ok"`
	Error        string                `json:"error,omitempty"`
	ElapsedMS    float64               `json:"elapsed_ms"`
	ConditionMet bool                  `json:"condition_met"`
	FrameDelta   uint64                `json:"frame_delta"`
	EventDelta   uint64                `json:"event_delta"`
	Perf         PerfState             `json:"perf"`
	NativeState  NativeAutomationState `json:"native_state"`
}

func resolveAutomationConfig(cfg *AutomationConfig) AutomationConfig {
	resolved := *cfg
	if strings.TrimSpace(resolved.Mode) == "" {
		resolved.Mode = "http"
	}
	if strings.TrimSpace(resolved.Host) == "" {
		resolved.Host = "127.0.0.1"
	}
	if resolved.Port == 0 {
		resolved.Port = 47831
	}
	if strings.TrimSpace(resolved.PipeName) == "" {
		resolved.PipeName = `\\.\pipe\poem_automation`
	}
	if strings.TrimSpace(resolved.CaptureDir) == "" {
		resolved.CaptureDir = "."
	}
	return resolved
}

func startAutomationServer(cfg AutomationConfig) {
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "pipe":
		startPipeAutomationServer(cfg)
	default:
		startHTTPAutomationServer(cfg)
	}
}

func startHTTPAutomationServer(cfg AutomationConfig) {
	go func() {
		addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
		server := &http.Server{
			Addr:    addr,
			Handler: newAutomationHTTPHandler(cfg),
		}
		if cfg.Verbose {
			fmt.Printf("POEM automation HTTP listening on http://%s\n", addr)
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("POEM automation HTTP server failed: %v\n", err)
		}
	}()
}

func handleAutomationConnection(conn io.ReadWriteCloser, cfg AutomationConfig) {
	defer conn.Close()

	payload, err := readMessage(conn)
	if err != nil {
		return
	}

	var req AutomationRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		_ = writeAutomationResponse(conn, AutomationResponse{OK: false, Error: err.Error()})
		return
	}

	resp, frame := handleAutomationRequest(req, cfg)
	flushAutomationFrame(frame)
	_ = writeAutomationResponse(conn, resp)
}

func writeAutomationResponse(conn io.Writer, resp AutomationResponse) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return writeMessage(conn, payload)
}

func newAutomationHTTPHandler(cfg AutomationConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		writeHTTPPerfJSON(w, http.StatusOK, healthSnapshot())
	})
	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, _ := handleAutomationRequest(AutomationRequest{Command: "get-state"}, cfg)
		writeHTTPAutomationJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, _ := handleAutomationRequest(AutomationRequest{Command: "list-components"}, cfg)
		writeHTTPAutomationJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/click", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "click")
	})
	mux.HandleFunc("/focus", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "focus")
	})
	mux.HandleFunc("/set-text", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "set-text")
	})
	mux.HandleFunc("/select-text", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "select-text")
	})
	mux.HandleFunc("/composition-start", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "composition-start")
	})
	mux.HandleFunc("/composition-update", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "composition-update")
	})
	mux.HandleFunc("/composition-end", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "composition-end")
	})
	mux.HandleFunc("/press-key", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "press-key")
	})
	mux.HandleFunc("/wheel", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "wheel")
	})
	mux.HandleFunc("/pan", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "pan")
	})
	mux.HandleFunc("/pinch", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "pinch")
	})
	mux.HandleFunc("/pointer-drag", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "pointer-drag")
	})
	mux.HandleFunc("/capture-frame", func(w http.ResponseWriter, r *http.Request) {
		writeAutomationHTTPCommand(w, r, cfg, "capture-frame")
	})
	mux.HandleFunc("/frame", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		bytes, err := captureCurrentFrameBytes()
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes)
	})
	mux.HandleFunc("/native-state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		writeHTTPNativeStateJSON(w, http.StatusOK, nativeAutomationStateFromProtocol(resp))
	})
	mux.HandleFunc("/native-frame", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{CaptureFrame: true})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		// The host reports why a capture could not be produced (an unsupported
		// presenter, a lost surface); without this the caller sees only a
		// generic encode failure for an empty buffer.
		if resp.Error != "" {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: resp.Error})
			return
		}
		pngBytes, err := encodeRGBAToPNG(resp.FrameRGBA, int(resp.FrameWidth), int(resp.FrameHeight))
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBytes)
	})
	mux.HandleFunc("/self-frame", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{CaptureFrame: true})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		// The host reports why a capture could not be produced (an unsupported
		// presenter, a lost surface); without this the caller sees only a
		// generic encode failure for an empty buffer.
		if resp.Error != "" {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: resp.Error})
			return
		}
		pngBytes, err := encodeRGBAToPNG(resp.FrameRGBA, int(resp.FrameWidth), int(resp.FrameHeight))
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBytes)
	})
	mux.HandleFunc("/window-frame", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{CapturePresentedFrame: true})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		// The host reports why a capture could not be produced (an unsupported
		// presenter, a lost surface); without this the caller sees only a
		// generic encode failure for an empty buffer.
		if resp.Error != "" {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: resp.Error})
			return
		}
		pngBytes, err := encodeRGBAToPNG(resp.FrameRGBA, int(resp.FrameWidth), int(resp.FrameHeight))
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBytes)
	})
	mux.HandleFunc("/desktop-frame", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{CaptureDesktopFrame: true})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		// The host reports why a capture could not be produced (an unsupported
		// presenter, a lost surface); without this the caller sees only a
		// generic encode failure for an empty buffer.
		if resp.Error != "" {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: resp.Error})
			return
		}
		pngBytes, err := encodeRGBAToPNG(resp.FrameRGBA, int(resp.FrameWidth), int(resp.FrameHeight))
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBytes)
	})
	mux.HandleFunc("/prepare-window", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		defer r.Body.Close()

		req := NativeWindowControlRequest{
			RestoreWindow:     true,
			ClampToWorkArea:   true,
			BringToForeground: true,
			MaximizeWindow:    false,
		}
		if r.ContentLength != 0 {
			if err := decodeAutomationJSON(w, r, &req); err != nil {
				writeHTTPAutomationJSON(w, http.StatusBadRequest, AutomationResponse{OK: false, Error: err.Error()})
				return
			}
		}

		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{
			RestoreWindow:     req.RestoreWindow,
			ClampToWorkArea:   req.ClampToWorkArea,
			BringToForeground: req.BringToForeground,
			MaximizeWindow:    req.MaximizeWindow,
		})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		writeHTTPNativeStateJSON(w, http.StatusOK, nativeAutomationStateFromProtocol(resp))
	})
	mux.HandleFunc("/resize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		defer r.Body.Close()

		var req ResizeWindowRequest
		if err := decodeAutomationJSON(w, r, &req); err != nil {
			writeHTTPAutomationJSON(w, http.StatusBadRequest, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		if req.Width <= 0 || req.Height <= 0 {
			writeHTTPAutomationJSON(w, http.StatusBadRequest, AutomationResponse{OK: false, Error: "width and height must both be positive"})
			return
		}

		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{
			ResizeWidth:  req.Width,
			ResizeHeight: req.Height,
		})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		writeHTTPNativeStateJSON(w, http.StatusOK, nativeAutomationStateFromProtocol(resp))
	})
	mux.HandleFunc("/inspect-frame", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		defer r.Body.Close()

		req := InspectFrameRequest{
			PrepareWindow:     true,
			RestoreWindow:     true,
			ClampToWorkArea:   true,
			BringToForeground: true,
			PreferDesktop:     true,
			FallbackToSelf:    true,
		}
		if r.ContentLength != 0 {
			if err := decodeAutomationJSON(w, r, &req); err != nil {
				writeHTTPAutomationJSON(w, http.StatusBadRequest, AutomationResponse{OK: false, Error: err.Error()})
				return
			}
		}

		pngBytes, source, state, err := inspectFramePNG(req)
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-POEM-Frame-Source", source)
		w.Header().Set("X-POEM-Window-Visible", strconv.FormatBool(state.WindowVisible))
		w.Header().Set("X-POEM-Window-Minimized", strconv.FormatBool(state.WindowMinimized))
		w.Header().Set("X-POEM-Window-Foreground", strconv.FormatBool(state.WindowForeground))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBytes)
	})
	mux.HandleFunc("/perf/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		writeHTTPPerfJSON(w, http.StatusOK, globalPerfTracker.snapshot())
	})
	mux.HandleFunc("/perf/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		writeHTTPPerfJSON(w, http.StatusOK, map[string]any{"events": globalPerfTracker.snapshot().Events})
	})
	mux.HandleFunc("/perf/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		globalPerfTracker.reset()
		// Clear the host's rings in the same call: a caller scoping a
		// measurement to one driven scene means both sides of the boundary,
		// and leaving native samples behind would blend two scenes. Hosts
		// without a native channel (web, test drivers) simply have nothing to
		// clear, so a failure here is not an error.
		_, _ = nativeDebugRequest(protocol.NativeDebugRequest{ResetPerf: true})
		writeHTTPPerfJSON(w, http.StatusOK, globalPerfTracker.snapshot())
	})
	mux.HandleFunc("/perf/native", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{})
		if err != nil {
			writeHTTPAutomationJSON(w, http.StatusInternalServerError, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		writeHTTPPerfJSON(w, http.StatusOK, nativePerfStateFromProtocol(resp))
	})
	mux.HandleFunc("/perf/measure-action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		defer r.Body.Close()

		var req PerfMeasureActionRequest
		if err := decodeAutomationJSON(w, r, &req); err != nil {
			writeHTTPAutomationJSON(w, http.StatusBadRequest, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		resp := measureAutomationAction(req, cfg)
		status := http.StatusOK
		if !resp.OK {
			status = http.StatusBadRequest
		}
		writeHTTPPerfJSON(w, status, resp)
	})
	mux.HandleFunc("/perf/measure-native-action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTPAutomationMethodNotAllowed(w)
			return
		}
		defer r.Body.Close()

		var req PerfMeasureNativeActionRequest
		if err := decodeAutomationJSON(w, r, &req); err != nil {
			writeHTTPAutomationJSON(w, http.StatusBadRequest, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
		resp := measureNativeAction(req)
		status := http.StatusOK
		if !resp.OK {
			status = http.StatusBadRequest
		}
		writeHTTPPerfJSON(w, status, resp)
	})
	return mux
}

func inspectFramePNG(req InspectFrameRequest) ([]byte, string, NativeAutomationState, error) {
	stateReq := protocol.NativeDebugRequest{}
	if req.PrepareWindow {
		stateReq.RestoreWindow = req.RestoreWindow
		stateReq.ClampToWorkArea = req.ClampToWorkArea
		stateReq.BringToForeground = req.BringToForeground
	}

	stateResp, err := nativeDebugRequest(stateReq)
	if err != nil {
		return nil, "", NativeAutomationState{}, err
	}
	state := nativeAutomationStateFromProtocol(stateResp)

	if req.PreferDesktop && state.WindowVisible && !state.WindowMinimized && state.WindowForeground {
		// Re-apply foreground preparation in the same native request that captures
		// the desktop. A separate HTTP/tool process can otherwise regain focus in
		// the gap after the readiness request, producing a valid PNG of an
		// occluding window instead of the application being inspected.
		captureReq := protocol.NativeDebugRequest{CaptureDesktopFrame: true}
		if req.PrepareWindow {
			captureReq.RestoreWindow = req.RestoreWindow
			captureReq.ClampToWorkArea = req.ClampToWorkArea
			captureReq.BringToForeground = req.BringToForeground
		}
		resp, err := nativeDebugRequest(captureReq)
		if err == nil {
			pngBytes, encodeErr := encodeRGBAToPNG(resp.FrameRGBA, int(resp.FrameWidth), int(resp.FrameHeight))
			if encodeErr == nil {
				return pngBytes, "desktop", nativeAutomationStateFromProtocol(resp), nil
			}
		}
	}

	if req.FallbackToSelf {
		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{CaptureFrame: true})
		if err != nil {
			return nil, "", state, err
		}
		pngBytes, err := encodeRGBAToPNG(resp.FrameRGBA, int(resp.FrameWidth), int(resp.FrameHeight))
		if err != nil {
			return nil, "", state, err
		}
		return pngBytes, "self", state, nil
	}

	if req.PreferDesktop && (!state.WindowVisible || state.WindowMinimized || !state.WindowForeground) {
		return nil, "", state, fmt.Errorf("window not ready for desktop capture: visible=%t minimized=%t foreground=%t", state.WindowVisible, state.WindowMinimized, state.WindowForeground)
	}

	return nil, "", state, fmt.Errorf("no inspection frame strategy succeeded")
}

func writeAutomationHTTPCommand(w http.ResponseWriter, r *http.Request, cfg AutomationConfig, command string) {
	if r.Method != http.MethodPost {
		writeHTTPAutomationMethodNotAllowed(w)
		return
	}
	defer r.Body.Close()

	var req AutomationRequest
	if r.ContentLength != 0 {
		if err := decodeAutomationJSON(w, r, &req); err != nil {
			writeHTTPAutomationJSON(w, http.StatusBadRequest, AutomationResponse{OK: false, Error: err.Error()})
			return
		}
	}
	req.Command = command
	resp := executeAutomationRequest(req, cfg)
	status := http.StatusOK
	if !resp.OK {
		status = http.StatusBadRequest
	}
	writeHTTPAutomationJSON(w, status, resp)
}

func decodeAutomationJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAutomationRequestBody)
	return json.NewDecoder(r.Body).Decode(target)
}

func writeHTTPAutomationMethodNotAllowed(w http.ResponseWriter) {
	writeHTTPAutomationJSON(w, http.StatusMethodNotAllowed, AutomationResponse{OK: false, Error: "method not allowed"})
}

func writeHTTPAutomationJSON(w http.ResponseWriter, status int, resp AutomationResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeHTTPNativeStateJSON(w http.ResponseWriter, status int, state NativeAutomationState) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(state)
}

func writeHTTPPerfJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func executeAutomationRequest(req AutomationRequest, cfg AutomationConfig) AutomationResponse {
	start := time.Now()
	resp, frame := handleAutomationRequest(req, cfg)
	flushAutomationFrame(frame)
	if strings.TrimSpace(req.Command) != "" {
		globalPerfTracker.recordAutomationAction(strings.ToLower(strings.TrimSpace(req.Command)), time.Since(start), automationActionDetails(req, resp))
	}
	return resp
}

func automationActionDetails(req AutomationRequest, resp AutomationResponse) string {
	details := []string{}
	if req.ID != "" {
		details = append(details, "id="+req.ID)
	}
	if req.Key != "" {
		details = append(details, "key="+req.Key)
	}
	if req.Selector != nil {
		details = append(details, "selector="+automationSelectorSummary(*req.Selector))
	}
	if !resp.OK && resp.Error != "" {
		details = append(details, "error="+resp.Error)
	}
	return strings.Join(details, " ")
}

func measureAutomationAction(req PerfMeasureActionRequest, cfg AutomationConfig) PerfMeasureActionResponse {
	timeout := 5 * time.Second
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	pollInterval := 15 * time.Millisecond
	if req.PollIntervalMS > 0 {
		pollInterval = time.Duration(req.PollIntervalMS) * time.Millisecond
	}

	startPerf := globalPerfTracker.snapshot()
	startFrameCount := startPerf.Frames.FrameCount
	start := time.Now()
	actionResp := executeAutomationRequest(AutomationRequest{
		Command:  req.Command,
		ID:       req.ID,
		Selector: req.Selector,
		Value:    req.Value,
		Path:     req.Path,
		Key:      req.Key,
	}, cfg)
	if !actionResp.OK {
		return PerfMeasureActionResponse{
			OK:           false,
			Error:        actionResp.Error,
			Command:      req.Command,
			ElapsedMS:    durationMS(time.Since(start)),
			ConditionMet: false,
			FrameDelta:   globalPerfTracker.snapshot().Frames.FrameCount - startFrameCount,
			Perf:         globalPerfTracker.snapshot(),
			FinalState:   actionResp,
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		met := false
		finalState := AutomationResponse{}
		if tryLockStateMutexWithTimeout(automationLockTimeout) {
			met = perfConditionMetLocked(req.Condition, startFrameCount)
			finalState = baseAutomationResponse(nil, nil)
			stateMutex.Unlock()
		}
		if met {
			elapsed := time.Since(start)
			globalPerfTracker.recordAutomationAction("measure-"+strings.ToLower(strings.TrimSpace(req.Command)), elapsed, "condition_met=true")
			return PerfMeasureActionResponse{
				OK:           true,
				Command:      req.Command,
				ElapsedMS:    durationMS(elapsed),
				ConditionMet: true,
				FrameDelta:   globalPerfTracker.snapshot().Frames.FrameCount - startFrameCount,
				Perf:         globalPerfTracker.snapshot(),
				FinalState:   finalState,
			}
		}
		if time.Now().After(deadline) {
			elapsed := time.Since(start)
			globalPerfTracker.recordAutomationAction("measure-"+strings.ToLower(strings.TrimSpace(req.Command)), elapsed, "condition_met=false")
			return PerfMeasureActionResponse{
				OK:           false,
				Error:        "timed out waiting for condition",
				Command:      req.Command,
				ElapsedMS:    durationMS(elapsed),
				ConditionMet: false,
				FrameDelta:   globalPerfTracker.snapshot().Frames.FrameCount - startFrameCount,
				Perf:         globalPerfTracker.snapshot(),
				FinalState:   actionResp,
			}
		}
		time.Sleep(pollInterval)
	}
}

func measureNativeAction(req PerfMeasureNativeActionRequest) PerfMeasureNativeActionResponse {
	timeout := 5 * time.Second
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	pollInterval := 15 * time.Millisecond
	if req.PollIntervalMS > 0 {
		pollInterval = time.Duration(req.PollIntervalMS) * time.Millisecond
	}

	startPerf := globalPerfTracker.snapshot()
	startFrameCount := startPerf.Frames.FrameCount
	startEventBatchCount := startPerf.EventBatches.BatchCount
	start := time.Now()

	initialResp, err := nativeDebugRequest(protocol.NativeDebugRequest{
		RestoreWindow:     req.RestoreWindow,
		ClampToWorkArea:   req.ClampToWorkArea,
		BringToForeground: req.BringToForeground,
		MaximizeWindow:    req.MaximizeWindow,
	})
	if err != nil {
		globalPerfTracker.recordAutomationAction("native-action", time.Since(start), "error="+err.Error())
		return PerfMeasureNativeActionResponse{
			OK:          false,
			Error:       err.Error(),
			ElapsedMS:   durationMS(time.Since(start)),
			FrameDelta:  globalPerfTracker.snapshot().Frames.FrameCount - startFrameCount,
			EventDelta:  globalPerfTracker.snapshot().EventBatches.BatchCount - startEventBatchCount,
			Perf:        globalPerfTracker.snapshot(),
			NativeState: NativeAutomationState{},
		}
	}
	globalPerfTracker.recordAutomationAction("native-action", time.Since(start), nativeActionDetails(req))

	initialState := nativeAutomationStateFromProtocol(initialResp)
	if nativePerfConditionMet(req.Condition, initialState, startFrameCount, startEventBatchCount) {
		elapsed := time.Since(start)
		globalPerfTracker.recordAutomationAction("measure-native-action", elapsed, "condition_met=true")
		return PerfMeasureNativeActionResponse{
			OK:           true,
			ElapsedMS:    durationMS(elapsed),
			ConditionMet: true,
			FrameDelta:   globalPerfTracker.snapshot().Frames.FrameCount - startFrameCount,
			EventDelta:   globalPerfTracker.snapshot().EventBatches.BatchCount - startEventBatchCount,
			Perf:         globalPerfTracker.snapshot(),
			NativeState:  initialState,
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			elapsed := time.Since(start)
			globalPerfTracker.recordAutomationAction("measure-native-action", elapsed, "condition_met=false")
			return PerfMeasureNativeActionResponse{
				OK:           false,
				Error:        "timed out waiting for native condition",
				ElapsedMS:    durationMS(elapsed),
				ConditionMet: false,
				FrameDelta:   globalPerfTracker.snapshot().Frames.FrameCount - startFrameCount,
				EventDelta:   globalPerfTracker.snapshot().EventBatches.BatchCount - startEventBatchCount,
				Perf:         globalPerfTracker.snapshot(),
				NativeState:  initialState,
			}
		}

		time.Sleep(pollInterval)

		resp, err := nativeDebugRequest(protocol.NativeDebugRequest{})
		if err != nil {
			continue
		}
		state := nativeAutomationStateFromProtocol(resp)
		if nativePerfConditionMet(req.Condition, state, startFrameCount, startEventBatchCount) {
			elapsed := time.Since(start)
			globalPerfTracker.recordAutomationAction("measure-native-action", elapsed, "condition_met=true")
			return PerfMeasureNativeActionResponse{
				OK:           true,
				ElapsedMS:    durationMS(elapsed),
				ConditionMet: true,
				FrameDelta:   globalPerfTracker.snapshot().Frames.FrameCount - startFrameCount,
				EventDelta:   globalPerfTracker.snapshot().EventBatches.BatchCount - startEventBatchCount,
				Perf:         globalPerfTracker.snapshot(),
				NativeState:  state,
			}
		}
		initialState = state
	}
}

func nativeActionDetails(req PerfMeasureNativeActionRequest) string {
	details := []string{}
	if req.RestoreWindow {
		details = append(details, "restore=true")
	}
	if req.ClampToWorkArea {
		details = append(details, "clamp=true")
	}
	if req.BringToForeground {
		details = append(details, "foreground=true")
	}
	if req.MaximizeWindow {
		details = append(details, "maximize=true")
	}
	return strings.Join(details, " ")
}

func perfConditionMetLocked(cond PerfCondition, startFrameCount uint64) bool {
	if globalState == nil {
		return false
	}
	if cond.FocusedID != "" && globalState.FocusedID != cond.FocusedID {
		return false
	}
	if cond.CurrentPage != "" && globalState.CurrentPage != cond.CurrentPage {
		return false
	}
	if cond.WindowWidthAtLeast > 0 && globalState.WindowWidth < cond.WindowWidthAtLeast {
		return false
	}
	if cond.WindowHeightAtLeast > 0 && globalState.WindowHeight < cond.WindowHeightAtLeast {
		return false
	}
	if cond.ComponentID != "" {
		comp := libFindComponent(cond.ComponentID)
		if comp == nil {
			return false
		}
		if cond.ContainsText != "" && !strings.Contains(componentText(comp), cond.ContainsText) {
			return false
		}
	}
	if cond.MinFrameCountDelta > 0 {
		if globalPerfTracker.snapshot().Frames.FrameCount-startFrameCount < cond.MinFrameCountDelta {
			return false
		}
	}
	return true
}

func nativePerfConditionMet(cond NativePerfCondition, state NativeAutomationState, startFrameCount, startEventBatchCount uint64) bool {
	if cond.WindowVisible && !state.WindowVisible {
		return false
	}
	if cond.WindowForeground && !state.WindowForeground {
		return false
	}
	if cond.WindowNotMinimized && state.WindowMinimized {
		return false
	}
	if cond.ClientWidthAtLeast > 0 && int(state.ClientWidth) < cond.ClientWidthAtLeast {
		return false
	}
	if cond.ClientHeightAtLeast > 0 && int(state.ClientHeight) < cond.ClientHeightAtLeast {
		return false
	}
	if cond.BackbufferWidthAtLeast > 0 && int(state.BackbufferWidth) < cond.BackbufferWidthAtLeast {
		return false
	}
	if cond.BackbufferHeightAtLeast > 0 && int(state.BackbufferHeight) < cond.BackbufferHeightAtLeast {
		return false
	}
	if cond.WindowWidthAtLeast > 0 && int(state.WindowRight-state.WindowLeft) < cond.WindowWidthAtLeast {
		return false
	}
	if cond.WindowHeightAtLeast > 0 && int(state.WindowBottom-state.WindowTop) < cond.WindowHeightAtLeast {
		return false
	}
	snapshot := globalPerfTracker.snapshot()
	if cond.MinFrameCountDelta > 0 && snapshot.Frames.FrameCount-startFrameCount < cond.MinFrameCountDelta {
		return false
	}
	if cond.MinEventBatchCountDelta > 0 && snapshot.EventBatches.BatchCount-startEventBatchCount < cond.MinEventBatchCountDelta {
		return false
	}
	return true
}

// automationLockTimeout bounds how long an interactive automation command
// waits to acquire stateMutex before failing fast with a clear "app busy"
// error, instead of hanging silently behind whatever (possibly wedged) owner
// -- typically the render/frame loop in run.go, which holds the same lock --
// currently has it. Observed live: /click hung for minutes while /frame,
// which never takes this lock, kept answering and wrongly suggesting the
// app was healthy. See /health below for a way to tell the two apart.
// A var, not a const, so tests can shrink it rather than actually waiting.
var automationLockTimeout = 5 * time.Second

// tryLockStateMutexWithTimeout polls TryLock rather than blocking on Lock,
// trading a small amount of latency (poll interval) for a bounded, honest
// failure instead of an indefinite hang.
func tryLockStateMutexWithTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if stateMutex.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// HealthResponse is deliberately independent of stateMutex: it never blocks
// on it, only reports whether it is currently held. That is the signal /frame
// cannot give -- /frame reads the last-produced frame buffer through its own
// separate synchronization and keeps answering even while stateMutex (and
// therefore every interactive command) is stuck behind a wedged render loop.
type HealthResponse struct {
	OK          bool   `json:"ok"`
	StateLocked bool   `json:"state_locked"`
	FrameCount  uint64 `json:"frame_count"`
}

func healthSnapshot() HealthResponse {
	locked := !stateMutex.TryLock()
	if !locked {
		stateMutex.Unlock()
	}
	return HealthResponse{
		OK:          true,
		StateLocked: locked,
		FrameCount:  globalPerfTracker.snapshot().Frames.FrameCount,
	}
}

// handleAutomationRequest executes one automation command under stateMutex
// and returns, alongside the response, any render frame a repaint produced.
// The frame is only buffered here, never written to the presenter pipe --
// callers must flush it (flushAutomationFrame) only after this function has
// returned and therefore released stateMutex. See automationRepaintInto for
// why writing synchronously from inside the lock is the hazard this avoids.
func handleAutomationRequest(req AutomationRequest, cfg AutomationConfig) (AutomationResponse, []byte) {
	if !tryLockStateMutexWithTimeout(automationLockTimeout) {
		return AutomationResponse{OK: false, Error: fmt.Sprintf(
			"automation surface busy: could not acquire state lock within %s; the render loop may be stuck -- check GET /health", automationLockTimeout)}, nil
	}
	defer stateMutex.Unlock()

	if globalState == nil {
		return AutomationResponse{OK: false, Error: "POEM state not initialized"}, nil
	}

	var outbound bytes.Buffer
	repaint := func() { automationRepaintInto(&outbound) }

	switch strings.ToLower(strings.TrimSpace(req.Command)) {
	case "list-components":
		nodes, flat := buildAutomationSnapshot()
		return baseAutomationResponse(nodes, flat), nil
	case "get-state":
		return baseAutomationResponse(nil, nil), nil
	case "click":
		target, err := resolveAutomationTarget(req)
		if err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		if err := automationClickComponent(target); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		repaint()
		return automationTargetResponse(target), outbound.Bytes()
	case "focus":
		target, err := resolveAutomationTarget(req)
		if err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		if err := automationFocusComponent(target); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		repaint()
		return automationTargetResponse(target), outbound.Bytes()
	case "set-text":
		target, err := resolveAutomationTarget(req)
		if err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		if err := automationSetText(target, req.Value); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		repaint()
		return automationTargetResponse(target), outbound.Bytes()
	case "select-text":
		target, err := resolveAutomationTarget(req)
		if err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		if err := automationSelectText(target, req.Start, req.End); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		repaint()
		return automationTargetResponse(target), outbound.Bytes()
	case "composition-start", "composition-update", "composition-end":
		target, err := resolveAutomationTarget(req)
		if err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		if err := automationComposition(target, strings.TrimPrefix(strings.ToLower(strings.TrimSpace(req.Command)), "composition-"), req.Value); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		repaint()
		return automationTargetResponse(target), outbound.Bytes()
	case "press-key":
		if err := automationPressKey(req.Key); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		repaint()
		return baseAutomationResponse(nil, nil), outbound.Bytes()
	case "wheel", "pan", "pinch", "pointer-drag":
		target, err := resolveAutomationTarget(req)
		if err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		if err := automationViewportInput(target, req); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		repaint()
		return automationTargetResponse(target), outbound.Bytes()
	case "capture-frame":
		targetPath := req.Path
		if strings.TrimSpace(targetPath) == "" {
			targetPath = filepath.Join(cfg.CaptureDir, "poem_capture.png")
		}
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(cfg.CaptureDir, targetPath)
		}
		if err := captureCurrentFrame(targetPath); err != nil {
			return AutomationResponse{OK: false, Error: err.Error()}, nil
		}
		resp := baseAutomationResponse(nil, nil)
		resp.CapturePath = targetPath
		return resp, nil
	default:
		return AutomationResponse{OK: false, Error: "unknown automation command"}, nil
	}
}

func baseAutomationResponse(nodes, flat []AutomationNode) AutomationResponse {
	resp := AutomationResponse{
		OK:                   true,
		CurrentPage:          globalState.CurrentPage,
		FocusedID:            globalState.FocusedID,
		HoveredID:            globalState.HoveredID,
		WindowWidth:          globalState.WindowWidth,
		WindowHeight:         globalState.WindowHeight,
		PhysicalWindowWidth:  globalState.PhysicalWindowWidth,
		PhysicalWindowHeight: globalState.PhysicalWindowHeight,
	}
	if nodes != nil {
		resp.Nodes = nodes
	}
	if flat != nil {
		resp.Flat = flat
	}
	return resp
}

func automationTargetResponse(target string) AutomationResponse {
	resp := baseAutomationResponse(nil, nil)
	resp.TargetID = target
	return resp
}

func resolveAutomationTarget(req AutomationRequest) (string, error) {
	id := strings.TrimSpace(req.ID)
	if id != "" && req.Selector != nil {
		return "", fmt.Errorf("automation target must use either id or selector, not both")
	}
	if id != "" {
		if len(id) > 1024 {
			return "", fmt.Errorf("automation id exceeds 1024 bytes")
		}
		return id, nil
	}
	if req.Selector == nil {
		return "", fmt.Errorf("automation target requires id or selector")
	}
	selector := *req.Selector
	selector.Role = strings.ToLower(strings.TrimSpace(selector.Role))
	selector.Name = strings.TrimSpace(selector.Name)
	if len(selector.Role) > 128 || len(selector.Name) > 1024 {
		return "", fmt.Errorf("automation selector text exceeds limit")
	}
	if selector.Role == "" && selector.Name == "" && len(selector.States) == 0 {
		return "", fmt.Errorf("automation selector is empty")
	}
	if len(selector.States) > 16 {
		return "", fmt.Errorf("automation selector has too many states")
	}
	for name := range selector.States {
		if _, ok := semanticStateValue(semantics.State{}, name); !ok {
			return "", fmt.Errorf("unknown automation state %q", name)
		}
	}
	tree := types.BuildSemanticsTree(globalState)
	if err := tree.Validate(); err != nil {
		return "", fmt.Errorf("semantic tree is invalid: %w", err)
	}
	matches := make([]string, 0, 2)
	var visit func(semantics.Node)
	visit = func(node semantics.Node) {
		if semanticNodeMatchesSelector(node, selector) {
			matches = append(matches, node.ID)
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(tree.Root)
	if len(matches) == 0 {
		return "", fmt.Errorf("automation selector %s matched no semantic nodes", automationSelectorSummary(selector))
	}
	if len(matches) > 1 {
		shown := matches
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return "", fmt.Errorf("automation selector %s is ambiguous; matched %d nodes: %s", automationSelectorSummary(selector), len(matches), strings.Join(shown, ", "))
	}
	return matches[0], nil
}

func semanticNodeMatchesSelector(node semantics.Node, selector AutomationSelector) bool {
	if selector.Role != "" && strings.ToLower(string(node.Role)) != selector.Role {
		return false
	}
	if selector.Name != "" && !strings.EqualFold(node.Name, selector.Name) {
		return false
	}
	for name, want := range selector.States {
		got, _ := semanticStateValue(node.State, name)
		if got != want {
			return false
		}
	}
	return true
}

func semanticStateValue(state semantics.State, name string) (bool, bool) {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "-", "_")) {
	case "disabled":
		return state.Disabled, true
	case "enabled":
		return !state.Disabled, true
	case "focused":
		return state.Focused, true
	case "selected":
		return state.Selected, true
	case "checked":
		return state.Checked, true
	case "expanded":
		return state.Expanded, true
	case "read_only", "readonly":
		return state.ReadOnly, true
	case "required":
		return state.Required, true
	case "invalid":
		return state.Invalid, true
	case "password":
		return state.Password, true
	case "offscreen":
		return state.Offscreen, true
	default:
		return false, false
	}
}

func automationSelectorSummary(selector AutomationSelector) string {
	parts := make([]string, 0, 3)
	if role := strings.TrimSpace(selector.Role); role != "" {
		parts = append(parts, "role="+role)
	}
	if name := strings.TrimSpace(selector.Name); name != "" {
		parts = append(parts, "name="+strconv.Quote(name))
	}
	stateNames := make([]string, 0, len(selector.States))
	for name := range selector.States {
		stateNames = append(stateNames, name)
	}
	sort.Strings(stateNames)
	for _, name := range stateNames {
		parts = append(parts, fmt.Sprintf("%s=%t", name, selector.States[name]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func buildAutomationSnapshot() ([]AutomationNode, []AutomationNode) {
	nodes := make([]AutomationNode, 0)
	flat := make([]AutomationNode, 0)
	if globalState == nil {
		return nodes, flat
	}
	comps := globalState.Pages[globalState.CurrentPage]
	for _, comp := range comps {
		comp.SetBounds(comp.Bounds())
		node := buildAutomationNode(comp)
		nodes = append(nodes, node)
		flattenAutomationNode(node, &flat)
	}
	if globalState.Overlays != nil {
		for _, overlay := range globalState.Overlays.Snapshot() {
			node := buildAutomationNode(overlay.Component)
			nodes = append(nodes, node)
			flattenAutomationNode(node, &flat)
		}
	}
	return nodes, flat
}

func buildAutomationNode(comp types.Component) AutomationNode {
	bounds := comp.Bounds()
	node := AutomationNode{
		ID:        comp.ID(),
		Type:      componentTypeName(comp),
		Bounds:    AutomationBounds{X: bounds.Min.X, Y: bounds.Min.Y, W: bounds.Dx(), H: bounds.Dy()},
		Text:      componentText(comp),
		Visible:   true,
		Focusable: comp.Focusable(),
		Focused:   globalState != nil && globalState.FocusedID == comp.ID(),
	}
	if semantic, ok := comp.(types.SemanticComponent); ok {
		info := semantic.Semantics(globalState)
		node.Role, node.Name, node.Value, node.Description = string(info.Role), info.Name, info.Value, info.Description
		node.Disabled, node.Selected, node.Checked = info.State.Disabled, info.State.Selected, info.State.Checked
		node.Expanded, node.ReadOnly, node.Invalid = info.State.Expanded, info.State.ReadOnly, info.State.Invalid
		if info.Text != nil {
			node.SelectionStart, node.SelectionEnd = info.Text.SelectionStart, info.Text.SelectionEnd
		}
		for _, action := range info.Actions {
			node.Actions = append(node.Actions, string(action))
		}
		for _, child := range info.Children {
			node.Children = append(node.Children, automationNodeFromSemantic(child))
		}
	}
	if container, ok := comp.(types.ChildComponent); ok {
		for _, child := range container.ChildComponents() {
			if child != nil {
				node.Children = append(node.Children, buildAutomationNode(child))
			}
		}
	}
	return node
}

func automationNodeFromSemantic(info semantics.Node) AutomationNode {
	node := AutomationNode{ID: info.ID, Type: "SemanticNode", Role: string(info.Role), Name: info.Name, Value: info.Value, Description: info.Description,
		Bounds: AutomationBounds{X: info.Bounds.Min.X, Y: info.Bounds.Min.Y, W: info.Bounds.Dx(), H: info.Bounds.Dy()}, Visible: true,
		Focused: info.State.Focused, Disabled: info.State.Disabled, Selected: info.State.Selected, Checked: info.State.Checked,
		Expanded: info.State.Expanded, ReadOnly: info.State.ReadOnly, Invalid: info.State.Invalid}
	if info.Text != nil {
		node.SelectionStart, node.SelectionEnd = info.Text.SelectionStart, info.Text.SelectionEnd
	}
	for _, action := range info.Actions {
		node.Actions = append(node.Actions, string(action))
		if action == semantics.ActionFocus {
			node.Focusable = true
		}
	}
	for _, child := range info.Children {
		node.Children = append(node.Children, automationNodeFromSemantic(child))
	}
	return node
}

func flattenAutomationNode(node AutomationNode, flat *[]AutomationNode) {
	*flat = append(*flat, node)
	for _, child := range node.Children {
		flattenAutomationNode(child, flat)
	}
}

func componentTypeName(comp types.Component) string {
	switch comp.(type) {
	case *components.Button:
		return "Button"
	case *components.TextInput:
		return "TextInput"
	case *components.TextArea:
		return "TextArea"
	case *components.Label:
		return "Label"
	case *components.DynamicLabel:
		return "DynamicLabel"
	case *components.Panel:
		return "Panel"
	case *components.GlassPanel:
		return "GlassPanel"
	case *components.ScrollView:
		return "ScrollView"
	case *components.Paragraph:
		return "Paragraph"
	case *components.ImageView:
		return "ImageView"
	case *components.Checkbox:
		return "Checkbox"
	case *components.Switch:
		return "Switch"
	case *components.Radio:
		return "Radio"
	case *components.Slider:
		return "Slider"
	case *components.ProgressBar:
		return "ProgressBar"
	case *components.Tabs:
		return "Tabs"
	case *components.Select:
		return "Select"
	case *components.Badge:
		return "Badge"
	case *components.VirtualList:
		return "VirtualList"
	case *components.DataTable:
		return "DataTable"
	case *components.Spinner:
		return "Spinner"
	case *components.Skeleton:
		return "Skeleton"
	case *components.Tooltip:
		return "Tooltip"
	case *components.Toast:
		return "Toast"
	case *components.Toolbar:
		return "Toolbar"
	case *components.Menu:
		return "Menu"
	case *components.Popover:
		return "Popover"
	case *layout.FlexBox:
		return "FlexBox"
	default:
		return fmt.Sprintf("%T", comp)
	}
}

func componentText(comp types.Component) string {
	switch c := comp.(type) {
	case *components.Button:
		return c.Text
	case *components.Label:
		return c.Text
	case *components.TextInput:
		if c.Masked {
			return ""
		}
		if globalState != nil && globalState.TextInputValues != nil {
			if val, ok := globalState.TextInputValues[c.CompID]; ok {
				return val
			}
		}
		return c.Value
	case *components.TextArea:
		if globalState != nil && globalState.TextInputValues != nil {
			if val, ok := globalState.TextInputValues[c.CompID]; ok {
				return val
			}
		}
		return c.Value
	case *components.Paragraph:
		return c.Text
	default:
		return ""
	}
}

func automationClickComponent(id string) error {
	globalState.DismissFocusLossOverlaysForTarget(id)
	comp := libFindComponent(id)
	if comp == nil {
		action := semantics.ActionInvoke
		if node, ok := types.BuildSemanticsTree(globalState).Find(id); ok {
			for _, advertised := range node.Actions {
				if advertised == semantics.ActionInvoke {
					action = semantics.ActionInvoke
					break
				}
				if advertised == semantics.ActionSelect {
					action = semantics.ActionSelect
				} else if advertised == semantics.ActionExpand && action == semantics.ActionInvoke {
					action = semantics.ActionExpand
				} else if advertised == semantics.ActionCollapse && action == semantics.ActionInvoke {
					action = semantics.ActionCollapse
				}
			}
		}
		for _, root := range interactionRoots() {
			handled := false
			root.Walk(func(candidate types.Component) {
				if handled {
					return
				}
				if semantic, ok := candidate.(types.SemanticActionComponent); ok {
					handled = semantic.PerformSemanticAction(id, action, "", globalState)
				}
			})
			if handled {
				return nil
			}
		}
		return fmt.Errorf("component or semantic target %q not found", id)
	}
	bounds := comp.Bounds()
	pt := image.Pt(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+bounds.Dy()/2)
	globalState.DismissFocusLossOverlaysForPointer(id, pt)
	globalState.MouseX = pt.X
	globalState.MouseY = pt.Y
	globalState.HoveredID = id
	globalState.FocusedID = id
	globalState.ActiveID = id
	_ = comp.OnMouseDown(pt, globalState)
	_ = comp.OnMouseUp(pt, globalState)
	globalState.ActiveID = ""
	return nil
}

// automationViewportInput deliberately enters through the same screen-space
// event path as a presenter. Calling a viewport method directly would make the
// automation blind to routing bugs in scroll containers and overlays—the exact
// class of framework defect these actions are intended to diagnose.
func automationViewportInput(id string, req AutomationRequest) error {
	point, err := automationInputPoint(id)
	if err != nil {
		return err
	}
	end := point.Add(image.Pt(req.DeltaX, req.DeltaY))
	phase, phased, err := automationGesturePhase(req.Phase)
	if err != nil {
		return err
	}
	var eventsToSend []protocol.Event
	switch strings.ToLower(strings.TrimSpace(req.Command)) {
	case "wheel":
		if req.Delta == 0 {
			return fmt.Errorf("wheel needs a non-zero delta")
		}
		eventsToSend = []protocol.Event{{Type: protocol.EventTypeMouseWheel, X: int32(point.X), Y: int32(point.Y), Delta: int32(req.Delta)}}
	case "pan":
		if (!phased || phase == protocol.GesturePhaseUpdate) && req.DeltaX == 0 && req.DeltaY == 0 {
			return fmt.Errorf("pan needs a non-zero delta_x or delta_y")
		}
		if phased {
			eventsToSend = []protocol.Event{{Type: protocol.EventTypePanGesture, X: int32(end.X), Y: int32(end.Y),
				DeltaX: int32(req.DeltaX), DeltaY: int32(req.DeltaY), Scale: 1, Phase: phase}}
		} else {
			eventsToSend = []protocol.Event{
				{Type: protocol.EventTypePanGesture, X: int32(point.X), Y: int32(point.Y), Scale: 1, Phase: protocol.GesturePhaseBegin},
				{Type: protocol.EventTypePanGesture, X: int32(end.X), Y: int32(end.Y), DeltaX: int32(req.DeltaX), DeltaY: int32(req.DeltaY), Scale: 1, Phase: protocol.GesturePhaseUpdate},
				{Type: protocol.EventTypePanGesture, X: int32(end.X), Y: int32(end.Y), Scale: 1, Phase: protocol.GesturePhaseEnd},
			}
		}
	case "pinch":
		if req.Scale == 0 && phased && phase != protocol.GesturePhaseUpdate {
			req.Scale = 1
		}
		if req.Scale <= 0 || math.IsNaN(req.Scale) || math.IsInf(req.Scale, 0) {
			return fmt.Errorf("pinch needs a finite positive scale")
		}
		if phased {
			eventsToSend = []protocol.Event{{Type: protocol.EventTypePinchGesture, X: int32(end.X), Y: int32(end.Y),
				DeltaX: int32(req.DeltaX), DeltaY: int32(req.DeltaY), Scale: float32(req.Scale), Phase: phase}}
		} else {
			eventsToSend = []protocol.Event{
				{Type: protocol.EventTypePinchGesture, X: int32(point.X), Y: int32(point.Y), Scale: 1, Phase: protocol.GesturePhaseBegin},
				{Type: protocol.EventTypePinchGesture, X: int32(end.X), Y: int32(end.Y), DeltaX: int32(req.DeltaX), DeltaY: int32(req.DeltaY), Scale: float32(req.Scale), Phase: protocol.GesturePhaseUpdate},
				{Type: protocol.EventTypePinchGesture, X: int32(end.X), Y: int32(end.Y), Scale: 1, Phase: protocol.GesturePhaseEnd},
			}
		}
	case "pointer-drag":
		button := req.Button
		if button == 0 {
			button = 1
		}
		if button < 1 || button > 3 {
			return fmt.Errorf("pointer-drag button must be 1, 2, or 3")
		}
		if (!phased || phase == protocol.GesturePhaseUpdate) && req.DeltaX == 0 && req.DeltaY == 0 {
			return fmt.Errorf("pointer-drag needs a non-zero delta_x or delta_y")
		}
		if phased {
			switch phase {
			case protocol.GesturePhaseBegin:
				eventsToSend = []protocol.Event{
					{Type: protocol.EventTypeMouseMove, X: int32(point.X), Y: int32(point.Y)},
					{Type: protocol.EventTypeMouseDown, X: int32(point.X), Y: int32(point.Y), Button: int32(button)},
				}
			case protocol.GesturePhaseUpdate:
				eventsToSend = []protocol.Event{{Type: protocol.EventTypeMouseMove, X: int32(end.X), Y: int32(end.Y)}}
			case protocol.GesturePhaseEnd, protocol.GesturePhaseCancel:
				eventsToSend = []protocol.Event{{Type: protocol.EventTypeMouseUp, X: int32(end.X), Y: int32(end.Y), Button: int32(button)}}
			}
		} else {
			eventsToSend = []protocol.Event{
				{Type: protocol.EventTypeMouseMove, X: int32(point.X), Y: int32(point.Y)},
				{Type: protocol.EventTypeMouseDown, X: int32(point.X), Y: int32(point.Y), Button: int32(button)},
				{Type: protocol.EventTypeMouseMove, X: int32(end.X), Y: int32(end.Y)},
				{Type: protocol.EventTypeMouseUp, X: int32(end.X), Y: int32(end.Y), Button: int32(button)},
			}
		}
	default:
		return fmt.Errorf("unsupported viewport input %q", req.Command)
	}
	processEventBatch(protocol.EventBatch{Events: eventsToSend}, io.Discard, nil)
	return nil
}

func automationGesturePhase(raw string) (protocol.GesturePhase, bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return protocol.GesturePhaseBegin, false, nil
	case "begin":
		return protocol.GesturePhaseBegin, true, nil
	case "update":
		return protocol.GesturePhaseUpdate, true, nil
	case "end":
		return protocol.GesturePhaseEnd, true, nil
	case "cancel":
		return protocol.GesturePhaseCancel, true, nil
	default:
		return protocol.GesturePhaseCancel, false, fmt.Errorf("unknown gesture phase %q; want begin, update, end, or cancel", raw)
	}
}

func automationInputPoint(id string) (image.Point, error) {
	component := libFindComponent(id)
	if component == nil {
		return image.Point{}, fmt.Errorf("component %q not found", id)
	}
	node, ok := types.BuildSemanticsTree(globalState).Find(id)
	if !ok || node.Bounds.Empty() || node.State.Offscreen {
		return image.Point{}, fmt.Errorf("component %q is not visible in the current viewport", id)
	}
	point := image.Pt(node.Bounds.Min.X+node.Bounds.Dx()/2, node.Bounds.Min.Y+node.Bounds.Dy()/2)
	route := topmostInputRoute(point)
	for _, entry := range route {
		if entry.component.ID() == id {
			return point, nil
		}
	}
	top := ""
	if len(route) > 0 {
		top = route[len(route)-1].component.ID()
	}
	return image.Point{}, fmt.Errorf("component %q is covered at %v (topmost target %q)", id, point, top)
}

func automationFocusComponent(id string) error {
	comp := libFindComponent(id)
	if comp == nil || comp.ID() != id {
		if node, ok := types.BuildSemanticsTree(globalState).Find(id); ok {
			for _, action := range node.Actions {
				if action == semantics.ActionFocus && performSemanticAction(id, semantics.ActionFocus, "") {
					return nil
				}
			}
			return fmt.Errorf("semantic target %q is not focusable", id)
		}
		if comp != nil {
			return fmt.Errorf("semantic target %q not found", id)
		}
		return fmt.Errorf("component or semantic target %q not found", id)
	}
	if !comp.Focusable() {
		return fmt.Errorf("component %q is not focusable", id)
	}
	globalState.DismissFocusLossOverlaysForTarget(id)
	globalState.FocusedID = id
	return nil
}

func automationSetText(id, value string) error {
	comp := libFindComponent(id)
	if comp == nil {
		return fmt.Errorf("component %q not found", id)
	}
	if globalState.TextInputValues == nil {
		globalState.TextInputValues = make(map[string]string)
	}
	switch c := comp.(type) {
	case *components.TextInput:
		c.Value = value
		c.CursorIndex = len([]rune(value))
		globalState.TextInputValues[id] = value
		globalState.TextInputValues[id+"_cursor"] = strconv.Itoa(c.CursorIndex)
		c.SetSelection(c.CursorIndex, c.CursorIndex, globalState)
		globalState.FocusedID = id
		return nil
	case *components.TextArea:
		c.Value = value
		c.CursorIndex = len([]rune(value))
		globalState.TextInputValues[id] = value
		globalState.TextInputValues[id+"_cursor"] = strconv.Itoa(c.CursorIndex)
		globalState.FocusedID = id
		return nil
	default:
		return fmt.Errorf("component %q is not a text input", id)
	}
}

func automationSelectText(id string, start, end int) error {
	comp := libFindComponent(id)
	input, ok := comp.(interface {
		SetSelection(int, int, *types.ApplicationState)
	})
	if !ok {
		return fmt.Errorf("component %q is not a text input", id)
	}
	length := len([]rune(componentText(comp)))
	if start < 0 || end < 0 || start > length || end > length {
		return fmt.Errorf("selection %d:%d is outside text length %d", start, end, length)
	}
	input.SetSelection(start, end, globalState)
	globalState.FocusedID = id
	return nil
}

func automationComposition(id, phase, value string) error {
	comp := libFindComponent(id)
	target, ok := comp.(types.TextCompositionComponent)
	if !ok {
		return fmt.Errorf("component %q does not support text composition", id)
	}
	globalState.FocusedID = id
	handled := false
	switch phase {
	case "start":
		handled = target.StartComposition(globalState)
	case "update":
		handled = target.UpdateComposition(value, globalState)
	case "end":
		handled = target.EndComposition(value, globalState)
	default:
		return fmt.Errorf("unsupported composition phase %q", phase)
	}
	if !handled {
		return fmt.Errorf("component %q rejected composition %s", id, phase)
	}
	return nil
}

func automationPressKey(key string) error {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return fmt.Errorf("missing key")
	}
	shortcut, shortcutErr := events.ParseShortcut(key)
	if shortcutErr == nil {
		if globalState.DispatchShortcut(shortcut.String()) {
			return nil
		}
		keyRunes := []rune(shortcut.Key)
		if shortcut.Modifiers.Alt && len(keyRunes) == 1 && activateMnemonic(keyRunes[0]) {
			return nil
		}
		if shortcut.Modifiers.Control || shortcut.Modifiers.Alt || shortcut.Modifiers.Meta {
			return fmt.Errorf("shortcut %q is not registered", shortcut.String())
		}
	}
	if normalized == "tab" || normalized == "shift+tab" {
		globalState.CycleFocus(normalized == "shift+tab")
		return nil
	}
	if globalState.FocusedID == "" {
		return fmt.Errorf("no focused component")
	}
	var code uint32
	var ch rune
	switch normalized {
	case "enter":
		code = 13
	case "space":
		code = 32
	case "backspace":
		code = 8
	case "escape", "esc":
		code = 0x1B
	case "left", "arrowleft":
		code = 0x25
	case "up", "arrowup":
		code = 0x26
	case "right", "arrowright":
		code = 0x27
	case "down", "arrowdown":
		code = 0x28
	case "home":
		code = 0x24
	case "end":
		code = 0x23
	case "pageup", "page up":
		code = 0x21
	case "pagedown", "page down":
		code = 0x22
	case "delete":
		code = 0x2E
	default:
		runes := []rune(key)
		if len(runes) != 1 {
			return fmt.Errorf("unsupported key %q", key)
		}
		ch = runes[0]
	}

	for _, comp := range interactionRoots() {
		if comp.OnKey(code, ch, globalState) {
			return nil
		}
	}
	return nil
}

// automationRepaintInto renders one frame into buf instead of writing it to
// the presenter pipe. handleAutomationRequest calls this while still holding
// stateMutex, and the presenter pipe write is a blocking, synchronous
// operation (see run.go's repaint loop) -- writing directly from here, like
// this used to, could stall behind a busy native host with the lock held,
// wedging every other stateMutex reader such as /components. Buffering here
// and flushing only after the lock is released (flushAutomationFrame) is the
// same fix the main repaint loop already applies to its own frame writes.
func automationRepaintInto(buf *bytes.Buffer) {
	if globalPainter == nil {
		return
	}
	triggerRepaintFrame(buf, globalPainter)
}

// flushAutomationFrame writes a repaint frame produced by an automation
// command to the presenter pipe. Callers must invoke this only after
// handleAutomationRequest has returned -- and therefore after stateMutex has
// been released -- never while still holding the lock.
func flushAutomationFrame(frame []byte) {
	if len(frame) == 0 || globalRenderConn == nil {
		return
	}
	flushBufferedFrames(globalRenderConn, frame)
}

func captureCurrentFrame(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	img, err := captureCurrentFrameBytes()
	if err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(img)
	return err
}

func captureCurrentFrameBytes() ([]byte, error) {
	img := renderFrameToImage(globalLastFrame)
	if globalState != nil && globalState.PhysicalWindowWidth > 0 && globalState.PhysicalWindowHeight > 0 {
		target := image.Rect(0, 0, globalState.PhysicalWindowWidth, globalState.PhysicalWindowHeight)
		if !target.Eq(img.Bounds()) {
			scaled := image.NewRGBA(target)
			xdraw.CatmullRom.Scale(scaled, target, img, img.Bounds(), xdraw.Over, nil)
			img = scaled
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeRGBAToPNG(pixels []byte, width, height int) ([]byte, error) {
	if len(pixels) == 0 || width <= 0 || height <= 0 {
		return nil, fmt.Errorf("missing native frame data")
	}
	img := &image.RGBA{
		Pix:    append([]byte(nil), pixels...),
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NativePerfState reports the host's own frame timings, which the Go tracker
// cannot see: everything in PerfState is measured on this side of the
// boundary, so without this the native presentation cost is unmeasured.
type NativePerfState struct {
	Phases []NativePerfPhaseState `json:"phases"`
}

// NativePerfPhaseState is one timed native channel. "present" is geometry
// compilation plus draw submission on the UI thread — the phase the migration
// plan budgets separately from Go frame work.
type NativePerfPhaseState struct {
	Name   string  `json:"name"`
	Count  uint32  `json:"count"`
	MeanMS float64 `json:"mean_ms"`
	P50MS  float64 `json:"p50_ms"`
	P95MS  float64 `json:"p95_ms"`
	P99MS  float64 `json:"p99_ms"`
	MaxMS  float64 `json:"max_ms"`
}

func nativePerfStateFromProtocol(resp protocol.NativeDebugResponse) NativePerfState {
	out := NativePerfState{Phases: make([]NativePerfPhaseState, 0, len(resp.PerfPhases))}
	for _, phase := range resp.PerfPhases {
		out.Phases = append(out.Phases, NativePerfPhaseState{
			Name:   phase.Name,
			Count:  phase.Count,
			MeanMS: phase.MeanMS,
			P50MS:  phase.P50MS,
			P95MS:  phase.P95MS,
			P99MS:  phase.P99MS,
			MaxMS:  phase.MaxMS,
		})
	}
	return out
}

func nativeAutomationStateFromProtocol(resp protocol.NativeDebugResponse) NativeAutomationState {
	return NativeAutomationState{
		DPI:              resp.DPI,
		WindowVisible:    resp.WindowVisible,
		WindowMinimized:  resp.WindowMinimized,
		WindowForeground: resp.WindowForeground,
		WindowLeft:       resp.WindowLeft,
		WindowTop:        resp.WindowTop,
		WindowRight:      resp.WindowRight,
		WindowBottom:     resp.WindowBottom,
		ClientWidth:      resp.ClientWidth,
		ClientHeight:     resp.ClientHeight,
		WorkLeft:         resp.WorkLeft,
		WorkTop:          resp.WorkTop,
		WorkRight:        resp.WorkRight,
		WorkBottom:       resp.WorkBottom,
		BackbufferWidth:  resp.BackbufferWidth,
		BackbufferHeight: resp.BackbufferHeight,
		FrameWidth:       resp.FrameWidth,
		FrameHeight:      resp.FrameHeight,
	}
}

func renderFrameToImage(frame protocol.RenderFrame) *image.RGBA {
	width := int(frame.Width)
	height := int(frame.Height)
	if width <= 0 {
		width = 1024
	}
	if height <= 0 {
		height = 768
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	currentClip := image.Rect(0, 0, width, height)
	clipEnabled := false
	offsetX := 0
	offsetY := 0
	for _, cmd := range frame.Commands {
		switch cmd.Type {
		case protocol.DrawCommandTypeSetOffset:
			offsetX = int(math.Round(float64(cmd.Val1)))
			offsetY = int(math.Round(float64(cmd.Val2)))
		case protocol.DrawCommandTypeSetClip:
			if cmd.Flag {
				currentClip = image.Rect(
					int(cmd.X1),
					int(cmd.Y1),
					int(cmd.X1+cmd.W),
					int(cmd.Y1+cmd.H),
				)
				clipEnabled = true
			} else {
				currentClip = image.Rect(0, 0, width, height)
				clipEnabled = false
			}
		case protocol.DrawCommandTypeFillRect, protocol.DrawCommandTypeDrawRoundedRect:
			fillRoundedRect(
				img,
				image.Rect(int(cmd.X1)+offsetX, int(cmd.Y1)+offsetY, int(cmd.X2)+offsetX, int(cmd.Y2)+offsetY),
				int(cmd.Radius),
				color.RGBA{cmd.R, cmd.G, cmd.B, cmd.A},
				clipEnabled,
				currentClip,
			)
		case protocol.DrawCommandTypeDrawLine:
			drawLine(img, int(cmd.X1)+offsetX, int(cmd.Y1)+offsetY, int(cmd.X2)+offsetX, int(cmd.Y2)+offsetY, color.RGBA{cmd.R, cmd.G, cmd.B, cmd.A}, clipEnabled, currentClip)
		case protocol.DrawCommandTypeDrawText:
			drawTextToImage(img, int(cmd.X1)+offsetX, int(cmd.Y1)+offsetY, cmd.Text, color.RGBA{cmd.R, cmd.G, cmd.B, cmd.A}, clipEnabled, currentClip)
		case protocol.DrawCommandTypeDrawImage:
			drawImageBytes(img, image.Rect(int(cmd.X1)+offsetX, int(cmd.Y1)+offsetY, int(cmd.X2)+offsetX, int(cmd.Y2)+offsetY), int(cmd.W), int(cmd.H), cmd.Bytes, clipEnabled, currentClip)
		}
	}
	return img
}

func fillRoundedRect(dst *image.RGBA, rect image.Rectangle, radius int, col color.RGBA, clipEnabled bool, clip image.Rectangle) {
	if radius < 0 {
		radius = 0
	}
	target := rect
	if clipEnabled {
		target = target.Intersect(clip)
	}
	if target.Empty() {
		return
	}
	for y := target.Min.Y; y < target.Max.Y; y++ {
		for x := target.Min.X; x < target.Max.X; x++ {
			if radius > 0 && !pointInRoundedRect(x, y, rect, radius) {
				continue
			}
			blendPixel(dst, x, y, col)
		}
	}
}

func pointInRoundedRect(x, y int, rect image.Rectangle, radius int) bool {
	if x >= rect.Min.X+radius && x < rect.Max.X-radius {
		return true
	}
	if y >= rect.Min.Y+radius && y < rect.Max.Y-radius {
		return true
	}
	corners := []image.Point{
		{rect.Min.X + radius, rect.Min.Y + radius},
		{rect.Max.X - radius - 1, rect.Min.Y + radius},
		{rect.Min.X + radius, rect.Max.Y - radius - 1},
		{rect.Max.X - radius - 1, rect.Max.Y - radius - 1},
	}
	for _, center := range corners {
		dx := float64(x - center.X)
		dy := float64(y - center.Y)
		if dx*dx+dy*dy <= float64(radius*radius) {
			return true
		}
	}
	return false
}

func drawLine(dst *image.RGBA, x1, y1, x2, y2 int, col color.RGBA, clipEnabled bool, clip image.Rectangle) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	steps := int(math.Max(math.Abs(dx), math.Abs(dy)))
	if steps == 0 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		x := x1 + int(float64(i)*dx/float64(steps))
		y := y1 + int(float64(i)*dy/float64(steps))
		if clipEnabled && !image.Pt(x, y).In(clip) {
			continue
		}
		blendPixel(dst, x, y, col)
	}
}

func drawTextToImage(dst *image.RGBA, x, y int, text string, col color.RGBA, clipEnabled bool, clip image.Rectangle) {
	if text == "" {
		return
	}
	tmp := image.NewRGBA(dst.Bounds())
	drawer := &font.Drawer{
		Dst:  tmp,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
	if clipEnabled {
		target := clip.Intersect(dst.Bounds())
		if target.Empty() {
			return
		}
		draw.Draw(dst, target, tmp, target.Min, draw.Over)
		return
	}
	draw.Draw(dst, dst.Bounds(), tmp, image.Point{}, draw.Over)
}

func drawImageBytes(dst *image.RGBA, rect image.Rectangle, srcW, srcH int, pixels []byte, clipEnabled bool, clip image.Rectangle) {
	if srcW <= 0 || srcH <= 0 || len(pixels) < srcW*srcH*4 {
		return
	}
	src := &image.RGBA{
		Pix:    append([]byte(nil), pixels...),
		Stride: srcW * 4,
		Rect:   image.Rect(0, 0, srcW, srcH),
	}
	if rect.Dx() == srcW && rect.Dy() == srcH {
		target := rect
		if clipEnabled {
			target = target.Intersect(clip)
		}
		draw.Draw(dst, target, src, image.Pt(target.Min.X-rect.Min.X, target.Min.Y-rect.Min.Y), draw.Over)
		return
	}
	scaled := image.NewRGBA(rect)
	xdraw.CatmullRom.Scale(scaled, rect, src, src.Bounds(), xdraw.Over, nil)
	target := rect
	if clipEnabled {
		target = target.Intersect(clip)
	}
	draw.Draw(dst, target, scaled, target.Min, draw.Over)
}

func blendPixel(img *image.RGBA, x, y int, src color.RGBA) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return
	}
	dst := img.RGBAAt(x, y)
	alpha := float64(src.A) / 255.0
	inv := 1.0 - alpha
	img.SetRGBA(x, y, color.RGBA{
		R: uint8(float64(src.R)*alpha + float64(dst.R)*inv),
		G: uint8(float64(src.G)*alpha + float64(dst.G)*inv),
		B: uint8(float64(src.B)*alpha + float64(dst.B)*inv),
		A: uint8(math.Min(255, float64(src.A)+float64(dst.A)*inv)),
	})
}
