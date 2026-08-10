# POEM Architecture Overview

## POEM 2.0 portability boundary

The shipping runtime remains Win32/D3D11. Platform-neutral packages under `pkg/render` own themes, drawing values, events, semantic accessibility, layout, and optional platform-service interfaces. Native presentation, text services, window management, and Windows UI Automation remain behind the C++ host boundary. M7 places that host and the compiled Go application/engine DLL in one OS process while retaining the repo-owned protocol over in-memory transport.

Historical note: the former Go executable plus C++ child-sidecar runtime, named renderer pipes, embedded presenter extraction, and sidecar override path were removed in M7. The legacy Rust presenter remains reference-only.

`runtime.LockOSThread()` binds native window/event-loop goroutines to a single OS thread for their lifetime — see [Thread Locking](../architecture/thread-locking.md) for why this is load-bearing, not optional.

## Architectural Evolution: The Modular Polylith

`pkg/render` is structured as an idiomatic modular Polylith: instead of a flat list of disparate files, components are partitioned into clean, acyclic subpackages.

```
                  +--------------------------------+
                  |           pkg/render           | <----+ (Single import path for consumers)
                  +--------------------------------+      |
                     /           |            \           | (Exposes hosted engine and
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
  - **`run.go`**: the Inversion-of-Control (IoC) launcher. Locks the thread, creates the window, binds events, and drives the frame tickers — see [Reactive Loop and IoC](../architecture/reactive-loop-and-ioc.md).
  - **`types/`**: core API definitions, `Painter`, `UIRenderer`, `Component` contracts, platform-neutral render context, overlay state, and semantic metadata.
  - **`components/`**: pure declarative interactive controls — see [Component Catalog](../architecture/component-catalog.md).
  - **`layout/`**: axis-alignment flexbox positioning logic (`FlexBox`) — see [FlexBox Layout](../architecture/flexbox-layout.md).
  - **`backend/`**: hardware and software rendering engines (`cpu.go`, `gpu.go`) swapped compile-time using build tags.

## See also
- [Public API Surface](../../reference/public-api.md)
- [Single-Process Windows Host](../architecture/windows-host.md)
- [Sidecar Protocol](../../reference/protocol.md)
- [Thread Locking](../architecture/thread-locking.md)
- [Win32 Syscall Stabilization](../architecture/win32-syscalls.md)
- [Process Isolation Comparison](../architecture/process-isolation-comparison.md)

# Citations
- [README.md](file:///d:/Programming/GUIProject/POEM/README.md)
