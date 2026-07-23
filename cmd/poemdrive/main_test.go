package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/render"
)

func TestSplitBudgetKey(t *testing.T) {
	metric, statistic, err := splitBudgetKey("go.total.p95")
	if err != nil || metric != "go.total" || statistic != "p95" {
		t.Fatalf("got %q/%q err=%v", metric, statistic, err)
	}
	for _, bad := range []string{"go.total", "go.total.", "go.total.p90", "p95", ""} {
		if _, _, err := splitBudgetKey(bad); err == nil {
			t.Errorf("splitBudgetKey(%q) should have failed", bad)
		}
	}
}

func TestScenarioValidate(t *testing.T) {
	valid := Scenario{
		Name: "s", BaseURL: "http://127.0.0.1:1", Iterations: 1,
		Steps: []Step{{Action: actionClick, ID: "a"}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid scenario rejected: %v", err)
	}

	cases := map[string]Scenario{
		"no name":          {BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionClick, ID: "a"}}},
		"no base_url":      {Name: "s", Iterations: 1, Steps: []Step{{Action: actionClick, ID: "a"}}},
		"no steps":         {Name: "s", BaseURL: "u", Iterations: 1},
		"no iterations":    {Name: "s", BaseURL: "u", Steps: []Step{{Action: actionClick, ID: "a"}}},
		"click without id": {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionClick}}},
		"unknown action":   {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: "teleport"}}},
		"wait without ms":  {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionWait}}},
	}
	for name, scenario := range cases {
		if err := scenario.Validate(); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}

	bad := valid
	bad.Budgets = map[string]float64{"go.total.p90": 1}
	if err := bad.Validate(); err == nil {
		t.Error("a budget with an unknown statistic should be rejected before the app is driven")
	}
}

// The shipped scenes are fixtures; a typo in one is a silent gate failure, so
// they are validated as part of the test suite.
func TestShippedScenesAreValid(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "scenes", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no scenes found; the fixtures they pin are documented in docs/M0_performance_baselines.md")
	}
	for _, path := range paths {
		if _, err := LoadScenario(path); err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)
		}
	}
}

func TestCheckBudgets(t *testing.T) {
	m := Measurement{
		NativeAvailable: true,
		Metrics: map[string]Stats{
			"go.total":       {P95: 3.0, P99: 7.0},
			"native.present": {P95: 0.9},
		},
	}
	if failures := CheckBudgets(m, map[string]float64{"go.total.p95": 8.33}); len(failures) != 0 {
		t.Errorf("within budget should pass, got %+v", failures)
	}
	failures := CheckBudgets(m, map[string]float64{"go.total.p95": 2.0})
	if len(failures) != 1 || !strings.Contains(failures[0].Reason, "exceeds budget") {
		t.Errorf("over budget should fail, got %+v", failures)
	}

	// A budget on a metric this run never produced must fail rather than pass
	// quietly — that is the difference between "met the budget" and "did not
	// measure it".
	failures = CheckBudgets(m, map[string]float64{"go.nonexistent.p95": 1.0})
	if len(failures) != 1 || !strings.Contains(failures[0].Reason, "not measured") {
		t.Errorf("absent metric should fail, got %+v", failures)
	}

	noNative := Measurement{NativeAvailable: false, Metrics: map[string]Stats{"go.total": {P95: 1}}}
	failures = CheckBudgets(noNative, map[string]float64{"native.present.p95": 4.17})
	if len(failures) != 1 || !strings.Contains(failures[0].Reason, "no native debug channel") {
		t.Errorf("a native budget on a host without the channel should explain why, got %+v", failures)
	}
}

func TestCheckRegression(t *testing.T) {
	baseline := Measurement{Metrics: map[string]Stats{"go.total": {P95: 1.0, P99: 2.0}}}

	same := Measurement{Metrics: map[string]Stats{"go.total": {P95: 1.05, P99: 2.0}}}
	if failures := CheckRegression(same, baseline, 15, 0); len(failures) != 0 {
		t.Errorf("5%% drift inside a 15%% tolerance should pass, got %+v", failures)
	}

	worse := Measurement{Metrics: map[string]Stats{"go.total": {P95: 1.5, P99: 2.0}}}
	failures := CheckRegression(worse, baseline, 15, 0)
	if len(failures) != 1 || !strings.Contains(failures[0].Key, "p95") {
		t.Errorf("50%% drift should fail on p95, got %+v", failures)
	}

	// Getting faster is never a failure.
	better := Measurement{Metrics: map[string]Stats{"go.total": {P95: 0.5, P99: 1.0}}}
	if failures := CheckRegression(better, baseline, 15, 0); len(failures) != 0 {
		t.Errorf("an improvement should pass, got %+v", failures)
	}

	// A metric absent from the baseline is new, not a regression.
	added := Measurement{Metrics: map[string]Stats{"native.present": {P95: 99}}}
	if failures := CheckRegression(added, baseline, 15, 0); len(failures) != 0 {
		t.Errorf("a metric missing from the baseline should be skipped, got %+v", failures)
	}
}

func TestCollectNamespacesMetrics(t *testing.T) {
	perf := render.PerfState{}
	perf.Frames.Percentiles.SampleCount = 12
	perf.Frames.Percentiles.Total = render.PerfPhaseStats{P95MS: 3.5}
	native := render.NativePerfState{Phases: []render.NativePerfPhaseState{{Name: "present", P95MS: 0.9, Count: 12}}}

	m := collect(perf, native, true)
	if m.Frames != 12 {
		t.Errorf("Frames = %d, want 12", m.Frames)
	}
	if got := m.Metrics["go.total"].P95; got != 3.5 {
		t.Errorf("go.total p95 = %v, want 3.5", got)
	}
	if got := m.Metrics["native.present"].P95; got != 0.9 {
		t.Errorf("native.present p95 = %v, want 0.9", got)
	}

	withoutNative := collect(perf, render.NativePerfState{}, false)
	if _, ok := withoutNative.Metrics["native.present"]; ok {
		t.Error("native metrics must be absent when the channel is unavailable, not zero-valued")
	}
	if withoutNative.NativeAvailable {
		t.Error("NativeAvailable should be false")
	}
}

func TestMeasurementRoundTrips(t *testing.T) {
	original := Measurement{
		Scenario: "s", Frames: 3, NativeAvailable: true,
		Metrics: map[string]Stats{"go.total": {Count: 3, P95: 1.25}},
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Measurement
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Metrics["go.total"].P95 != 1.25 || !decoded.NativeAvailable {
		t.Errorf("baseline did not survive a round trip: %+v", decoded)
	}
}

// The absolute floor is what makes the gate usable. Two consecutive runs of an
// unchanged gallery binary were observed drifting 19-35% at p95/p99 because
// Go-side timings quantize to roughly 0.5 ms on Windows and the metric sits
// near that resolution. Percentage alone would fail those runs.
func TestRegressionFloorSuppressesTimerNoise(t *testing.T) {
	baseline := Measurement{Metrics: map[string]Stats{"go.total": {P95: 0.572, P99: 0.623}}}
	noisy := Measurement{Metrics: map[string]Stats{"go.total": {P95: 0.773, P99: 0.744}}}

	if failures := CheckRegression(noisy, baseline, 15, 0); len(failures) == 0 {
		t.Fatal("without a floor this drift should fail; the test fixture is wrong")
	}
	if failures := CheckRegression(noisy, baseline, 15, 0.5); len(failures) != 0 {
		t.Errorf("a 0.2 ms move below the 0.5 ms floor must not fail: %+v", failures)
	}

	// A genuine regression clears both bars.
	real := Measurement{Metrics: map[string]Stats{"go.total": {P95: 2.6, P99: 3.0}}}
	failures := CheckRegression(real, baseline, 15, 0.5)
	if len(failures) != 2 {
		t.Errorf("a 2 ms regression should still fail on p95 and p99, got %+v", failures)
	}
}
