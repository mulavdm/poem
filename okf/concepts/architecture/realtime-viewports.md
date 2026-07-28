---
type: Concept
title: Native Real-Time Viewports
description: The versioned native boundary for embedding real-time renderers in POEM.
tags: [windows, native, viewport, d3d12, accessibility]
timestamp: 2026-07-26T00:00:00Z
---
# Native Real-Time Viewports

`shared/poem/realtime_viewport.h` defines the renderer-neutral native contract
used to embed a real-time renderer without making its game types part of POEM.
Every structure carries a byte size and every export table carries an ABI
version. ABI v2 adds semantic action events for left, right, confirm, and
cancel; the host maps platform keys before crossing the boundary. Missing
callbacks, incompatible versions, oversized semantic payloads,
and inconsistent pointer/count pairs fail validation before invocation.

The host owns the window, input routing, GPU device, command queue, command
list, render targets, device-loss policy, composition, and final presentation.
Objects in `FrameInput` are borrowed for one callback and must not be retained.
Shutdown is explicit and occurs only after callbacks have quiesced.

The viewport publishes a bounded semantic snapshot rather than exposing native
objects to accessibility or automation code. POEM converts that snapshot into
its ordinary semantic UI and inspection surfaces.

`components.RealtimeViewport` implements the ordinary wheel-input component
contract. When the pointer is inside its bounds, it forwards a non-zero native
delta (120 per Windows wheel notch) to its optional `OnWheel` callback. The
component does not interpret the delta, so orbit, zoom, scrub, and other
renderer-specific behavior remains owned by the embedding application.

For native views that need distinct navigation gestures,
`RealtimeViewport.OnPointerButton` receives the same normalized pointer phases
with the host button code (primary, secondary, or middle). It takes precedence
over the legacy button-agnostic callback, preserving compatibility while
keeping gesture interpretation in the embedding application.

The existing Windows D3D11 presenter remains the default and does not load this
contract. `POEM_REALTIME_VIEWPORT_DLL` is the explicit Windows opt-in. It loads
the module, validates `HamsterRealtimeViewportExports`, selects a hardware
adapter, creates the D3D12 device/queue/swap chain, owns transitions and fences,
and invokes the viewport between render-target transitions. Resize drains the
queue, recreates swap-chain targets, and notifies the viewport.

The opt-in currently proves hosting rather than full UI composition: ordinary
POEM frame geometry is not replayed over the D3D12 target and native D3D12
backbuffer capture returns unavailable. Invalid explicit configuration fails
startup rather than falling back to D3D11 and hiding a packaging fault.
