---
type: Concept
title: Performance & Latency Profiling
description: The perf endpoints for rolling frame timings and condition-based action measurement.
tags: [automation, performance, profiling, latency]
timestamp: 2026-07-23T00:00:00Z
---
# Performance And Latency Profiling

POEM's shared automation layer includes lightweight timing hooks intended for downstream-app debugging and agent-driven diagnosis.

## `GET /perf/state`

Returns JSON with:

- `frames` — rolling frame timings for rebuild/render/serialize/write and total frame cost
- `frames.percentiles` — the distribution over a retained raw-sample window
- `event_batches` — timings for native input batches processed from the presenter
- `automation` — timings for HTTP automation commands handled inside POEM
- `events` — a capped recent event stream for quick inspection

### `frames.percentiles`

The `avg_*`/`max_*` fields alongside it are running aggregates and **cannot
answer a percentile budget**. `percentiles` reports `mean_ms`/`p50_ms`/`p95_ms`/
`p99_ms`/`max_ms` for `total`, `build_pages`, `render_pipeline`, `serialize`,
`write`, and `go_work` (build+render+serialize, the split budgeted separately
from transport write time).

- ranks are nearest-rank over the retained window; `sample_count` saturates at
  `sample_window` (4096 frames), oldest evicted first
- each phase is ranked on its own ordering — a frame that is p95 for
  serialization need not be p95 for layout
- this window is independent of `events`, which is capped at 256 entries shared
  with automation and event-batch entries and therefore wraps after roughly 128
  driven interactions

## `GET /perf/events`

Returns the recent perf event log. Frame entries carry the phase split twice:
`details` is `%.2f`-formatted text for humans and quantizes to 0.01 ms, while
`build_pages_ms`, `render_pipeline_ms`, `serialize_ms`, `write_ms`, and
`command_count` are the full-precision values consumers should read.

## `GET /perf/native`

Returns the **host's own** frame timings, which nothing in `/perf/state` can
see: every figure there is measured on the Go side of the boundary. One entry
per timed native channel, sorted by name so two snapshots can be diffed
positionally:

- `present` — geometry compilation plus draw submission on the UI thread. This
  is the native framework CPU work, excluding GPU execution.
- `decode_frame` — protocol decode of a `RenderFrame` on the transport thread
- `decode_map_scene` / `apply_map_scene` — map-scene decode and retention

Each carries `count`, `mean_ms`, `p50_ms`, `p95_ms`, `p99_ms`, `max_ms`, using
the same nearest-rank definition and the same 4096-sample window size as the Go
tracker, so a native p95 and a Go p95 mean the same thing.

The recorder itself lives in `shared/poem/perf.{h,cpp}`, so the D3D11 and GLES
presenters report the same channel names under the same percentile definition —
a p95 that meant one thing on Windows and another on Android would make the
comparison worthless. Both hosts implement it; the web backend and test drivers
return an error.

## `POST /perf/reset`

Clears accumulated perf counters, the event history, and the retained sample
window. It also asks the host to clear its native rings in the same call, so a
measurement scoped to one driven scene does not blend with native samples from
the previous one. Hosts with no native channel have nothing to clear and this
is not treated as an error.

## `POST /perf/measure-action`

This endpoint is useful when a caller wants "how long until the UI reaches a condition?" instead of only "how long did the command handler run?"

Example body:

```json
{
  "command": "click",
  "id": "wf_z-image-turbo-multi-lora_yaml",
  "timeout_ms": 8000,
  "poll_interval_ms": 20,
  "condition": {
    "component_id": "selected_workflow",
    "contains_text": "z-image-turbo-multi-lora.yaml"
  }
}
```

Supported condition fields:

- `focused_id`
- `current_page`
- `component_id`
- `contains_text`
- `window_width_at_least`
- `window_height_at_least`
- `min_frame_count_delta`

Example:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:47831/perf/measure-action `
  -ContentType 'application/json' `
  -Body '{"command":"click","id":"wf_z-image-turbo-multi-lora_yaml","timeout_ms":8000,"poll_interval_ms":20,"condition":{"component_id":"selected_workflow","contains_text":"z-image-turbo-multi-lora.yaml"}}'
```

## `POST /perf/measure-native-action`

This endpoint is for timing native window-control behavior rather than POEM-internal component actions.

Example body:

```json
{
  "maximize_window": true,
  "bring_to_foreground": true,
  "timeout_ms": 8000,
  "poll_interval_ms": 20,
  "condition": {
    "window_foreground": true,
    "window_not_minimized": true,
    "client_width_at_least": 1400
  }
}
```

Supported request action fields:

- `restore_window`
- `clamp_to_work_area`
- `bring_to_foreground`
- `maximize_window`

Supported native condition fields:

- `window_visible`
- `window_foreground`
- `window_not_minimized`
- `client_width_at_least`
- `client_height_at_least`
- `backbuffer_width_at_least`
- `backbuffer_height_at_least`
- `window_width_at_least`
- `window_height_at_least`
- `min_frame_count_delta`
- `min_event_batch_count_delta`

This is the preferred endpoint when the question is about:

- maximize or restore latency
- window-state readiness
- whether native resize handling produced follow-up frame or event activity
- desktop-facing sluggishness that a synthetic `POST /click` does not exercise

The response includes: `elapsed_ms`, `condition_met`, `frame_delta`, `perf`, `final_state`. This makes it possible to compare raw automation command cost, number of frames produced while the UI settles, and current render timings at the moment the condition is met.

Example:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:47831/perf/measure-native-action `
  -ContentType 'application/json' `
  -Body '{"maximize_window":true,"bring_to_foreground":true,"timeout_ms":8000,"poll_interval_ms":20,"condition":{"window_foreground":true,"window_not_minimized":true,"client_width_at_least":1400}}'
```

## Scenario replay: `cmd/poemdrive`

The endpoints above report *what a run cost*; they say nothing about whether
two runs did the same work. `cmd/poemdrive` supplies that half. It replays a
scenario file against a running app, then gates the timings against budgets and
a recorded baseline.

```bash
go run ./cmd/poemdrive scenes/gallery-tabs.json
go run ./cmd/poemdrive -save-baseline gallery.json scenes/gallery-tabs.json
go run ./cmd/poemdrive -baseline gallery.json scenes/gallery-tabs.json
```

Nothing about any app is compiled in: the scenario supplies `base_url` and the
step list, so the same binary drives the Windows gallery, MAPPS, or an Android
device reached through `adb forward`. Steps are `click`, `focus`, `set-text`,
`press-key`, and `wait`.

Metrics arrive in one flat namespace: `go.<phase>` from `frames.percentiles`
and `native.<phase>` from `/perf/native`, so a budget reads
`go.total.p95` or `native.present.p95`. Percentiles are taken from the
server-side ring rather than reconstructed by polling `/perf/events`, which is
both exact and free of the polling load that would otherwise perturb what is
being measured.

Three behaviours are deliberate and worth keeping:

- **Warmup runs before the reset**, so first-paint costs and lazily built
  caches never enter the sample.
- **A budget naming a metric the run did not produce fails.** Passing because
  the measurement is absent is the worst outcome a gate can have, so a
  `native.*` budget against a host with no native debug channel reports exactly
  that rather than succeeding quietly.
- **A regression must exceed both a percentage and an absolute floor**
  (`regression_floor_ms`, default 0.5). Percentage alone is the wrong
  instrument near the timer resolution: Go-side timings on Windows quantize to
  roughly 0.5 ms, and two consecutive runs of an *unchanged* gallery binary
  were measured drifting 19–35% at p95/p99 for that reason alone. A gate that
  fails on an unchanged binary gets switched off.

## See also
- [Performance Benchmarks](/concepts/architecture/performance-benchmarks.md) — engine-level pprof/benchmarks
- [HTTP Endpoints](/concepts/automation/endpoints.md)
