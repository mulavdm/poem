---
type: Concept
title: Zero-GC Rendering & Threading Breakthroughs
description: Async repaint decoupling, the zero-allocation draw command buffer, and high-precision delta physics.
tags: [architecture, performance, gc, threading, win32]
timestamp: 2026-07-10T00:00:00Z
---
# Phase 3 Performance Breakthroughs & Zero-GC Rendering

Phase 3 identified and eliminated low-level threading and memory bottlenecks, unlocking rock-solid 60Hz+ performance on both CPU and GPU backends.

## Asynchronous Thread Repaints (`PostMessage` Dummy Wake-up Loop)

To drive real-time background animations, a background thread runs an update ticker at 60Hz. Calling `win32.UpdateWindow(hwnd)` from this background thread forced a synchronous inter-thread `SendMessage` to the main UI thread, blocking the background ticker until the main thread finished rendering and dropping the tick rate to 40Hz.

**The Solution**: the threads were decoupled by updating `triggerRepaint` to perform an asynchronous `InvalidateRect` (marking the window dirty) followed by a native `PostMessage(hwnd, WM_NULL, 0, 0)` call.

**How it works**: `PostMessage` places the dummy `WM_NULL` message in the main thread's queue asynchronously and returns instantly. The main thread's `GetMessage` loop wakes up immediately, processes and ignores the `WM_NULL` message, and then — seeing the dirty update region — natively generates a high-priority `WM_PAINT` cycle. This yields perfect, unblocked asynchronous execution.

## Zero-GC Allocation Draw Command Buffer

Building declarative UI lists and real-time telemetry graphs requires clearing and rebuilding hundreds of components and draw calls every frame. Allocating fresh slices for paint instructions on every frame would thrash Go's Garbage Collector.

**The Solution**: a slice-retaining strategy inside `ProtocolPainter`:

```go
func (p *ProtocolPainter) Reset() {
    p.commands = p.commands[:0]
    ...
}
```

Calling `commands[:0]` resets the length of the command slice to zero while preserving the underlying allocated backing array capacity. Subsequent appends copy drawing commands into retained memory, keeping redraw ticks low-allocation and predictable.

## High-Precision Floating-Point Delta Physics

Unlocking unthrottled 60Hz+ performance dropped the frame delta `dt` to exactly `0.016` seconds. Because particle coordinates were stored as integers, calculating steps like `int(velocity * dt)` (e.g. `int(40 * 0.016) = int(0.64) = 0`) truncated the step size to exactly `0` every frame, causing background particles to freeze solid as soon as performance became perfect.

**The Solution**: `particles.go` was refactored to store and accumulate all coordinates, velocities, and physics steps using high-precision `float64` variables. The values are only cast to integers at the final drawing phase, ensuring fluid animations at any framerate.

## See also
- [Performance Benchmarks](/concepts/architecture/performance-benchmarks.md)
- [Architecture Overview](/concepts/architecture/overview.md)
