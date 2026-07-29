---
type: Concept
title: Automation HTTP Endpoints
description: Full reference for state, interaction, capture, native-state, and window-control endpoints.
tags: [automation, http, api-reference]
timestamp: 2026-07-10T00:00:00Z
---
# HTTP Endpoints

## State and components

- `GET /health` — never blocks on `stateMutex`, the lock every interactive command and `/state`/`/components` require; reports `{"ok":true,"state_locked":bool,"frame_count":uint64}`. `state_locked:true` means some other caller is currently holding that lock — usually the render/frame loop mid-frame, occasionally a wedged one. Use this to tell "the app is momentarily busy" apart from "the app is actually stuck", which `/frame` alone cannot do: it reads the last-produced frame through separate synchronization and keeps answering even while `stateMutex` (and therefore every interactive command) is stuck. See [Interaction commands](#interaction-commands) below.
- `GET /state` — returns current page, focused ID, hovered ID, and logical/physical window size
- `GET /components` — returns both hierarchical and flat component snapshots

## Interaction commands

- `POST /click` — body: `{"id":"component_id"}`; semantic alternative: `{"selector":{"role":"button","name":"Publish","states":{"enabled":true}}}`
- `POST /focus` — body: `{"id":"component_id"}`
- `POST /set-text`
- `POST /select-text` with `{ "id": "field", "start": 0, "end": 4 }`
- `POST /composition-start`, `/composition-update`, and `/composition-end` — body: `{"id":"component_id","value":"new text"}`
- `POST /press-key` — body: `{"key":"Enter"}`; named keys include `Tab`, `Shift+Tab`, `Enter`, `Space`, `Backspace`, `Delete`, `Escape`, the four arrow keys, `Home`, `End`, `Page Up`, and `Page Down`

`click`, `focus`, `set-text`, `select-text`, and composition commands accept either `id` or `selector`, never both. Selector `role` and accessible `name` use exact case-insensitive matching. The `states` map accepts `enabled`, `disabled`, `focused`, `selected`, `checked`, `expanded`, `read_only`, `required`, `invalid`, `password`, and `offscreen`, with either `true` or `false` values. Successful targeted commands return the resolved stable ID as `target_id`. This keeps semantic scripts readable without sacrificing stable ID diagnostics.

HTTP JSON request bodies are capped at 1 MiB. Selector text, state counts, and IDs have tighter limits; unknown states, empty selectors, dangling semantic trees, and ambiguous matches fail without performing an action.

Every command above, plus `/state` and `/components`, waits up to `automationLockTimeout` (5s) to acquire `stateMutex` before failing with an explicit `"automation surface busy"` error rather than hanging indefinitely behind a wedged owner (typically the render/frame loop, which holds the same lock). Check `GET /health` when a command fails this way, or seems to be taking unusually long, to confirm whether the lock is actually held.

## Frame capture endpoints

- `POST /capture-frame` — writes a PNG to disk; body optional: `{"path":"custom.png"}`
- `GET /frame` — returns a PNG built from POEM's last logical render frame

## Native state and native capture endpoints

- `GET /native-state` — returns native window, DPI, work-area, client-area, and backbuffer state
- `GET /self-frame` — returns the app's own internal render output
- `GET /window-frame` — returns a client-area capture from the app window itself
- `GET /desktop-frame` — returns the actual desktop pixels under the app's client-area screen region

See [Capture Modes](/concepts/automation/capture-modes.md) for how to interpret these three frame sources, and [Native State Fields](/concepts/automation/capture-modes.md#native-state-fields) for the full `native-state` field list.

## Shared window-control and inspection endpoints

- `POST /prepare-window` — shared helper for restore/clamp/foreground; request body may also include `maximize_window`
- `POST /resize` — body: `{"width":1280,"height":720}`, both required and positive; resizes the window's client area (not the outer window rect, so callers do not need to account for title bar/border chrome) and returns the same native-state shape as `/prepare-window`. Crossing a responsive breakpoint is the primary use — see [Perf Profiling](/concepts/automation/perf-profiling.md) for the `resize` scenario step
- `POST /inspect-frame` — shared "best inspection frame" endpoint for agents and tests

See [Inspect Flow](/concepts/automation/inspect-flow.md) for full request/response details on both endpoints.

## Performance endpoints

See [Perf Profiling](/concepts/automation/perf-profiling.md) for `/perf/state`, `/perf/events`, `/perf/native`, `/perf/reset`, `/perf/measure-action`, and `/perf/measure-native-action`.

## PowerShell examples

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

## See also
- [Automation Overview](/concepts/automation/overview.md)
- [Capture Modes](/concepts/automation/capture-modes.md)
- [Inspect Flow](/concepts/automation/inspect-flow.md)
