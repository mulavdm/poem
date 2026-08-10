# The Three Capture Modes

## 1. `self-frame`

`GET /self-frame`

This captures the app's own internal render output from the POEM/native rendering path.

Use it when you want to know:

- whether the app itself is rendering the right content
- whether a layout bug is inside POEM/app logic instead of desktop/window visibility
- what the app looks like even if it is backgrounded or occluded

This does **not** prove the user can actually see that frame on their desktop.

## 2. `window-frame`

`GET /window-frame`

This captures the app window's client-area presentation.

Use it when you want a window-owned view that is closer to native presentation than the logical backbuffer capture.

This is useful, but it is still not the same thing as the literal desktop result in every occlusion scenario.

## 3. `desktop-frame`

`GET /desktop-frame`

This captures the desktop surface currently occupying the app's client-area screen region.

Use it when you want to know:

- what is literally visible on the desktop right now
- whether another window is covering the app
- whether the app is on-screen in the way the user experiences it

If the app is behind another window, `desktop-frame` will show the other window.

## Why the distinction matters

These three cases can all happen:

1. `self-frame` looks correct, but `desktop-frame` shows another app
2. the app is visible but not foregrounded, so `desktop-frame` may be unreliable for desktop-facing inspection
3. the app is minimized or off-screen, so internal render health and desktop visibility diverge completely

This is why agents should not treat a single screenshot path as universal truth.

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

## See also
- [Automation Overview](../automation/TDD.md)
- [HTTP Endpoints](../automation/endpoints.md)
- [Inspect Flow](../automation/inspect-flow.md)

# Citations
- [AGENTS.md](file:///d:/Programming/GUIProject/POEM/AGENTS.md) — Common Gotchas
