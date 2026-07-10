---
type: Concept
title: POEM Architecture Overview
description: The POEM 2.0 portability boundary and the Go/C++ modular polylith package topology.
tags: [architecture, go, cpp, sidecar, polylith]
timestamp: 2026-07-10T00:00:00Z
---
# POEM Architecture Overview

## POEM 2.0 portability boundary

The shipping runtime remains Win32/D3D11. Platform-neutral packages under `pkg/render` own themes, drawing values, events, semantic accessibility, layout, and optional platform-service interfaces. Native presentation, text services, window management, and the Windows UI Automation provider stay behind the sidecar/platform boundary. Protocol v2 carries semantic snapshots without embedding Windows accessibility concepts in Go components.

Historical note: earlier single-process and Rust-sidecar experiments are superseded. The active runtime is a Go orchestrator plus a Windows-first C++ sidecar using Win32, D3D11, named pipes, and a repo-owned binary protocol.

`runtime.LockOSThread()` binds native window/event-loop goroutines to a single OS thread for their lifetime — see [Thread Locking](/concepts/architecture/thread-locking.md) for why this is load-bearing, not optional.

## Architectural Evolution: The Modular Polylith

`pkg/render` is structured as an idiomatic modular Polylith: instead of a flat list of disparate files, components are partitioned into clean, acyclic subpackages.

```
                  +--------------------------------+
                  |           pkg/render           | <----+ (Single import path for consumers)
                  +--------------------------------+      |
                     /           |            \           | (Exposes Run() and
                    v            v             v          |  type-aliases all symbols)
         +------------+    +------------+    +------------+
         |  backend   |    | components |    |   layout   |
         +------------+    +------------+    +------------+
                    \            |             /
                     v           v            v
                  +--------------------------------+
                  |             types              | (Core APIs, State, Semantics)
                  +--------------------------------+
```

### Realized component topology

- `cmd/engine/`: main event loop and message handling, dogfooding the library.
- `internal/win32/`: low-level, private Win32 OS interaction (completely isolated from UI logic).
- `pkg/render/`: the unified public wrapper package:
  - **`render.go`**: the single import interface. Leverages Go type aliasing to expose subpackage components, constants, and options so that consumers never deal with deep subpackage imports.
  - **`run.go`**: the Inversion-of-Control (IoC) launcher. Locks the thread, creates the window, binds events, and drives the frame tickers — see [Reactive Loop and IoC](/concepts/architecture/reactive-loop-and-ioc.md).
  - **`types/`**: core API definitions, `Painter`, `UIRenderer`, `Component` contracts, platform-neutral render context, overlay state, and semantic metadata.
  - **`components/`**: pure declarative interactive controls — see [Component Catalog](/concepts/architecture/component-catalog.md).
  - **`layout/`**: axis-alignment flexbox positioning logic (`FlexBox`) — see [FlexBox Layout](/concepts/architecture/flexbox-layout.md).
  - **`backend/`**: hardware and software rendering engines (`cpu.go`, `gpu.go`) swapped compile-time using build tags.

## See also
- [Public API Surface](/concepts/public-api.md)
- [Sidecar Protocol](/concepts/protocol.md)
- [Thread Locking](/concepts/architecture/thread-locking.md)
- [Win32 Syscall Stabilization](/concepts/architecture/win32-syscalls.md)
- [Process Isolation Comparison](/concepts/architecture/process-isolation-comparison.md)

# Citations
- [README.md](../../../README.md)
