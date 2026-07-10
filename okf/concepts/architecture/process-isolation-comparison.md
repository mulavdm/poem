---
type: Concept
title: Process Isolation Comparison
description: Architectural comparison between the original single-process GDI/OpenGL runtime and the active Go + C++ D3D11 sidecar.
tags: [architecture, comparison, d3d11, ipc]
timestamp: 2026-07-10T00:00:00Z
---
# Architectural Comparison: Single-Process GDI/OpenGL vs. Process-Isolated Native Presentation

Rigorous architectural comparison matrix (porting metrics):

| Architectural Vector | Original Single-Process GDI/OpenGL | Active Go + C++ D3D11 Sidecar |
| :--- | :--- | :--- |
| **Runtime Isolation** | None (Any native crash terminates Go main program) | **Process Isolation** (native presentation sidecar runs separately) |
| **Graphics API** | GDI / OpenGL Core Profile 3.3 | **Direct3D 11** in the active Windows sidecar |
| **Windowing & Input** | Custom Win32 Syscalls (Go thread-locked) | **Win32** in the active Windows sidecar |
| **IPC Strategy** | Direct heap pointer sharing (same process thread) | **Win32 Named Pipes** with a repo-owned custom binary protocol |
| **Acoustic Audio** | Win32 DSP (winmm.dll) in Go | **Native sidecar playback** via Windows multimedia APIs |
| **CGO Required** | Yes (when building `-tags gpu` with `go-gl`) | **No CGO Required** for the active Go + C++ sidecar runtime |
| **High-DPI / 4K Scaling** | None (Microscopic elements, layout coordinate clash) | **Automated Coordinate Translation Subsystem** (Logical vs. Physical) |
| **Frame Telemetry** | CPU bound (~1.1ms), thrashes Go GC on VBO arrays | **Batched protocol frames** with low-allocation Go serialization and native D3D11 presentation |

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Sidecar Protocol](/concepts/protocol.md)
- [DPI Coordinate Translation](/concepts/architecture/dpi-coordinate-translation.md)
