---
type: Concept
title: Performance Observability & Benchmarks
description: pprof endpoints and Go benchmark results for the render backend.
tags: [architecture, performance, pprof, benchmarks]
timestamp: 2026-07-10T00:00:00Z
---
# Performance Observability & Benchmarks

The engine integrates native Go diagnostics to monitor rendering stability.

## PPROF Profiling

- **Server**: `http://127.0.0.1:6060/debug/pprof/`
- **CPU Profiling**: `go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=10`

## Performance Benchmarks

- **Location**: `pkg/render/backend/cpu_test.go`
- **Command**: `go test -v -bench="." go_native_gpu_gui/pkg/render/backend`
- **CPU Performance Metrics**:
  - `BenchmarkPaint` (Full 60FPS UI Redraw): ~1.1 ms (exceeds the <10ms standard by nearly 10x)
  - `BenchmarkDrawRoundedRect` (Alpha-blended SDF panels): ~1.4 ms

## See also
- [Zero-GC Rendering](/concepts/architecture/zero-gc-rendering.md)
- [Automation Perf Profiling](/concepts/automation/perf-profiling.md)