# POEM Automation Reference

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
- `POST /focus`
  - body: `{"id":"component_id"}`
- `POST /set-text`
  - body: `{"id":"component_id","value":"new text"}`
- `POST /press-key`
  - body: `{"key":"Enter"}`

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
- `POST /inspect-frame`
  - shared "best inspection frame" endpoint for agents and tests

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

Default behavior:

```json
{
  "restore_window": true,
  "clamp_to_work_area": true,
  "bring_to_foreground": true
}
```

Request fields:

- `restore_window`
  - restore a minimized or hidden window
- `clamp_to_work_area`
  - keep the window within the current monitor work area
- `bring_to_foreground`
  - request foreground activation

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
