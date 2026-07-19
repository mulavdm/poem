---
type: Concept
title: High-DPI Coordinate Translation Subsystem
description: How POEM keeps layout in logical coordinates while the in-process D3D11 host presents physical pixels.
tags: [architecture, dpi, coordinates, d3d11, scaling]
timestamp: 2026-07-10T00:00:00Z
---
# High-DPI Coordinate Translation Subsystem

POEM implements a single-process dual-runtime architecture. The Go DLL manages business logic, layout generation, and state metrics while the C++ Win32/D3D11 host presents the repo-owned protocol over in-memory transport. The coordinate contract is unchanged from earlier releases.

## The High-DPI Engineering Challenge

On high-DPI displays (e.g., 4K monitors with 150%–250% system scaling), rendering layouts at a 1:1 pixel ratio causes the entire UI to appear microscopic. Standard operating system window managers automatically scale windows, but low-level hardware-accelerated drawing surfaces must manually adjust their projection and scissor boundaries.

However, forcing the layout engine to compute high-DPI coordinates causes dynamic layout code to become highly complex and prone to visual alignment bugs.

## The Solution: Dual Coordinate Spaces

POEM segregates the framework into two distinct coordinate systems:

1. **Logical Coordinate Space (Go Orchestrator)**: the layout matrix, event boundaries, padding, rounding, and vector coordinates run strictly in a virtual Logical Space (e.g. standard 1024x768 units).
2. **Physical Coordinate Space (Windows D3D11 Host)**: native presentation resources, render targets, and swapchains run in the device's Physical Pixel Space (e.g. 2048x1536 pixels on 200% scaling).

## Dynamic Orthographic Projection Translation

To bridge these coordinate spaces, the host scales its render projection using the current physical size and DPI factor:

```rust
let left = 0.0;
let right = physical_width as f32 / scale_factor;
let bottom = physical_height as f32 / scale_factor;
let top = 0.0;
```

This maps logical layout coordinates into the host's native render space while preserving correct physical sizing.

```
       Go Engine DLL                       Windows D3D11 Host
+----------------------------+      +------------------------------+
| Logical Space (1024x768)   |      | Physical Pixel Space (4K)   |
|                            |      |                              |
| - Layout flex calculations |      | - D3D11 swapchain            |
| - Mouse hit-test loops     |      | - Native render targets      |
| - Padding & SDF bounds     |      | - Scissor clipping rects     |
+----------------------------+      +------------------------------+
              |                                     ^
              | POEM Protocol Frames                | Dynamic Scale
              v                                     | (scale_factor)
      [ DrawCommand(x,y) ] -------------------------+
```

## Physical Presentation Alignment

The Windows host receives logical draw commands from Go and applies the current DPI scale when configuring projection, scissor, and presentation bounds. Using physical swapchain dimensions for native presentation prevents visual stretching, offset shifts, and coordinate mismatches on scaled displays.

## Physical Scissor Clipping Boundaries

Scissor tests (`set_scissor_rect`) operate on physical hardware-level boundaries. If Go requests clipping in a `ScrollView` at logical `x, y, w, h`, these coordinates are scaled up by `scale_factor` in the renderer before applying the GPU scissor:

$$\text{clip}_{\text{physical}} = \text{clip}_{\text{logical}} \cdot \text{scale\_factor}$$

This ensures that scrollviews are cropped perfectly down to the physical pixel boundary, completely resolving scrolling clip-box bugs on high-density displays.

## Bidirectional Event Scaling

To keep the Go orchestrator decoupled from scaling metrics:

- **Cursor Movements**: raw native cursor positions are divided by the active DPI scale before being sent back to Go for precise hit-testing.
- **Scroll Deltas**: pixel-based wheel scroll offsets from `MouseWheel` are divided by `scale_factor` to maintain uniform scrolling sensitivity across all screens.
- **Window Resizes**: physical window resizes are divided by the scale factor, keeping Go aware of the true logical grid bounds.

## Usage: what downstream developers do (nothing)

As a downstream developer, no scaling math, factor multiplication, or resolution adjustments are needed:

- **Pure Logical Layouts**: widgets declared in `BuildPagesFn` (e.g. `image.Rect(100, 100, 300, 400)`) are defined strictly in logical units. POEM's layout flexboxes and margins run in this logical space.
- **Auto-Adjusted Views**: behind the scenes, the presentation engine scales elements to match the screen's system density, preventing them from appearing tiny on high-resolution screens.
- **Integrated Clipping & Interaction**: scissor clips inside `ScrollView` and mouse coordinates (`MouseMove`, `MouseDown`, `MouseWheel`) are automatically converted between physical screen pixels and logical layout space. Custom hover cues, clicks, and scrolling operate cleanly out-of-the-box on any screen size.

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Process Isolation Comparison](/concepts/architecture/process-isolation-comparison.md)
- [Scroll Viewports](/concepts/architecture/scroll-viewports.md)
