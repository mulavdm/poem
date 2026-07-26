---
type: Concept
title: Native Realtime Viewport
description: Versioned D3D12 viewport composition inside POEM layout.
tags: [windows, d3d12, viewport, abi, accessibility]
timestamp: 2026-07-26T00:00:00Z
---
# Native real-time viewport

The native real-time viewport is a reusable leaf in POEM layout. POEM owns
composition, input routing, accessibility, automation, and presentation. A
plugin receives borrowed D3D12 frame services and a clipped rectangle.

ABI v2 remains the render-only compatibility contract. ABI v3 appends bounded
opaque command submission and caller-buffered event polling without assigning
application meaning to either payload. Structure-size validation permits an
older v2 export table and requires the complete appended table for v3.

Wire protocol v5 carries polled plugin output as a targeted opaque component
event. The D3D12 presenter composites POEM's display list after the borrowed
viewport pass so shell controls and overlays remain POEM-owned.

No editor, scene, entity, or game concept crosses this boundary.
