# State-Driven Dynamic Cursor System

To isolate native Windows syscalls from components, POEM uses a fully state-driven, dynamic mouse cursor subsystem:

- **Global Preloading**: system handles for Arrow (`IDC_ARROW` = 32512), Click-Hand (`IDC_HAND` = 32649), and Text-IBeam (`IDC_IBEAM` = 32513) are cached once at startup.
- **Pipeline Frame Reset**: the orchestrator `RenderPipeline` resets the active cursor to `state.ArrowCursor` at the start of every paint tick.
- **Immediate-Mode Claims**: during `Draw()`, hovering components claim their cursor handles by mutating `state.CursorID`.
- **Win32 Message Hook**: `libWndProc` intercepts the native `WM_SETCURSOR` (0x0020) message. If the mouse is inside the window's client area (`HTCLIENT`), it pushes the active `state.CursorID` handle directly to the OS kernel, allowing instantaneous, low-latency pointer updates.

## Usage: claiming a cursor from a component

The core pipeline automatically resets `state.CursorID` to `state.ArrowCursor` at the start of every frame, allowing components to claim cursor states reactively:

- **Standard Pointers**: default reset inside the coordinate pipelines.
- **Interactive Clicking Hand (`state.HandCursor`)**: set automatically when hovering over `Button` or `Slider` components.
- **Text I-Beam (`state.IBeamCursor`)**: set automatically when hovering over `TextInput` controls.

```go
func (b *MyComponent) Draw(pnt types.Painter, state *types.ApplicationState) {
    if state.HoveredID == b.CompID {
        // Shift mouse pointer to the OS interaction hand!
        state.CursorID = state.HandCursor
    }
}
```

# Recursive Pre-Evaluation Layout Synchronization Pass

A recurring issue in declarative frameworks is temporal layout lag — when component trees rebuild, layout children are initially created with relative coordinates at `(0, 0)`. If hit-testing occurs before they are drawn, hover states fail because bounds have not yet been evaluated by layout formulas.

To solve this, POEM runs a recursive pre-evaluation layout synchronization pass:

```go
for _, comp := range page {
    comp.SetBounds(comp.Bounds())
}
```

This is run automatically right before hit-testing in `RenderPipeline`, `WM_MOUSEMOVE`, and the mouse-click dispatcher. It recursively triggers `performLayout()` on all nested structures (like `FlexBox`), guaranteeing that every element is positioned at its exact, finalized desktop coordinates before interactive mouse hit-tests are computed.

Downstream developers never need to manually align components — nested children bounds are fully updated, responsive, and ready for hover interactions automatically out-of-the-box.

## See also
- [Architecture Overview](TDD.md)
- [FlexBox Layout](../architecture/flexbox-layout.md)
- [Layout Measurement](../architecture/layout-measurement.md)
