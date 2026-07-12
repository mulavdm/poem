---
type: Concept
title: FlexBox Layout Control
description: POEM's multi-axis grid positioning system and its relationship to the native measurement contract.
tags: [layout, flexbox, api, usage]
timestamp: 2026-07-10T00:00:00Z
---
# Layout Control using FlexBox

POEM features a fully responsive multi-axis grid positioning system mirroring FlexBox. Instead of hardcoding absolute pixel offsets, elements are wrapped in a `render.FlexBox` container:

```go
&render.FlexBox{
    CompID:         "horizontal_button_bar",
    Rect:           image.Rect(100, 420, 600, 480),
    Direction:      render.Horizontal,          // render.Vertical or render.Horizontal
    AlignItems:     render.AlignCenter,         // AlignStart, Center, End, Stretch
    JustifyContent: render.JustifySpaceBetween, // JustifyStart, Center, End, SpaceBetween
    Padding:        10,
    Gap:            15, // Space between elements
    Children: []render.Component{
        &render.Button{CompID: "btn_a", Rect: image.Rect(0,0,100,40), Text: "Button A"},
        &render.Button{CompID: "btn_b", Rect: image.Rect(0,0,100,40), Text: "Button B"},
    },
}
```

POEM `FlexBox` is not browser-grade intrinsic flexbox. It is a native layout helper where children still contribute preferred sizing information and the parent places them. See [Layout Measurement](/concepts/architecture/layout-measurement.md) for the native `Measure(...)`/`ContentSize(...)` contract that `FlexBox` consumes, and [Cursor and Layout Sync](/concepts/architecture/cursor-and-layout-sync.md) for how bounds are kept current before hit-testing.

**Systematic Layout Rule**: never use hardcoded visual offsets or ad-hoc coordinate bypasses to fix visual text or container clipping. Position and baseline issues must be resolved systematically within layout components (like `FlexBox` or `Grid`) or component-level metric calculations, and parent container dimensions must be properly sized to fit their contents.

## See also
- [Layout Measurement](/concepts/architecture/layout-measurement.md)
- [Component Catalog](/concepts/architecture/component-catalog.md)

# Citations
- [README.md](../../../README.md) — Layout Measurement
