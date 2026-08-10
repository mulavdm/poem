# `POST /prepare-window`

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

- `restore_window` — restore a minimized or hidden window
- `clamp_to_work_area` — keep the window within the current monitor work area
- `bring_to_foreground` — request foreground activation
- `maximize_window` — request a native maximize transition

Response: JSON native window state, same shape as `GET /native-state`.

This endpoint is intentionally narrow. It is for safe inspection setup, not for broad desktop automation.

# `POST /inspect-frame`

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

## Request fields

- `prepare_window`
- `restore_window`
- `clamp_to_work_area`
- `bring_to_foreground`
- `prefer_desktop`
- `fallback_to_self`

## Behavior

1. Optionally prepares the window using the shared native helper.
2. Reads native state.
3. If desktop inspection is preferred and the window is visible, not minimized, and foregrounded, then it returns a desktop-visible capture.
4. Otherwise, if fallback is enabled, it returns the app's internal self-frame.

## Response

`POST /inspect-frame` returns `image/png`.

Headers include:

- `X-POEM-Frame-Source` — `desktop` or `self`
- `X-POEM-Window-Visible`
- `X-POEM-Window-Minimized`
- `X-POEM-Window-Foreground`

That lets callers reason about why a given frame source was chosen.

# Recommended Agent Workflow

For general inspection, use this order:

1. Call `POST /inspect-frame`
2. Read `X-POEM-Frame-Source`
3. If source is `desktop`, treat it as desktop-visible truth
4. If source is `self`, treat it as app-render truth, not desktop-visibility proof
5. If needed, call `GET /native-state` for more exact visibility context

## Practical meaning

- `source=desktop` — the app was foreground-ready and the returned image reflects desktop-visible output
- `source=self` — the app is alive and rendering, but POEM did not trust desktop-visible capture conditions enough to use that as the primary result

## See also
- [Capture Modes](../automation/capture-modes.md)
- [Automation Overview](../automation/TDD.md)
- [HTTP Endpoints](../automation/endpoints.md)

# Citations
- [AGENTS.md](file:///d:/Programming/GUIProject/POEM/AGENTS.md) — Automation Rules
