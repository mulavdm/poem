package main

import (
	"fmt"
	"sort"

	"github.com/mulavdm/poem/pkg/render"
)

// Stats is one metric's distribution. It mirrors render.PerfPhaseStats and
// render.NativePerfPhaseState so both sides of the language boundary reduce to
// the same shape — a p95 that means one thing in Go and another in C++ makes
// the comparison worthless.
type Stats struct {
	Count int     `json:"count"`
	Mean  float64 `json:"mean_ms"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
	Max   float64 `json:"max_ms"`
}

// Measurement is one scenario run, in the form written to and read from a
// baseline file.
type Measurement struct {
	Scenario string           `json:"scenario"`
	Captured string           `json:"captured"`
	Frames   int              `json:"frames"`
	Metrics  map[string]Stats `json:"metrics"`
	// NativeAvailable records whether the host exposed /perf/native. A host
	// without a native debug channel is not a failure, but a run that silently
	// dropped the native half would be indistinguishable from one that met its
	// native budget.
	NativeAvailable bool `json:"native_available"`
}

func statsFromPhase(phase render.PerfPhaseStats, count int) Stats {
	return Stats{
		Count: count,
		Mean:  phase.MeanMS,
		P50:   phase.P50MS,
		P95:   phase.P95MS,
		P99:   phase.P99MS,
		Max:   phase.MaxMS,
	}
}

func statsFromNativePhase(phase render.NativePerfPhaseState) Stats {
	return Stats{
		Count: int(phase.Count),
		Mean:  phase.MeanMS,
		P50:   phase.P50MS,
		P95:   phase.P95MS,
		P99:   phase.P99MS,
		Max:   phase.MaxMS,
	}
}

// collect reduces the two perf endpoints into one flat metric namespace.
// `go.*` is measured on the Go side of the boundary and `native.*` on the
// host side; the plan budgets them separately because they are separately
// actionable.
func collect(perf render.PerfState, native render.NativePerfState, nativeOK bool) Measurement {
	pct := perf.Frames.Percentiles
	n := pct.SampleCount
	metrics := map[string]Stats{
		"go.total":           statsFromPhase(pct.Total, n),
		"go.build_pages":     statsFromPhase(pct.BuildPages, n),
		"go.render_pipeline": statsFromPhase(pct.RenderPipeline, n),
		"go.serialize":       statsFromPhase(pct.Serialize, n),
		"go.write":           statsFromPhase(pct.Write, n),
		"go.go_work":         statsFromPhase(pct.GoWork, n),
	}
	if nativeOK {
		for _, phase := range native.Phases {
			metrics["native."+phase.Name] = statsFromNativePhase(phase)
		}
	}
	return Measurement{Frames: n, Metrics: metrics, NativeAvailable: nativeOK}
}

func (s Stats) value(statistic string) float64 {
	switch statistic {
	case "mean":
		return s.Mean
	case "p50":
		return s.P50
	case "p95":
		return s.P95
	case "p99":
		return s.P99
	case "max":
		return s.Max
	}
	return 0
}

// Failure is one unmet budget or one regression against a baseline.
type Failure struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// CheckBudgets reports every budget the run missed. A budget naming a metric
// the run never produced is itself a failure: silently passing because the
// measurement is absent is the worst outcome a gate can have.
func CheckBudgets(m Measurement, budgets map[string]float64) []Failure {
	keys := make([]string, 0, len(budgets))
	for key := range budgets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var failures []Failure
	for _, key := range keys {
		metric, statistic, err := splitBudgetKey(key)
		if err != nil {
			failures = append(failures, Failure{key, err.Error()})
			continue
		}
		stats, ok := m.Metrics[metric]
		if !ok {
			reason := fmt.Sprintf("metric %q was not measured in this run", metric)
			if !m.NativeAvailable && len(metric) > 7 && metric[:7] == "native." {
				reason += "; this host exposes no native debug channel"
			}
			failures = append(failures, Failure{key, reason})
			continue
		}
		if got := stats.value(statistic); got > budgets[key] {
			failures = append(failures, Failure{key,
				fmt.Sprintf("%.3f ms exceeds budget %.3f ms", got, budgets[key])})
		}
	}
	return failures
}

// CheckRegression compares against a recorded baseline. This is the primary
// gate: the absolute budgets describe where the project wants to be, while a
// regression check describes whether this change made things worse, and only
// the second is a fair question to ask of any single commit.
//
// A regression must exceed both the percentage tolerance and floorMS, an
// absolute delta. Percentage alone is the wrong instrument once a metric
// approaches the timer resolution: Go-side timings on Windows quantize to
// roughly 0.5 ms, so a p95 of 0.55 ms moving one bucket reads as +35% while
// meaning nothing. Without the floor this gate fails on consecutive runs of an
// unchanged binary, and a gate that cries wolf is switched off.
func CheckRegression(current, baseline Measurement, tolerancePct, floorMS float64) []Failure {
	names := make([]string, 0, len(current.Metrics))
	for name := range current.Metrics {
		names = append(names, name)
	}
	sort.Strings(names)

	var failures []Failure
	for _, name := range names {
		was, ok := baseline.Metrics[name]
		if !ok {
			continue
		}
		now := current.Metrics[name]
		for _, statistic := range []string{"p95", "p99"} {
			before, after := was.value(statistic), now.value(statistic)
			if before <= 0 {
				continue
			}
			drift := (after - before) / before * 100
			if drift > tolerancePct && (after-before) > floorMS {
				failures = append(failures, Failure{
					name + "." + statistic,
					fmt.Sprintf("%.3f ms vs baseline %.3f ms (+%.1f%% and +%.3f ms; tolerance %.1f%% / %.3f ms)",
						after, before, drift, after-before, tolerancePct, floorMS),
				})
			}
		}
	}
	return failures
}
