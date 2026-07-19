---
type: Concept
title: Composable Scroll Viewports
description: How render.ScrollView clips, translates coordinates, and captures scrollbar drags, and how to compose one.
tags: [architecture, scroll, clipping, d3d11, hit-testing]
timestamp: 2026-07-10T00:00:00Z
---
# Scroll Viewports & Coordinate Translation Engine

To support large lists, telemetry streams, and logging consoles, POEM integrates vertical scroll viewports (`render.ScrollView`) and dual-backend clipping masks.

## Viewport Bounded Clipping (D3D11 Scissor Tests)

To render items inside a scroll container without them bleeding onto stationary UI elements, POEM adds a native viewport clipping bounding box to the drawing tree:

- **Native GPU Scissor Tests**: the active Windows host executes hardware scissor tests in D3D11 render passes.
- **Aspect Scaling Conversion**: because the native renderer operates on physical pixels, it converts the logical scissor bounds requested by Go into physical pixels using the current display DPI scale factor. The calculated bounds are safely clamped to avoid exceeding swapchain sizes, providing zero-overhead, anti-aliased sub-frame viewport clipping.

Elements outside the `ScrollView` `Rect` are automatically clipped at the GPU level inside the native host.

## Relative Coordinate Translation

Hit-testing and event dispatching (mouse clicks, dragging, hovers) normally operate on absolute screen coordinates. If a child component is scrolled up by `ScrollY`, it is drawn at screen coordinate `Y - ScrollY`.

To handle this transparently, `ScrollView` recursively translates the input coordinates for all hit-test and mouse event dispatching loops:

$$\text{scrolledPt} = \text{screenPt} + (0, \text{ScrollY})$$

This translates coordinates perfectly before forwarding events, allowing standard buttons, inputs, sliders, and hovers to remain fully operational when scrolled. In code: `scrolledPt = pt.Add(image.Point{0, s.ScrollY})`.

## Captured Drag Capturing

To prevent scrollbar dragging from stuttering when the user moves the mouse rapidly outside the scroll track, POEM locks the scroll captures using Win32 capture handles (`SetCapture`, `ReleaseCapture`).

When the scrollbar thumb is clicked, the `ScrollView` ID becomes the `ActiveID`. Subsequent mouse movements, even those moving outside the client window, continue to route raw offset deltas directly to the active `ScrollView`, ensuring a silky-smooth, glitch-free dragging feedback loop.

## Usage: composing a scroll view

Wrap a vertical `render.FlexBox` container inside a `render.ScrollView` and place as many interactive components (labels, buttons, text fields) as needed inside:

```go
&render.ScrollView{
    CompID: "my_diagnostics_scroll",
    Rect:   image.Rect(100, 150, 500, 650), // Fixed viewport bounds (400x500)
    Children: []render.Component{
        &render.FlexBox{
            CompID:    "inner_scroll_content",
            Direction: render.Vertical,
            Padding:   15,
            Gap:       15,
            Children: []render.Component{
                &render.Label{CompID: "lbl_header", Text: "MASSIVE LIST CONTENT"},
                &render.Button{CompID: "scroll_btn_1", Rect: image.Rect(0, 0, 150, 40), Text: "CLICK ME"},
                &render.TextInput{CompID: "scroll_txt_1", Rect: image.Rect(0, 0, 150, 45), Placeholder: "Type here..."},
                // Add as many children as needed...
            },
        },
    },
}
```

Downstream developers get fully functional, hover-reactive, and click-sensitive components out-of-the-box inside scrolling panels — coordinate translation is handled transparently.

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Cursor and Layout Sync](/concepts/architecture/cursor-and-layout-sync.md)
- [DPI Coordinate Translation](/concepts/architecture/dpi-coordinate-translation.md)
