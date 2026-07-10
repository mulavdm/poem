---
type: Concept
title: Performance & Latency Profiling
description: The perf endpoints for rolling frame timings and condition-based action measurement.
tags: [automation, performance, profiling, latency]
timestamp: 2026-07-10T00:00:00Z
---
# Performance And Latency Profiling

POEM's shared automation layer includes lightweight timing hooks intended for downstream-app debugging and agent-driven diagnosis.

## `GET /perf/state`

Returns JSON with:

- `frames` — rolling frame timings for rebuild/render/serialize/write and total frame cost
- `event_batches` — timings for native input batches processed from the sidecar
- `automation` — timings for HTTP automation commands handled inside POEM
- `events` — a capped recent event stream for quick inspection

Important interpretation notes:

- frame timings are measured inside POEM's Go render loop
- event-batch timings only reflect native sidecar input batches
- direct HTTP automation actions like `POST /click` do not go through the native event queue, so they can update UI state without increasing `event_batches.batch_count`
- sub-millisecond values are preserved, so very fast actions may legitimately appear as fractions like `0.18`

## `GET /perf/events`

Returns the recent perf event log.

## `POST /perf/reset`

Clears accumulated perf counters and event history.

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

## See also
- [Performance Benchmarks](/concepts/architecture/performance-benchmarks.md) — engine-level pprof/benchmarks
- [HTTP Endpoints](/concepts/automation/endpoints.md)
