package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
		"no name":                   {BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionClick, ID: "a"}}},
		"no base_url":               {Name: "s", Iterations: 1, Steps: []Step{{Action: actionClick, ID: "a"}}},
		"no steps":                  {Name: "s", BaseURL: "u", Iterations: 1},
		"no iterations":             {Name: "s", BaseURL: "u", Steps: []Step{{Action: actionClick, ID: "a"}}},
		"click without id":          {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionClick}}},
		"unknown action":            {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: "teleport"}}},
		"wait without ms":           {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionWait}}},
		"resize without dimensions": {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionResize}}},
		"resize with zero height":   {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionResize, Width: 800}}},
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

	withResize := valid
	withResize.Steps = []Step{{Action: actionResize, Width: 800, Height: 600}}
	if err := withResize.Validate(); err != nil {
		t.Errorf("resize step with positive dimensions rejected: %v", err)
	}

	assertCases := map[string]Scenario{
		"assert-exists without id":      {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionAssertExists}}},
		"assert-absent without id":      {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionAssertAbsent}}},
		"assert-count without prefix":   {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionAssertCount, Count: 1}}},
		"assert-no-overlap with one id": {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionAssertNoOverlap, IDs: []string{"a"}}}},
		"assert-no-overlap with no ids": {Name: "s", BaseURL: "u", Iterations: 1, Steps: []Step{{Action: actionAssertNoOverlap}}},
	}
	for name, scenario := range assertCases {
		if err := scenario.Validate(); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}

	validAsserts := Scenario{
		Name: "s", BaseURL: "u", Iterations: 1,
		Steps: []Step{
			{Action: actionAssertExists, ID: "a"},
			{Action: actionAssertAbsent, ID: "b"},
			{Action: actionAssertCount, IDPrefix: "row_", Count: 0},
			{Action: actionAssertNoOverlap, IDs: []string{"a", "b"}},
		},
	}
	if err := validAsserts.Validate(); err != nil {
		t.Errorf("valid assertion steps rejected: %v", err)
	}
}

func TestNeedsForegroundWindow(t *testing.T) {
	nativeOnly := Scenario{Steps: []Step{{Action: actionCapture, Name: "a", Source: "native"}}}
	if nativeOnly.NeedsForegroundWindow() {
		t.Error("a native-only capture should not need the foreground")
	}
	noSource := Scenario{Steps: []Step{{Action: actionCapture, Name: "a"}}}
	if noSource.NeedsForegroundWindow() {
		t.Error("an unset capture source defaults to native and should not need the foreground")
	}
	noCaptures := Scenario{Steps: []Step{{Action: actionClick, ID: "a"}}}
	if noCaptures.NeedsForegroundWindow() {
		t.Error("a scenario with no captures should not need the foreground")
	}
	desktop := Scenario{Steps: []Step{{Action: actionCapture, Name: "a", Source: "desktop"}}}
	if !desktop.NeedsForegroundWindow() {
		t.Error("a desktop capture should need the foreground")
	}
	mixed := Scenario{Steps: []Step{
		{Action: actionCapture, Name: "a", Source: "native"},
		{Action: actionCapture, Name: "b", Source: "desktop"},
	}}
	if !mixed.NeedsForegroundWindow() {
		t.Error("any desktop capture in the scenario should need the foreground")
	}
}

// componentsServer fakes /components with a fixed flat list, for testing the
// assertion helpers without a real running app.
func componentsServer(t *testing.T, flat []map[string]any) *driver {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"flat": flat})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return &driver{scenario: &Scenario{BaseURL: server.URL}, client: http.DefaultClient}
}

func TestAssertComponentExistsAndAbsent(t *testing.T) {
	d := componentsServer(t, []map[string]any{{"id": "a"}})

	if err := d.assertComponentExists("a"); err != nil {
		t.Errorf("expected id %q to exist: %v", "a", err)
	}
	if err := d.assertComponentExists("missing"); err == nil {
		t.Error("expected assertComponentExists to fail for a missing id")
	}
	if err := d.assertComponentAbsent("missing"); err != nil {
		t.Errorf("expected id %q to be absent: %v", "missing", err)
	}
	if err := d.assertComponentAbsent("a"); err == nil {
		t.Error("expected assertComponentAbsent to fail for a present id")
	}
}

func TestAssertComponentCount(t *testing.T) {
	d := componentsServer(t, []map[string]any{
		{"id": "row_1"}, {"id": "row_2"}, {"id": "row_3"}, {"id": "other"},
	})

	if err := d.assertComponentCount("row_", 3); err != nil {
		t.Errorf("expected 3 matches for prefix row_: %v", err)
	}
	if err := d.assertComponentCount("row_", 2); err == nil {
		t.Error("expected a wrong count to fail")
	}
	if err := d.assertComponentCount("nonexistent_", 0); err != nil {
		t.Errorf("expecting zero matches should be a valid, passing assertion: %v", err)
	}
}

func TestAssertNoOverlap(t *testing.T) {
	separated := componentsServer(t, []map[string]any{
		{"id": "a", "bounds": map[string]int{"x": 0, "y": 0, "w": 10, "h": 10}},
		{"id": "b", "bounds": map[string]int{"x": 20, "y": 0, "w": 10, "h": 10}},
	})
	if err := separated.assertNoOverlap([]string{"a", "b"}); err != nil {
		t.Errorf("non-overlapping rects should pass: %v", err)
	}

	overlapping := componentsServer(t, []map[string]any{
		{"id": "a", "bounds": map[string]int{"x": 0, "y": 0, "w": 10, "h": 10}},
		{"id": "b", "bounds": map[string]int{"x": 5, "y": 5, "w": 10, "h": 10}},
	})
	if err := overlapping.assertNoOverlap([]string{"a", "b"}); err == nil {
		t.Error("expected overlapping rects to fail")
	}

	missing := componentsServer(t, []map[string]any{
		{"id": "a", "bounds": map[string]int{"x": 0, "y": 0, "w": 10, "h": 10}},
	})
	if err := missing.assertNoOverlap([]string{"a", "b"}); err == nil {
		t.Error("expected a missing id to fail")
	}
}

// TestRunFunctionalModeSkipsNoFramesError proves gap C's actual fix: a
// scenario built purely from assertion steps produces zero frames (nothing
// it does repaints anything), which run() otherwise rejects outright.
func TestRunFunctionalModeSkipsNoFramesError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"flat": []map[string]any{{"id": "a"}}})
	})
	mux.HandleFunc("/perf/reset", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/perf/state", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(render.PerfState{})
	})
	mux.HandleFunc("/perf/native", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	defer server.Close()

	pace := 0
	scenario := &Scenario{
		Name: "s", BaseURL: server.URL, Iterations: 1, PaceMS: &pace,
		Steps: []Step{{Action: actionAssertExists, ID: "a"}},
	}
	d := &driver{scenario: scenario, client: http.DefaultClient}

	if _, err := d.run(); err == nil || !strings.Contains(err.Error(), "no frames") {
		t.Fatalf("expected a no-frames error without Functional set, got %v", err)
	}

	scenario.Functional = true
	m, err := d.run()
	if err != nil {
		t.Fatalf("a Functional scenario with zero frames should succeed, got: %v", err)
	}
	if m.Frames != 0 {
		t.Fatalf("expected zero frames, got %d", m.Frames)
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

func TestForwardPortFollowsBaseURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	body := `{"name":"s","base_url":"http://127.0.0.1:47999","iterations":1,
	  "launch":{"platform":"android","package":"com.example","avd":"x"},
	  "steps":[{"action":"click","id":"a"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	scenario, err := LoadScenario(path)
	if err != nil {
		t.Fatal(err)
	}
	// Two scenes on one device must not collide, so the forward follows the
	// port the scene already declared rather than a second hard-coded copy.
	if scenario.Launch.ForwardPort != 47999 {
		t.Errorf("ForwardPort = %d, want 47999", scenario.Launch.ForwardPort)
	}
}

func TestTeardownOnlyStopsWhatItStarted(t *testing.T) {
	// Nothing was started, so teardown must be a no-op rather than killing a
	// device or app the developer already had running.
	borrowed := &launcher{launch: &Launch{Platform: "android", Package: "com.example"}}
	borrowed.Teardown() // must not panic, and has no adb path to call

	if borrowed.bootedAVD || borrowed.startedApp || borrowed.forwardedPort != 0 {
		t.Error("a launcher that started nothing should record nothing to tear down")
	}
}
