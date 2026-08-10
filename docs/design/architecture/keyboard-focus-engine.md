# Keyboard Focus & Global Hotkey Engine Architecture

POEM features a dedicated, low-latency keyboard controller integrated into the native Win32 message procedure. It provides platform-neutral accessibility semantics with sequential focus cycling, theme-defined focus indicators, automatic scroll centering, and customizable global hotkey listeners. On Windows, the in-process host exposes the semantic tree through UI Automation.

Visible form labels should use `LabeledBox`; it emits a semantic text label and connects the child through the portable `LabeledBy` relationship. Custom semantic components can populate `semantics.Relationships` (`LabeledBy`, `DescribedBy`, `Controls`, and `FlowsTo`). Standard buttons expose the same optional value as `Button.Relations`. POEM validates every relationship target before publication.

## Sequential Focus Cycling Pass (`CycleFocus`)

Focus navigation is managed sequentially via `Tab` and `Shift+Tab`. To prevent legacy state corruption, `CycleFocus`:

1. Traverses components declared inside the active page registry (`s.Pages[s.CurrentPage]`) instead of a static global registry.
2. Recursively walks the component hierarchy to collect all interactive widgets returning `Focusable() bool { return true }` (e.g. `Button`, `TextInput`, `Slider`).
3. Determines the index of the currently focused widget ID, shifts the focus pointer forward or backward (clamped to slice boundaries), and triggers a repaint.

- **Focus Navigation**: tapping `Tab` or `Shift+Tab` cycles keyboard focus sequentially across focusable elements on the active page.
- **Escape behavior**: tapping `Esc` dismisses the top eligible overlay and restores its launcher; with no overlay open it clears active focus.

## Polymorphic Scroll Centering (`ScrollContainer`)

To automatically glide focused items into view when navigating massive lists, POEM defines the `types.ScrollContainer` interface:

```go
type ScrollContainer interface {
    Component
    ScrollToChild(childID string, childBounds image.Rectangle, state *ApplicationState) bool
}
```

- **Circular Import Avoidance**: defining this interface inside `pkg/render/types` instead of `pkg/render/components` keeps the package layout clean and fully compliant with Go's package tree dependencies.
- **Centered Centering Math with Boundary Spacing**: `ScrollView` implements `ScrollToChild`. It dynamically queries if the target child is a descendant, computes its Y bounds relative to the viewport top coordinate (independent of LERP visual scroll offsets), and shifts `ScrollY` up or down to center the element inside the visible viewport. Boundary padding prevents focused elements and their theme-defined focus outlines from clipping against viewport edges.

## Semantic Focus Painter Integration

Focused interactive elements use the current theme's focus token. Inside `Button.Draw`, `TextInput.Draw`, and `Slider.Draw`:

- Components query `state.FocusedID == CompID`.
- Core controls draw a restrained outline outside the control boundary using `Theme.Colors.Focus`.
- Glow remains available to application-specific effects but is not mandatory control styling.

**Theme-defined focus styling**: focused components use the active theme's accessible focus-ring token. Product-specific glow remains an optional application effect rather than a core-control requirement.

## Declarative Keyboard Adjustments & Event Submissions

Interactive components capture specialized keystroke operations:

- **`Button.OnKey`**: intercepts virtual key `Enter` (`VK_RETURN` = 13) and `Space` (`VK_SPACE` = 32) only during `WM_KEYDOWN` key events, running the button's `OnClick` closure. This filters out the duplicate `WM_CHAR` character translations (such as character `\r`) that would otherwise double-trigger the buttons and instantly negate toggles.
- **`Slider.OnKey`**: captures `Left Arrow` (`VK_LEFT`) and `Right Arrow` (`VK_RIGHT`) keys to mathematically increment or decrement the slider value. It snaps values to the nearest 5% increment (`math.Round((Value ± step) / step) * step`) to clean up any precise decimal offsets left behind from custom mouse dragging.
- **`TextInput.OnKey`**: supports submission, rune-index selection, word navigation, replacement editing, and clipboard shortcuts. Windows IME pre-edit strings arrive through explicit composition events rather than duplicate `WM_CHAR` insertion.

## Win32 Key Dispatching Loop

Inside the native `libWndProc` under `WM_KEYDOWN` (0x0100):

- **Escape (`VK_ESCAPE`)**: resets `globalState.FocusedID = ""` to clear keyboard capture instantly.
- **Tab Cycling**: checks modifier states via `win32.GetKeyState(VK_SHIFT) < 0` to trigger forward/reverse cycling.
- **Modifier Shortcuts (Ctrl+S)**: detects Left, Right, and Generic Control modifier pressed states simultaneously. Queries registered global listeners from the `state.Hotkeys` map and fires handlers.
- **Engine Keylogger**: logs keystroke virtual key codes and active Control modifiers to standard output in real-time, providing immediate visibility during debugging.
- **Event Propagation**: dispatches keys directly to the focused component's `OnKey(...)` method, facilitating modular key handling.

## Standard Key Triggers & Input Submissions Per Component

- **Buttons**: pressing `Enter` on a focused button executes its `OnClick` action instantly.
- **Sliders**: pressing the `Left Arrow` or `Right Arrow` keys increments or decrements the slider value by exactly 5% steps.
- **Tab lists**: arrow keys wrap across enabled tabs; `Home` and `End` select the first and last enabled tab. The strip occupies one sequential tab stop.
- **Menus**: opening a focused menu moves keyboard focus to its first enabled item. Arrows wrap, `Home`/`End` jump, character keys search by prefix, and dismissal restores the launcher's focus.
- **Selects**: options use the overlay layer. Closed arrows/typeahead change the controlled value; while expanded, arrows, `Home`, `End`, and typeahead move the active option, `Enter`/`Space` commit, and `Escape` cancels.
- **Date pickers**: arrow keys move by day or week, `Home`/`End` move to week boundaries, `Page Up`/`Page Down` change month, and range limits are enforced before `Enter`/`Space` commits.
- **Accordions**: Up/Down wrap across enabled headers, `Home`/`End` jump to enabled boundaries, and `Enter`/`Space` toggles only the active disclosure.
- **Trees**: Up/Down and `Home`/`End` move the active selection, character keys search visible labels, and Left/Right navigate the preserved parent/child hierarchy.
- **Pagination**: Left/Right advance the retained active page and `Home`/`End` jump to the first/last page without requiring uncontrolled application state.
- **Anchored overlays**: tabbing or clicking elsewhere dismisses menus, select lists, autocomplete suggestions, and calendars before advancing focus. Owner clicks still toggle correctly, nested popup ancestors remain open, and non-interactive toasts remain visible.
- **Dialogs**: modal dialogs trap sequential focus, restore their launcher when closed, wrap message text with active-theme typography, and size actions from their labels.
- **Data tables**: `Up`, `Down`, `Home`, `End`, `Page Up`, and `Page Down` move the active row and its controlled selection; `Enter` and `Space` activate it. Navigation scrolls the row fully into view.
- **Text Inputs**: pressing `Enter` inside a text field fires the optional `OnSubmit` callback:
  ```go
  &render.TextInput{
      CompID: "console_cmd",
      Rect:   image.Rect(0, 0, 150, 40),
      Placeholder: "COMMAND...",
      OnSubmit: func(text string, state *render.ApplicationState) {
          state.StatusText = "Executed: " + text
      },
  }
  ```

## Global Shortcuts and Mnemonics

Register normalized portable chords with `RegisterShortcut`. Supported modifiers are Control, Alt, Shift, and Meta; keys include letters, digits, navigation keys, Delete, and F1-F24. Invalid or ambiguous chords return an error.

```go
func BuildAllPages(state *render.ApplicationState) {
    if err := state.RegisterShortcut("Ctrl+Shift+S", func(s *render.ApplicationState) {
        s.StatusText = "Configuration Saved Successfully!"
    }); err != nil {
        panic(err)
    }
}
```

`RegisterHotkey` remains as a source-compatible wrapper. Buttons may also set an explicit `Mnemonic` rune. POEM routes `Alt+<key>` within the active modal/page, focuses and invokes the enabled button, and publishes its chord through the portable semantic `AccessKey` field and Windows UIA `AccessKey` property.

Sliders created with `NewSlider(..., onChange)` are controlled: update the application value in `onChange` and pass it back on the next build. POEM retains only the interaction state; it does not overwrite that value from the legacy slider map.

## See also
- [Architecture Overview](TDD.md)
- [Cursor and Layout Sync](../architecture/cursor-and-layout-sync.md)
- [Component Catalog](../architecture/component-catalog.md)
