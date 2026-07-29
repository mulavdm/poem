---
type: Concept
title: Automation Troubleshooting & Validation
description: Common automation failure modes and the required test/validation commands.
tags: [automation, troubleshooting, testing]
timestamp: 2026-07-10T00:00:00Z
---
# Troubleshooting

## The app is rendering but `/desktop-frame` is wrong

This often means the app is backgrounded or occluded. Compare:

- `GET /native-state`
- `GET /self-frame`
- `GET /desktop-frame`

## `inspect-frame` keeps returning `self`

That usually means at least one of these is false:

- `window_visible`
- `!window_minimized`
- `window_foreground`

## The app launches but HTTP is unreachable

Check:

- that automation is enabled in `render.AutomationConfig`
- the host and port
- whether the app actually stayed alive after startup
- whether the adjacent `poem_app.dll` exists and exports ABI v1

## An interactive command hangs while `/frame` keeps answering

`/frame` reads the last-produced frame through its own separate synchronization, independent of `stateMutex` — a clean `/frame` response is not proof the app is healthy. Check `GET /health`: `state_locked:true` means `stateMutex` is genuinely held (likely the render/frame loop stuck mid-frame), not just contended. An interactive command that has been failing with `"automation surface busy"` for longer than a moment, rather than actually hanging, confirms the same thing — `automationLockTimeout` (5s) bounds the wait so a stuck lock now fails fast instead of hanging indefinitely. See [Endpoints: State and components](/concepts/automation/endpoints.md#state-and-components).

## The Windows host does not rebuild

The binary is often still in use by a running POEM app. Stop the product-named host process, then rebuild `poem_windows_host`.

# Testing And Validation

Relevant tests live in:

- `pkg/render/automation_test.go`
- `pkg/render/protocol/codec_test.go`

Recommended validation commands:

```powershell
go test ./pkg/render/...
cmake --build cpp_sidecar/build --config Release
```

When changing protocol or native-host behavior, validate both Go and C++ sides together, including CTest and a c-shared DLL smoke test.

# Documentation Maintenance

If you change:

- `AutomationConfig`
- endpoint names
- request/response shapes
- native-state fields
- inspection fallback rules
- protocol flags for native debug

update the corresponding `okf/concepts/automation/` concept in the same change.

## See also
- [Automation Overview](/concepts/automation/overview.md)
- [Native Presenter Protocol](/concepts/protocol.md)
