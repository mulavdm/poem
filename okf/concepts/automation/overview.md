---
type: Concept
title: Automation Layer Overview
description: Goals, configuration, and the transport model of POEM's shared HTTP automation/inspection layer.
tags: [automation, http, config, transport]
timestamp: 2026-07-23T00:00:00Z
---
# POEM Automation Reference

This is the source of truth for POEM's automation and inspection system. It is written for both humans operating or debugging downstream POEM apps, and agents that need a reliable way to inspect, drive, and reason about native POEM windows.

The automation layer is implemented inside `pkg/render` and is shared by all downstream apps that enable it through `render.AutomationConfig`.

POEM 2.0 component snapshots additionally expose platform-neutral semantic `role`, `name`, `value`, `description`, state flags, and supported `actions`. Stable IDs remain supported and are the preferred selector when known. Interaction commands may instead provide a semantic selector. Selectors must resolve to exactly one node; zero matches and ambiguous matches are errors.

## Goals

POEM automation exists to solve three related problems:

1. **Drive UI state** without writing per-app debug code.
2. **Inspect component trees** and focus state from outside the app process.
3. **Capture frames correctly**, while explicitly distinguishing between:
   - what the app is rendering internally
   - what its own window is presenting
   - what is actually visible on the user's desktop

That third distinction is critical. A native app can be alive and rendering correctly while still being occluded, backgrounded, or partially off-screen — see [Capture Modes](/concepts/automation/capture-modes.md).

## Platform support

The automation layer is plain Go in `pkg/render` with no build tags, and every
host reaches it through the same `RunHosted` path, so **it runs on Android as
well as Windows**. Both presenters now implement the native debug channel
(protocol messages 201/202); the remaining difference is frame capture.

| Surface | Windows | Android |
|---|---|---|
| `/state`, `/components`, `/click`, `/focus`, `/set-text`, `/press-key` | yes | yes |
| `/frame` (Go reference rasterizer) | yes | yes |
| `/perf/state`, `/perf/events`, `/perf/reset` | yes | yes |
| `/native-state` | yes | yes |
| `/perf/native` | yes | yes |
| `/native-frame`, `/self-frame`, `/window-frame` | yes | yes |
| `/desktop-frame` | yes | no (needs MediaProjection consent) |
| `/prepare-window` | yes | no-op (no window manager) |

Capture works on both platforms and returns the same top-down RGBA, so a caller
does not branch on the host. Android serves it by queueing the request from the
transport thread to the render thread, where the GL context is current; see
[Android Presenter](/concepts/architecture/android-presenter.md) for the
mechanism and its bounded wait. `/native-frame`, `/self-frame`, and
`/window-frame` are the same readback there, because the surface is the window.
Only `/desktop-frame` is unavailable, needing MediaProjection consent that an
automated scenario cannot answer.

Uniformity is the point. The alternative — refusing capture and telling callers
to run `adb shell screencap` — put the work out of band in every caller, could
not be driven by a scenario, and captured the screen rather than the
presenter's own output.

On Android `/native-state` reports the surface as the window — there is no
separate window rect — and density where Windows reports DPI.

### Enabling it on Android

NativeActivity has no command line to carry a launch flag, so the gate is a
system property read by `pkg/mobile` at package init and translated into the
same `POEM_INSPECTION` environment contract every platform uses. It must be
read on the Go side: the Go runtime snapshots the environment at init, so a
`setenv()` from the C++ host afterwards is invisible to `os.Getenv`.

```bash
adb shell setprop debug.poem.inspection 1
adb shell am force-stop <package> && adb shell am start -n <package>/android.app.NativeActivity
adb forward tcp:47831 tcp:47831
```

The property is read once at process start, so relaunch the activity after
setting it — which is why the order above is setprop, force-stop, then start.

Those commands are the manual form. A scenario driven by `cmd/poemdrive`
declares a `launch` block instead and the tool performs all of it: it boots the
configured AVD when no device is attached, builds and installs the APK, sets
the property, restarts the activity, and forwards the port. That turns an
Android run into one command from a cold machine rather than a preamble a human
has to remember, and a preamble nobody can forget is the difference between a
scenario that is a fixture and one that is a suggestion. See
[Performance And Latency Profiling](/concepts/automation/perf-profiling.md).

`debug.poem.inspection.port` overrides the port. The surface can
click, type, and read the component tree, so it stays off unless asked for and
binds loopback only — on a device that still means any local process can reach
it, so enable it on development builds only.

Applications opt in by calling `render.InspectionAutomationConfig(os.Getenv)`
and assigning the result to `AppConfig.Automation`; a nil result means
inspection was not requested.

## Configuration

Automation is enabled through `render.AppConfig.Automation`.

```go
config := render.AppConfig{
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
}
poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "Example.App", Title: config.Title, Width: config.Width, Height: config.Height})
```

### `AutomationConfig`

- `Enabled bool` — turns automation on
- `Mode string` — recommended value: `http`; optional legacy value: `pipe`
- `Host string` — default: `127.0.0.1`
- `Port int` — default: `47831`
- `PipeName string` — only used in pipe mode
- `CaptureDir string` — base directory for disk-backed frame captures
- `Verbose bool` — logs automation server startup details

## Transport Model

POEM's automation system has two layers:

1. **Transport-agnostic automation core**
   - component snapshots
   - click/focus/set-text/press-key
   - frame serialization
   - native debug / native capture requests
2. **Transport adapters**
   - HTTP
   - optional named pipe legacy transport

HTTP is the recommended path because it works well across shell contexts, test runners, and agent-driven inspection workflows. All endpoints bind only to localhost when HTTP automation is enabled.

## See also
- [HTTP Endpoints](/concepts/automation/endpoints.md)
- [Capture Modes](/concepts/automation/capture-modes.md)
- [Inspect Flow](/concepts/automation/inspect-flow.md)
- [Native Presenter Protocol](/concepts/protocol.md)

# Citations
- [README.md](file:///d:/Programming/GUIProject/POEM/README.md) — Automation
- [AGENTS.md](file:///d:/Programming/GUIProject/POEM/AGENTS.md) — Automation Rules
