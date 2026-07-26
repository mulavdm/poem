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
version. Missing callbacks, incompatible versions, oversized semantic payloads,
and inconsistent pointer/count pairs fail validation before invocation.

The host owns the window, input routing, GPU device, command queue, command
list, render targets, device-loss policy, composition, and final presentation.
Objects in `FrameInput` are borrowed for one callback and must not be retained.
Shutdown is explicit and occurs only after callbacks have quiesced.

The viewport publishes a bounded semantic snapshot rather than exposing native
objects to accessibility or automation code. POEM converts that snapshot into
its ordinary semantic UI and inspection surfaces.

The existing Windows D3D11 presenter remains the default and does not load this
contract. D3D12 viewport composition is additive work: the contract and tests
land first, followed by the shared D3D12 presenter and application opt-in. This
prevents an unfinished renderer from silently changing established apps.

