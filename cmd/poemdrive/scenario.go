package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Scenario is a repeatable interaction over POEM's automation surface, plus
// the budgets its measurements must satisfy.
//
// The point of putting this in a file rather than a script is that the scene
// itself becomes a fixture. A performance gate that cannot replay the exact
// interaction it measured is comparing two different things and calling the
// difference a regression.
type Scenario struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// BaseURL is the automation surface. Windows and MAPPS bind loopback
	// directly; Android is reached through `adb forward` to the same port.
	BaseURL string `json:"base_url"`

	// Warmup runs the step list this many times before counters are reset, so
	// first-paint costs and lazily built caches do not land in the sample.
	Warmup int `json:"warmup"`

	// Iterations is how many steps execute after the reset. Each step is one
	// interaction, and most interactions produce one repaint.
	Iterations int `json:"iterations"`

	// SettleMS waits after the last step before sampling, for scenes whose
	// work continues past the interaction that triggered it.
	SettleMS int `json:"settle_ms,omitempty"`

	// PaceMS is the minimum interval between steps. Driving flat out submits
	// frames faster than the compositor retires them, so the renderer blocks on
	// swapchain back-pressure and the per-frame cost measures queue saturation
	// rather than framework work. Budgets are per-frame numbers, so a scene
	// that measures them has to produce frames at a sustainable cadence.
	// Defaults to 16 ms, one 60 Hz frame. Set 0 deliberately to measure
	// throughput under saturation instead.
	PaceMS *int `json:"pace_ms,omitempty"`

	// Launch, when set, brings the app up before driving it: booting a device,
	// installing, starting, and forwarding the port. Without it the scenario
	// assumes something else already did that.
	Launch *Launch `json:"launch,omitempty"`

	Steps []Step `json:"steps"`

	// Budgets maps `metric.statistic` to a millisecond ceiling, for example
	// "go.total.p95" or "native.present.p95". Metrics are `go.<phase>` from
	// /perf/state and `native.<phase>` from /perf/native.
	Budgets map[string]float64 `json:"budgets,omitempty"`

	// RegressionTolerancePct is how far p95/p99 may drift above a recorded
	// baseline before the run fails. Defaults to 15.
	RegressionTolerancePct float64 `json:"regression_tolerance_pct,omitempty"`

	// RegressionFloorMS is the absolute delta a regression must also exceed.
	// It exists because percentage drift is meaningless near the timer
	// resolution — Go-side timings on Windows quantize to roughly 0.5 ms, so a
	// p95 of 0.55 ms moving a single bucket reads as +35%. Defaults to 0.5.
	RegressionFloorMS float64 `json:"regression_floor_ms,omitempty"`

	// Functional marks a scenario that exists to prove behavior, not gate
	// frame timing: run() does not fail when the steps produce zero frames
	// (a pure assertion pass, or a click that legitimately repaints nothing),
	// and Budgets/regression checking is expected to stay unset rather than
	// skipped by special-casing — an empty Budgets map already checks
	// nothing.
	Functional bool `json:"functional,omitempty"`
}

// Step is one interaction. Exactly one action per step.
type Step struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty"`
	// Value is the literal text for set-text, the required substring for
	// assert-value, and the required substring for wait-for (omit it on
	// wait-for to poll for existence instead of a value).
	Value string `json:"value,omitempty"`
	Key   string `json:"key,omitempty"`
	// MS is the sleep duration for wait, and the poll timeout for wait-for
	// (defaults to waitForDefaultTimeout when zero).
	MS     int     `json:"ms,omitempty"`
	Delta  int     `json:"delta,omitempty"`
	DeltaX int     `json:"delta_x,omitempty"`
	DeltaY int     `json:"delta_y,omitempty"`
	Scale  float64 `json:"scale,omitempty"`
	Button int     `json:"button,omitempty"`
	Phase  string  `json:"phase,omitempty"`
	Width  int     `json:"width,omitempty"`
	Height int     `json:"height,omitempty"`
	// Dynamic marks a target that is expected to appear only after an earlier
	// step changes the component tree. It is skipped by the initial fail-fast
	// target scan and still validated by its action when that step executes.
	Dynamic bool `json:"dynamic,omitempty"`

	// Capture only. Name becomes the PNG filename; Source selects which image
	// the host should produce.
	Name   string `json:"name,omitempty"`
	Source string `json:"source,omitempty"`

	// assert-count only: components whose id has this prefix are counted and
	// compared against Count.
	IDPrefix string `json:"id_prefix,omitempty"`
	Count    int    `json:"count,omitempty"`

	// assert-no-overlap only: every one of these ids must be present, and no
	// two of their bounding rectangles may intersect.
	IDs []string `json:"ids,omitempty"`
}

const (
	actionClick           = "click"
	actionFocus           = "focus"
	actionSetText         = "set-text"
	actionPressKey        = "press-key"
	actionWait            = "wait"
	actionCapture         = "capture"
	actionWheel           = "wheel"
	actionPan             = "pan"
	actionPinch           = "pinch"
	actionDrag            = "pointer-drag"
	actionAssertValue     = "assert-value"
	actionResize          = "resize"
	actionAssertExists    = "assert-exists"
	actionAssertAbsent    = "assert-absent"
	actionAssertCount     = "assert-count"
	actionAssertNoOverlap = "assert-no-overlap"
	actionWaitFor         = "wait-for"
)

// waitForDefaultTimeout bounds a wait-for step when the scene does not set
// ms. Some interactions apply asynchronously, through a round trip a driven
// action's HTTP response does not wait for -- HamsterEditor's document
// command queue is one example, where a property commit only lands once the
// C++ authority has processed and echoed it back. A fixed sleep before
// asserting is a guess at that round trip's latency; wait-for polls the
// actual condition instead, so five seconds of headroom costs nothing when
// the condition is already met and only matters on the rare slow run.
const waitForDefaultTimeout = 5 * time.Second

// captureSources maps a scenario's source name to the automation endpoint that
// produces it. "native" is the host's own backbuffer — the pixels the presenter
// actually drew, on Windows and Android alike. "go" is the engine-side
// reference rasterization, which is what the two are compared against.
var captureSources = map[string]string{
	"native": "/native-frame",
	"go":     "/frame",
	"window": "/window-frame",
}

// Validate rejects a scenario before any app is driven. A scene that fails
// halfway through leaves the app in an unknown state and the numbers
// meaningless, so the cheap checks all happen first.
func (s *Scenario) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("scenario needs a name")
	}
	if strings.TrimSpace(s.BaseURL) == "" {
		return fmt.Errorf("scenario %q needs a base_url", s.Name)
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("scenario %q has no steps", s.Name)
	}
	if s.Iterations <= 0 {
		return fmt.Errorf("scenario %q needs a positive iterations", s.Name)
	}
	for i, step := range s.Steps {
		switch step.Action {
		case actionClick, actionFocus:
			if step.ID == "" {
				return fmt.Errorf("step %d (%s) needs an id", i, step.Action)
			}
		case actionSetText:
			if step.ID == "" {
				return fmt.Errorf("step %d (set-text) needs an id", i)
			}
		case actionPressKey:
			if step.Key == "" {
				return fmt.Errorf("step %d (press-key) needs a key", i)
			}
		case actionWheel:
			if step.ID == "" || step.Delta == 0 {
				return fmt.Errorf("step %d (wheel) needs an id and non-zero delta", i)
			}
		case actionPan, actionDrag:
			if step.ID == "" || (step.Phase == "" || step.Phase == "update") && step.DeltaX == 0 && step.DeltaY == 0 {
				return fmt.Errorf("step %d (%s) needs an id and non-zero delta_x or delta_y", i, step.Action)
			}
			if step.Action == actionDrag && (step.Button < 0 || step.Button > 3) {
				return fmt.Errorf("step %d (pointer-drag) button must be 1, 2, or 3", i)
			}
		case actionPinch:
			if step.ID == "" || (step.Phase == "" || step.Phase == "update") && step.Scale <= 0 {
				return fmt.Errorf("step %d (pinch) needs an id and positive scale", i)
			}
		case actionAssertValue:
			if step.ID == "" || step.Value == "" {
				return fmt.Errorf("step %d (assert-value) needs an id and value substring", i)
			}
		case actionResize:
			if step.Width <= 0 || step.Height <= 0 {
				return fmt.Errorf("step %d (resize) needs a positive width and height", i)
			}
		case actionAssertExists, actionAssertAbsent:
			if step.ID == "" {
				return fmt.Errorf("step %d (%s) needs an id", i, step.Action)
			}
		case actionAssertCount:
			if step.IDPrefix == "" {
				return fmt.Errorf("step %d (assert-count) needs an id_prefix", i)
			}
		case actionAssertNoOverlap:
			if len(step.IDs) < 2 {
				return fmt.Errorf("step %d (assert-no-overlap) needs at least two ids", i)
			}
		case actionWait:
			if step.MS <= 0 {
				return fmt.Errorf("step %d (wait) needs a positive ms", i)
			}
		case actionWaitFor:
			if step.ID == "" {
				return fmt.Errorf("step %d (wait-for) needs an id", i)
			}
			if step.MS < 0 {
				return fmt.Errorf("step %d (wait-for) ms must not be negative", i)
			}
		case actionCapture:
			if step.Name == "" {
				return fmt.Errorf("step %d (capture) needs a name; it becomes the PNG filename", i)
			}
			if step.Source != "" {
				if _, ok := captureSources[step.Source]; !ok {
					return fmt.Errorf("step %d (capture) has unknown source %q; want native, go or window", i, step.Source)
				}
			}
		case "":
			return fmt.Errorf("step %d has no action", i)
		default:
			return fmt.Errorf("step %d has unknown action %q", i, step.Action)
		}
		if step.Phase != "" && step.Phase != "begin" && step.Phase != "update" && step.Phase != "end" && step.Phase != "cancel" {
			return fmt.Errorf("step %d has unknown phase %q", i, step.Phase)
		}
	}
	if s.Launch != nil {
		if err := s.Launch.validate(); err != nil {
			return fmt.Errorf("scenario %q launch: %w", s.Name, err)
		}
	}
	for key := range s.Budgets {
		if _, _, err := splitBudgetKey(key); err != nil {
			return fmt.Errorf("budget %q: %w", key, err)
		}
	}
	return nil
}

// splitBudgetKey divides "go.total.p95" into metric "go.total" and statistic
// "p95".
func splitBudgetKey(key string) (metric, statistic string, err error) {
	idx := strings.LastIndex(key, ".")
	if idx <= 0 || idx == len(key)-1 {
		return "", "", fmt.Errorf("want `metric.statistic`, for example go.total.p95")
	}
	metric, statistic = key[:idx], key[idx+1:]
	switch statistic {
	case "mean", "p50", "p95", "p99", "max":
		return metric, statistic, nil
	}
	return "", "", fmt.Errorf("unknown statistic %q; want mean, p50, p95, p99 or max", statistic)
}

// HasCaptures reports whether any step writes an image, so the caller can
// refuse to run a capturing scenario without somewhere to put the output
// rather than discovering it mid-run or, worse, skipping silently.
func (s *Scenario) HasCaptures() bool {
	for _, step := range s.Steps {
		if step.Action == actionCapture {
			return true
		}
	}
	return false
}

// NeedsForegroundWindow reports whether preparing this scenario's window
// should bring it to the foreground, not just restore and clamp it on
// screen. Only a "desktop" capture genuinely needs that -- it reads the
// literal desktop pixels under the app's screen region, which only shows
// the app's own content when nothing else is drawn on top. Every other
// capture source (native/self/window) reads the GPU backbuffer or internal
// render output directly, unaffected by window z-order, so foregrounding
// for those would just be an intrusive alt-tab with no benefit.
func (s *Scenario) NeedsForegroundWindow() bool {
	for _, step := range s.Steps {
		if step.Action == actionCapture && step.Source == "desktop" {
			return true
		}
	}
	return false
}

// portFromURL extracts the TCP port a scenario's base_url targets.
func portFromURL(raw string) (int, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return 0, err
	}
	port := parsed.Port()
	if port == "" {
		return 0, fmt.Errorf("no port in %q", raw)
	}
	return strconv.Atoi(port)
}

func LoadScenario(path string) (*Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var scenario Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if scenario.RegressionTolerancePct <= 0 {
		scenario.RegressionTolerancePct = 15
	}
	if scenario.RegressionFloorMS <= 0 {
		scenario.RegressionFloorMS = 0.5
	}
	// The forward follows base_url unless overridden, so a scene changes port
	// in one place and two scenes can target one device without colliding.
	if scenario.Launch != nil && scenario.Launch.Platform == "android" && scenario.Launch.ForwardPort == 0 {
		if port, err := portFromURL(scenario.BaseURL); err == nil {
			scenario.Launch.ForwardPort = port
		}
	}
	if scenario.PaceMS == nil {
		pace := 16
		scenario.PaceMS = &pace
	}
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	return &scenario, nil
}
