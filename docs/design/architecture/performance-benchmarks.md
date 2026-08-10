# Performance Observability & Benchmarks

The engine integrates native Go diagnostics to monitor rendering stability.

For **live per-frame timings on a running app** — including percentiles and the
host's own native frame cost — use the automation perf endpoints instead of the
benchmarks below: [Performance & Latency Profiling](../automation/perf-profiling.md).
Benchmarks measure synthetic paint cost in isolation; the perf endpoints measure
what a real driven scene actually did, on either platform. Note that POEM
repaints on change rather than on a clock, so there is no idle steady state to
sample: every measurement is per-interaction.

## PPROF Profiling

- **Server**: `http://127.0.0.1:6060/debug/pprof/`
- **CPU Profiling**: `go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=10`

## Performance Benchmarks

- **Location**: `pkg/render/backend/cpu_test.go`
- **Command**: `go test -v -bench="." github.com/mulavdm/poem/pkg/render/backend`
- **CPU Performance Metrics**:
  - `BenchmarkPaint` (Full 60FPS UI Redraw): ~1.1 ms (exceeds the <10ms standard by nearly 10x)
  - `BenchmarkDrawRoundedRect` (Alpha-blended SDF panels): ~1.4 ms

## Pinned scenes

Benchmarks measure functions; scenes measure the app. The repeatable
interactions live in `scenes/` as scenario files replayed by `cmd/poemdrive`
(see [Automation Perf Profiling](../automation/perf-profiling.md)):

- `scenes/gallery-tabs.json` — cycles the gallery's four tabs, each click a
  full page rebuild. The reference scene for the Go-side budgets, and cheap
  enough to run often.
- `scenes/mapps-camera.json` — drives the MAPPS map camera through a **balanced**
  zoom cycle that returns to its starting zoom. The balance is the point:
  earlier measurements of this scene differed by ~50% between runs because
  relative zoom clicks were unbalanced, so each run drifted to a different zoom
  depth and covered different geometry. A scene that does not return to its
  starting state is not a fixture.

The scenario files are validated by `cmd/poemdrive`'s test suite, so a typo in
a fixture fails the build rather than silently producing a gate that measures
the wrong thing.

`scenes/mapps-camera.json` deliberately carries **no budgets**. Its recorded
baseline is 15–24x over the migration plan's Go-side budget today, before any
migration work; gating it on the absolute target would fail every run for a
pre-existing condition. Use `-baseline` there and let the absolute targets stay
a separate goal. See `docs/M0_performance_baselines.md`.

## See also
- [Zero-GC Rendering](../architecture/zero-gc-rendering.md)
- [Automation Perf Profiling](../automation/perf-profiling.md)