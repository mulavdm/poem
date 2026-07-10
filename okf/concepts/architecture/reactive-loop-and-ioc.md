---
type: Concept
title: Reactive Loop & Inversion of Control
description: The re-evaluation loop that drives page rebuilds, and how render.Run(AppConfig) hides Win32 wiring from consumers.
tags: [architecture, ioc, reactive-loop, public-api]
timestamp: 2026-07-10T00:00:00Z
---
# Core Architecture & Reactive Loop

POEM uses a process-isolated re-evaluation loop driven over local IPC. Instead of manually updating widgets, applications write a Page Builder function.

The lifecycle:

1. The native presentation sidecar captures low-level window interactions and transmits them back to Go.
2. The Go orchestrator processes these events, mutates the global `ApplicationState`, and invokes the Page Builder function to rebuild the component hierarchy from scratch.
3. Go serializes the new drawing commands using the repo-owned binary protocol in `pkg/render/protocol`.
4. The sidecar processes the frame asynchronously and flushes draw calls to the GPU.

```
+---------------------+   Input Event Batch        +-------------------+
| Native Sidecar Core | -------------------------> |  Go Orchestrator  |
| (Win32 / D3D11 now) | <------------------------- |    (Game Loop)    |
+---------------------+   Render / Sound Commands  +-------------------+
                                                             |
                                                             v Triggers
                                                   +-------------------+
                                                   |  BuildPagesFn()   |
                                                   | (Rebuilds UI tree)|
                                                   +-------------------+
```

When application-owned state changes from a goroutine after IO, model work, or a service callback, call `render.RequestRepaint()` after publishing the new state. This marks the current frame dirty without enabling continuous animation and without coupling the application to the Windows backend.

## Design for External Integration (IoC)

The library uses the Inversion of Control (IoC) pattern through `render.Run(AppConfig)`. External consumers do not have to write native Win32 window callbacks, event routers, thread locking, or frame tickers.

To instantiate the UI, host projects import `"go_native_gpu_gui/pkg/render"` and declare their component layout tree inside the `BuildPagesFn` callback, which the library automatically manages and updates dynamically.

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Public API Surface](/concepts/public-api.md)
- [Downstream App Example](/concepts/architecture/downstream-example.md)
