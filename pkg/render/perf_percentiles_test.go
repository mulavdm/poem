package render

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

func ms(v float64) time.Duration {
	return time.Duration(v * float64(time.Millisecond))
}

// 1..100 makes every nearest-rank percentile checkable by inspection, and
// pins the same ranks the native recorder's contract test asserts so a Go p95
// and a native p95 mean the same thing.
func TestFramePercentilesUseNearestRank(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })

	for i := 1; i <= 100; i++ {
		v := float64(i)
		globalPerfTracker.recordFrame(ms(v), 0, 0, 0, ms(v), 1)
	}

	p := globalPerfTracker.snapshot().Frames.Percentiles
	if p.SampleCount != 100 {
		t.Fatalf("sample count = %d, want 100", p.SampleCount)
	}
	if p.SampleWindow != maxPerfSamples {
		t.Fatalf("sample window = %d, want %d", p.SampleWindow, maxPerfSamples)
	}
	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"p50", p.Total.P50MS, 51},
		{"p95", p.Total.P95MS, 95},
		{"p99", p.Total.P99MS, 99},
		{"max", p.Total.MaxMS, 100},
		{"mean", p.Total.MeanMS, 50.5},
	} {
		if diff := tc.got - tc.want; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("total %s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// A frame that is p95 for serialization need not be p95 for layout, so each
// phase must be ranked on its own ordering rather than read off one sort.
func TestFramePercentilesRankPhasesIndependently(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })

	// build ascends while serialize descends: reading serialize off build's
	// ordering would invert its percentiles.
	for i := 1; i <= 100; i++ {
		build := float64(i)
		serialize := float64(101 - i)
		globalPerfTracker.recordFrame(ms(build), 0, ms(serialize), 0, ms(build+serialize), 1)
	}

	p := globalPerfTracker.snapshot().Frames.Percentiles
	if diff := p.BuildPages.P95MS - 95; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("build p95 = %v, want 95", p.BuildPages.P95MS)
	}
	if diff := p.Serialize.P95MS - 95; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("serialize p95 = %v, want 95", p.Serialize.P95MS)
	}
	// GoWork is build+render+serialize, constant at 101 for every frame here.
	if diff := p.GoWork.P95MS - 101; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("go work p95 = %v, want 101", p.GoWork.P95MS)
	}
}

// The sample ring must outlive the 256-entry event trace, which is shared with
// automation entries and wraps after roughly 128 driven interactions.
func TestFrameSamplesOutliveEventRing(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })

	const frames = maxPerfEvents * 3
	for i := 0; i < frames; i++ {
		globalPerfTracker.recordFrame(ms(1), 0, 0, 0, ms(1), 1)
	}

	state := globalPerfTracker.snapshot()
	if len(state.Events) > maxPerfEvents {
		t.Fatalf("event ring grew to %d, cap is %d", len(state.Events), maxPerfEvents)
	}
	if state.Frames.Percentiles.SampleCount != frames {
		t.Fatalf("retained %d samples, want %d", state.Frames.Percentiles.SampleCount, frames)
	}
}

func TestFrameSampleRingIsBounded(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })

	for i := 0; i < maxPerfSamples+500; i++ {
		// Everything before the final window is 1 ms; the window itself is 9 ms.
		v := 9.0
		if i < 500 {
			v = 1.0
		}
		globalPerfTracker.recordFrame(ms(v), 0, 0, 0, ms(v), 1)
	}

	p := globalPerfTracker.snapshot().Frames.Percentiles
	if p.SampleCount != maxPerfSamples {
		t.Fatalf("sample count = %d, want %d", p.SampleCount, maxPerfSamples)
	}
	if diff := p.Total.MeanMS - 9; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("mean = %v, want 9 (oldest samples should be evicted)", p.Total.MeanMS)
	}
}

// Details is %.2f text and quantizes to 0.01 ms; the structured fields must
// carry the real values.
func TestFrameEventCarriesFullPrecisionSplits(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })

	globalPerfTracker.recordFrame(ms(0.001234), ms(0.002345), ms(0.003456), ms(0.004567), ms(0.011602), 7)

	events := globalPerfTracker.snapshot().Events
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.BuildPagesMS <= 0 || e.RenderPipelineMS <= 0 || e.SerializeMS <= 0 || e.WriteMS <= 0 {
		t.Fatalf("structured splits lost sub-0.01ms values: %+v", e)
	}
	if e.CommandCount != 7 {
		t.Errorf("command count = %d, want 7", e.CommandCount)
	}
}

func TestNativePerfEndpointReportsHostPhases(t *testing.T) {
	original := nativeDebugRequest
	t.Cleanup(func() { nativeDebugRequest = original })
	nativeDebugRequest = func(protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		return protocol.NativeDebugResponse{PerfPhases: []protocol.NativePerfPhase{
			{Name: "present", Count: 3, MeanMS: 2, P50MS: 2, P95MS: 4.5, P99MS: 5, MaxMS: 5},
		}}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/perf/native", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var state NativePerfState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(state.Phases) != 1 || state.Phases[0].Name != "present" || state.Phases[0].P95MS != 4.5 {
		t.Fatalf("unexpected native perf state: %+v", state)
	}
}

// Resetting a scoped measurement has to clear both sides of the boundary, or
// the next scene blends with native samples from the previous one.
func TestPerfResetAlsoResetsNativeRings(t *testing.T) {
	globalPerfTracker.reset()
	t.Cleanup(func() { globalPerfTracker.reset() })

	original := nativeDebugRequest
	t.Cleanup(func() { nativeDebugRequest = original })
	var sawReset bool
	nativeDebugRequest = func(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
		sawReset = req.ResetPerf
		return protocol.NativeDebugResponse{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/perf/reset", nil)
	rec := httptest.NewRecorder()
	newAutomationHTTPHandler(resolveAutomationConfig(&AutomationConfig{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !sawReset {
		t.Fatal("/perf/reset did not ask the host to clear its native rings")
	}
}
