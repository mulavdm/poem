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

## Anti-pattern: layout output mistaken for author intent (`UI101`)

A `Measure(...)` implementation must derive its preferred size from content and available space, **never from its own laid-out `Rect`**. Feeding the assigned bounds back as an explicit size request freezes a component at whatever size it was first laid out at: after a resize it reports the *old* size, so it truncates its label or fails to track the window. `Button` and `Badge` had this defect (they read `explicitSize(Rect)`) and now size from label content — `Button` carries a separate `FixedWidth` for genuine author sizing, kept distinct from the laid-out bounds. The map viewport (`ImageViewportNode`) had the same defect and now fills its available space, so it responds to window resizes and keeps its edge-anchored overlay controls on-screen. The design guide records this as `UI101`; a canvas that hosts corner controls must fit its viewport for those controls to stay reachable.

**Systematic Layout Rule**: never use hardcoded visual offsets or ad-hoc coordinate bypasses to fix visual text or container clipping. Position and baseline issues must be resolved systematically within the rendering engine's layout components (like `FlexBox` or `Grid`) or component-level metric calculations, and parent container dimensions must be properly sized to fit their contents.

## See also
- [FlexBox Layout](../architecture/flexbox-layout.md)
- [Cursor and Layout Sync](../architecture/cursor-and-layout-sync.md)
- [Scroll Viewports](../architecture/scroll-viewports.md)

# Citations
- [README.md](file:///d:/Programming/GUIProject/POEM/README.md) — Layout Measurement
- [AGENTS.md](file:///d:/Programming/GUIProject/POEM/AGENTS.md) — Systematic Layout Design
