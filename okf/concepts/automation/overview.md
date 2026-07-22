---
type: Concept
title: Automation Layer Overview
description: Goals, configuration, and the transport model of POEM's shared HTTP automation/inspection layer.
tags: [automation, http, config, transport]
timestamp: 2026-07-10T00:00:00Z
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
