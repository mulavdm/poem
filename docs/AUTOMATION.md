# POEM Automation Reference

POEM 2.0 component snapshots additionally expose platform-neutral semantic
`role`, `name`, `value`, `description`, state flags, and supported `actions`.
Stable IDs remain supported and are the preferred selector when known.
Interaction commands may instead provide a semantic selector. Selectors must
resolve to exactly one node; zero matches and ambiguous matches are errors.

This document is the source of truth for POEM's automation and inspection system.

It is written for both:

- humans operating or debugging downstream POEM apps
- agents that need a reliable way to inspect, drive, and reason about native POEM windows

The automation layer is implemented inside `POEM/pkg/render` and is shared by all downstream apps that enable it through `render.AutomationConfig`.

## Goals

POEM automation exists to solve three related problems:

1. **Drive UI state** without writing per-app debug code.
2. **Inspect component trees** and focus state from outside the app process.
3. **Capture frames correctly**, while explicitly distinguishing between:
   - what the app is rendering internally
   - what its own window is presenting
   - what is actually visible on the user's desktop

That third distinction is critical. A native app can be alive and rendering correctly while still being occluded, backgrounded, or partially off-screen.

## Configuration

Automation is enabled through `render.AppConfig.Automation`.

Example:

```go
render.Run(render.AppConfig{
    Title:        "My POEM App",
    Width:        1280,
    Height:       800,
    BuildPagesFn: buildPages,
    Automation: &render.AutomationConfig{
        Enabled:    true,
        Mode:       "http",
        Host:       "127.0.0.1",
        Port:       47831,
        CaptureDir: filepath.Join(rootDir, "output", "automation"),
        Verbose:    true,
    },
})
```

### `AutomationConfig`

- `Enabled bool`
  - turns automation on
- `Mode string`
  - recommended value: `http`
  - optional legacy value: `pipe`
- `Host string`
  - default: `127.0.0.1`
- `Port int`
  - default: `47831`
- `PipeName string`
  - only used in pipe mode
- `CaptureDir string`
  - base directory for disk-backed frame captures
- `Verbose bool`
  - logs automation server startup details

## Transport Model

POEM's automation system has two layers:

1. **transport-agnostic automation core**
   - component snapshots
   - click/focus/set-text/press-key
   - frame serialization
   - native debug / native capture requests
2. **transport adapters**
   - HTTP
   - optional named pipe legacy transport

HTTP is the recommended path because it works well across shell contexts, test runners, and agent-driven inspection workflows.

## HTTP Endpoints

All endpoints bind only to localhost when HTTP automation is enabled.

### State and components

- `GET /state`
  - returns current page, focused ID, hovered ID, and logical/physical window size
- `GET /components`
  - returns both hierarchical and flat component snapshots

### Interaction commands

- `POST /click`
  - body: `{"id":"component_id"}`
  - semantic alternative: `{"selector":{"role":"button","name":"Publish","states":{"enabled":true}}}`
- `POST /focus`
  - body: `{"id":"component_id"}`
- `POST /set-text`
- `POST /select-text` with `{ "id": "field", "start": 0, "end": 4 }`
- `POST /composition-start`, `/composition-update`, and `/composition-end`
  - body: `{"id":"component_id","value":"new text"}`
- `POST /press-key`
  - body: `{"key":"Enter"}`
  - named keys include `Tab`, `Shift+Tab`, `Enter`, `Space`, `Backspace`, `Delete`,
    `Escape`, the four arrow keys, `Home`, `End`, `Page Up`, and `Page Down`

`click`, `focus`, `set-text`, `select-text`, and composition commands accept
either `id` or `selector`, never both. Selector `role` and accessible `name`
use exact case-insensitive matching. The `states` map accepts `enabled`,
`disabled`, `focused`, `selected`, `checked`, `expanded`, `read_only`,
`required`, `invalid`, `password`, and `offscreen`, with either `true` or
`false` values. Successful targeted commands return the resolved stable ID as
`target_id`. This keeps semantic scripts readable without sacrificing stable
ID diagnostics.

HTTP JSON request bodies are capped at 1 MiB. Selector text, state counts, and
IDs have tighter limits; unknown states, empty selectors, dangling semantic
trees, and ambiguous matches fail without performing an action.

### Frame capture endpoints

- `POST /capture-frame`
  - writes a PNG to disk
  - body optional: `{"path":"custom.png"}`
- `GET /frame`
  - returns a PNG built from POEM's last logical render frame

### Native state and native capture endpoints

- `GET /native-state`
  - returns native window, DPI, work-area, client-area, and backbuffer state
- `GET /self-frame`
  - returns the app's own internal render output
- `GET /window-frame`
  - returns a client-area capture from the app window itself
- `GET /desktop-frame`
  - returns the actual desktop pixels under the app's client-area screen region

### Shared window-control and inspection endpoints

- `POST /prepare-window`
  - shared helper for restore/clamp/foreground
  - request body may also include `maximize_window`
- `POST /inspect-frame`
  - shared "best inspection frame" endpoint for agents and tests

### Performance endpoints

- `GET /perf/state`
  - returns rolling frame, event-batch, and automation-action timing snapshots
- `GET /perf/events`
  - returns the recent perf event log
- `POST /perf/reset`
  - clears accumulated perf counters and event history
- `POST /perf/measure-action`
  - runs an automation command, then waits for a condition and reports elapsed time plus the current perf snapshot
- `POST /perf/measure-native-action`
  - runs a native window-control action, then waits for native window-state conditions and reports elapsed time plus the current perf snapshot

## Native State Fields

`GET /native-state` returns a JSON structure with the current native window state.

Important fields:

- `dpi`
- `window_visible`
- `window_minimized`
- `window_foreground`
- `window_left`
- `window_top`
- `window_right`
- `window_bottom`
- `client_width`
- `client_height`
- `work_left`
- `work_top`
- `work_right`
- `work_bottom`
- `backbuffer_width`
- `backbuffer_height`

Interpretation:

- `window_visible=true` only means the window exists and is shown, not that it is unobstructed
- `window_foreground=true` means the app is currently frontmost
- `window_minimized=true` means desktop-visible inspection is not meaningful

## The Three Capture Modes

### 1. `self-frame`

`GET /self-frame`

This captures the app's own internal render output from the POEM/native rendering path.

Use it when you want to know:

- whether the app itself is rendering the right content
- whether a layout bug is inside POEM/app logic instead of desktop/window visibility
- what the app looks like even if it is backgrounded or occluded

This does **not** prove the user can actually see that frame on their desktop.

### 2. `window-frame`

`GET /window-frame`

This captures the app window's client-area presentation.

Use it when you want a window-owned view that is closer to native presentation than the logical backbuffer capture.

This is useful, but it is still not the same thing as the literal desktop result in every occlusion scenario.

### 3. `desktop-frame`

`GET /desktop-frame`

This captures the desktop surface currently occupying the app's client-area screen region.

Use it when you want to know:

- what is literally visible on the desktop right now
- whether another window is covering the app
- whether the app is on-screen in the way the user experiences it

If the app is behind another window, `desktop-frame` will show the other window.

## Why The Distinction Matters

These three cases can all happen:

1. `self-frame` looks correct, but `desktop-frame` shows another app
2. the app is visible but not foregrounded, so `desktop-frame` may be unreliable for desktop-facing inspection
3. the app is minimized or off-screen, so internal render health and desktop visibility diverge completely

This is why agents should not treat a single screenshot path as universal truth.

## `POST /prepare-window`

This endpoint applies shared, narrow native window-control actions before inspection.

POEM's current Windows implementation uses a stronger foreground-activation sequence than a plain `SetForegroundWindow` call alone. When requested, it may combine restore/show, z-order nudging, and thread-input-assisted activation so downstream apps can be surfaced more reliably during debugging and automation.

Default behavior:

```json
{
  "restore_window": true,
  "clamp_to_work_area": true,
  "bring_to_foreground": true,
  "maximize_window": false
}
```

Request fields:

- `restore_window`
  - restore a minimized or hidden window
- `clamp_to_work_area`
  - keep the window within the current monitor work area
- `bring_to_foreground`
  - request foreground activation
- `maximize_window`
  - request a native maximize transition

Response:

- JSON native window state, same shape as `GET /native-state`

This endpoint is intentionally narrow. It is for safe inspection setup, not for broad desktop automation.

## `POST /inspect-frame`

This is the recommended shared inspection endpoint for agents.

Default behavior:

```json
{
  "prepare_window": true,
  "restore_window": true,
  "clamp_to_work_area": true,
  "bring_to_foreground": true,
  "prefer_desktop": true,
  "fallback_to_self": true
}
```

### Request fields

- `prepare_window`
- `restore_window`
- `clamp_to_work_area`
- `bring_to_foreground`
- `prefer_desktop`
- `fallback_to_self`

### Behavior

1. Optionally prepares the window using the shared native helper.
2. Reads native state.
3. If desktop inspection is preferred and the window is:
   - visible
   - not minimized
   - foregrounded
   then it returns a desktop-visible capture.
4. Otherwise, if fallback is enabled, it returns the app's internal self-frame.

### Response

`POST /inspect-frame` returns `image/png`.

Headers include:

- `X-POEM-Frame-Source`
  - `desktop` or `self`
- `X-POEM-Window-Visible`
- `X-POEM-Window-Minimized`
- `X-POEM-Window-Foreground`

That lets callers reason about why a given frame source was chosen.

## Recommended Agent Workflow

For general inspection, use this order:

1. Call `POST /inspect-frame`
2. Read `X-POEM-Frame-Source`
3. If source is `desktop`
   - treat it as desktop-visible truth
4. If source is `self`
   - treat it as app-render truth, not desktop-visibility proof
5. If needed, call `GET /native-state` for more exact visibility context

### Practical meaning

- `source=desktop`
  - the app was foreground-ready and the returned image reflects desktop-visible output
- `source=self`
  - the app is alive and rendering, but POEM did not trust desktop-visible capture conditions enough to use that as the primary result

## Performance And Latency Profiling

POEM's shared automation layer also includes lightweight timing hooks intended for downstream-app debugging and agent-driven diagnosis.

### `GET /perf/state`

Returns JSON with:

- `frames`
  - rolling frame timings for rebuild/render/serialize/write and total frame cost
- `event_batches`
  - timings for native input batches processed from the sidecar
- `automation`
  - timings for HTTP automation commands handled inside POEM
- `events`
  - a capped recent event stream for quick inspection

Important interpretation notes:

- frame timings are measured inside POEM's Go render loop
- event-batch timings only reflect native sidecar input batches
- direct HTTP automation actions like `POST /click` do not go through the native event queue, so they can update UI state without increasing `event_batches.batch_count`
- sub-millisecond values are preserved, so very fast actions may legitimately appear as fractions like `0.18`

### `POST /perf/measure-action`

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

### `POST /perf/measure-native-action`

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

The response includes:

- `elapsed_ms`
- `condition_met`
- `frame_delta`
- `perf`
- `final_state`

This makes it possible to compare:

- raw automation command cost
- number of frames produced while the UI settles
- current render timings at the moment the condition is met

## PowerShell Examples

### Query native state

```powershell
Invoke-RestMethod -Method Get -Uri http://127.0.0.1:47831/native-state
```

### Prepare the window

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:47831/prepare-window `
  -ContentType 'application/json' `
  -Body '{"restore_window":true,"clamp_to_work_area":true,"bring_to_foreground":true}'
```

### Save the best inspection frame

```powershell
Invoke-WebRequest `
  -Method Post `
  -Uri http://127.0.0.1:47831/inspect-frame `
  -OutFile .\inspect-frame.png
```

### Inspect the chosen frame source

```powershell
$resp = Invoke-WebRequest -Method Post -Uri http://127.0.0.1:47831/inspect-frame
$resp.Headers['X-POEM-Frame-Source']
$resp.Headers['X-POEM-Window-Foreground']
```

### Capture a perf snapshot

```powershell
Invoke-RestMethod -Method Get -Uri http://127.0.0.1:47831/perf/state
```

### Measure a UI action until a condition is visible

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:47831/perf/measure-action `
  -ContentType 'application/json' `
  -Body '{"command":"click","id":"wf_z-image-turbo-multi-lora_yaml","timeout_ms":8000,"poll_interval_ms":20,"condition":{"component_id":"selected_workflow","contains_text":"z-image-turbo-multi-lora.yaml"}}'
```

### Measure a native maximize action

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:47831/perf/measure-native-action `
  -ContentType 'application/json' `
  -Body '{"maximize_window":true,"bring_to_foreground":true,"timeout_ms":8000,"poll_interval_ms":20,"condition":{"window_foreground":true,"window_not_minimized":true,"client_width_at_least":1400}}'
```

## Testing And Validation

Relevant tests live in:

- `pkg/render/automation_test.go`
- `pkg/render/protocol/codec_test.go`

Recommended validation commands:

```powershell
go test ./pkg/render/...
cmake --build cpp_sidecar/build --config Release
```

When changing protocol or sidecar behavior, validate both Go and native sides together.

## Troubleshooting

### The app is rendering but `/desktop-frame` is wrong

This often means the app is backgrounded or occluded. Compare:

- `GET /native-state`
- `GET /self-frame`
- `GET /desktop-frame`

### `inspect-frame` keeps returning `self`

That usually means at least one of these is false:

- `window_visible`
- `!window_minimized`
- `window_foreground`

### The app launches but HTTP is unreachable

Check:

- that automation is enabled in `render.AutomationConfig`
- the host and port
- whether the app actually stayed alive after startup
- whether the sidecar path resolved correctly

### The sidecar does not rebuild

The binary is often still in use by a running POEM app. Stop the downstream app and `poem_cpp_sidecar.exe`, then rebuild.

## Documentation Maintenance

If you change:

- `AutomationConfig`
- endpoint names
- request/response shapes
- native-state fields
- inspection fallback rules
- protocol flags for native debug

update this file in the same change.
