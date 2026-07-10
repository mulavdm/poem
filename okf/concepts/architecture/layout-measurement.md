---
type: Concept
title: Layout Measurement Contract
description: The native Measure()/ContentSize() contract that lets FlexBox and ScrollView size children without browser-style intrinsic layout.
tags: [layout, measurement, api, migration]
timestamp: 2026-07-10T00:00:00Z
---
# Layout Measurement Quickstart

POEM's layout model is intentionally lighter than browser-grade CSS flexbox.

Today:

- parent layouts still place children explicitly
- leaf components can opt into a native `Measure(...)` contract
- `FlexBox` prefers measured sizes when available and falls back to legacy `Bounds()` sizing otherwise
- containers can optionally expose `ContentSize(...)` when their scrollable content is larger than their assigned draw bounds

This means downstream apps should not assume "declare children and forget it" browser behavior yet.

## Native measurement contract

Components can optionally implement:

```go
Measure(avail image.Point, state *render.ApplicationState) render.MeasureResult
```

Where `MeasureResult` provides:

- `Preferred image.Point`
- `Min image.Point`

This is a POEM-native sizing contract, not a CSS model. It does not currently introduce concepts like percentages, flex-basis, or browser intrinsic content negotiation.

Scrollable containers can also consume:

```go
ContentSize(avail image.Point, state *render.ApplicationState) image.Point
```

Use this when a component's assigned `Bounds()` describes the visible box, but its laid-out content is taller or wider. `render.ScrollView` uses this distinction so fixed viewport bounds do not accidentally erase scrollable content.

## For stable layouts

- use explicit pane rects for major dashboard regions
- let leaf controls report preferred size through `Measure(...)`
- wrap overflow regions in `ScrollView`; it uses `ContentSize(...)` where available so fixed viewport bounds do not erase scrollable extent
- avoid using oversized placeholder `Bounds()` as a proxy for wrapped text size
- cap long status text or labels explicitly when the UI has tight horizontal budgets

## Downstream migration checklist

- prefer explicit container rects for major panes
- rely on `Measure(...)` for buttons, labels, inputs, text blocks, image views, and scroll viewports
- rely on `ScrollView` for intentional overflow instead of allowing children to draw through sibling regions
- avoid stale oversized `Bounds()` values as a stand-in for wrapped or clipped content
- cap long status text and chips explicitly when sharing a header row
- preserve explicit component rects when the layout is intentionally fixed

**Systematic Layout Rule**: never use hardcoded visual offsets or ad-hoc coordinate bypasses to fix visual text or container clipping. Position and baseline issues must be resolved systematically within the rendering engine's layout components (like `FlexBox` or `Grid`) or component-level metric calculations, and parent container dimensions must be properly sized to fit their contents.

## See also
- [FlexBox Layout](/concepts/architecture/flexbox-layout.md)
- [Cursor and Layout Sync](/concepts/architecture/cursor-and-layout-sync.md)
- [Scroll Viewports](/concepts/architecture/scroll-viewports.md)

# Citations
- [README.md](../../../README.md) — Layout Measurement
- [AGENTS.md](../../../AGENTS.md) — Systematic Layout Design
