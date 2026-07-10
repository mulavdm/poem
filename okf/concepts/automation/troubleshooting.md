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
- whether the sidecar path resolved correctly

## The sidecar does not rebuild

The binary is often still in use by a running POEM app. Stop the downstream app and `poem_cpp_sidecar.exe`, then rebuild.

# Testing And Validation

Relevant tests live in:

- `pkg/render/automation_test.go`
- `pkg/render/protocol/codec_test.go`

Recommended validation commands:

```powershell
go test ./pkg/render/...
cmake --build cpp_sidecar/build --config Release
```

When changing protocol or sidecar behavior, validate both Go and native sides together.

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
- [Sidecar Protocol](/concepts/protocol.md)
