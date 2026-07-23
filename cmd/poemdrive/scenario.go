package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
}

// Step is one interaction. Exactly one action per step.
type Step struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty"`
	Value  string `json:"value,omitempty"`
	Key    string `json:"key,omitempty"`
	MS     int    `json:"ms,omitempty"`

	// Capture only. Name becomes the PNG filename; Source selects which image
	// the host should produce.
	Name   string `json:"name,omitempty"`
	Source string `json:"source,omitempty"`
}

const (
	actionClick    = "click"
	actionFocus    = "focus"
	actionSetText  = "set-text"
	actionPressKey = "press-key"
	actionWait     = "wait"
	actionCapture  = "capture"
)

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
		case actionWait:
			if step.MS <= 0 {
				return fmt.Errorf("step %d (wait) needs a positive ms", i)
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
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	return &scenario, nil
}
