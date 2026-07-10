# POEM 2.0 Foundation

POEM 2.0 is being delivered as a sequence of buildable milestones. Windows and
D3D11 remain the only shipping backend for now, while public design, event,
semantics, drawing, and platform-service contracts are deliberately independent
of Win32 types.

## Themes

Applications can install a live theme manager through `render.AppConfig`:

```go
themes := render.NewThemeManager(render.ModernDarkTheme())

render.Run(render.AppConfig{
    Title:        "Example",
    Theme:        themes,
    BuildPagesFn: buildPages,
})
```

Switching is an immutable publication operation:

```go
_ = themes.Set(render.EditorialTheme())
```

Built-in foundations currently include modern dark, modern light, Windows,
Editorial, and high contrast. Product themes should derive a value with
`Theme.With` and change semantic tokens rather than editing a published value.

Theme publication enforces WCAG-oriented contrast across normal and muted text,
standard surfaces, accent/destructive/warning/success foreground pairs, and
focus indicators. Semantic foreground tokens (`OnAccent`, `OnDanger`,
`OnWarning`, and `OnSuccess`) keep control variants accessible without
hardcoded widget colors.

Applications may follow live Windows preferences without importing Windows
types:

```go
Accessibility: render.AccessibilityConfig{
    FollowSystemTheme:   true,
    FollowReducedMotion: true,
    SystemThemeResolver: func(mode render.ThemeMode) render.Theme {
        if mode == render.ThemeModeHighContrast {
            return render.HighContrastTheme()
        }
        if mode == render.ThemeModeLight {
            return render.ModernLightTheme().With("Product Light", applyProductTokens)
        }
        return render.ModernDarkTheme().With("Product Dark", applyProductTokens)
    },
},
```

If no resolver is supplied, POEM selects its modern light/dark or high-contrast
foundation. Windows preferences are polled through the optional
`platform.SystemPreferences` capability and applied on POEM's synchronized
render loop. Reduced motion freezes loading phases, suppresses particles and
continuous animation repaints, and completes page transitions immediately.

## Themed controls

POEM 2.0 constructors enable theme-based rendering and semantic accessibility:

```go
save := render.NewButton("save", "Save", saveDocument)
save.Variant = render.VariantPrimary

autosave := render.NewSwitch("autosave", "Autosave", enabled, func(next bool, state *render.ApplicationState) {
    enabled = next
})
```

Existing struct literals remain source-compatible while applications migrate.
New controls should use semantic variants and `StyleOverride` only for genuine
product-specific exceptions.

Themed `Panel` accepts a scoped `StyleOverride` for content-owned surfaces such
as book-cover swatches; ordinary surfaces keep their theme tokens. `LabeledBox`
derives its visible label color from the active theme instead of assuming a dark
background.

Raw legacy colors are no longer selected by default. An application can set
`AppConfig.LegacyComponentStyles` temporarily while migrating, but new code must
not depend on that compatibility escape hatch.

Run the visual acceptance gallery with `go run ./cmd/gallery`. It covers live
theme switching, core actions and inputs, overlays, semantics, and responsive
layout in one automation-enabled window.
`go test ./cmd/gallery` builds every gallery tab and fails if the gallery
reintroduces default glass, particles, or non-themed ordinary controls. Keep it
green when adding examples so the public showcase continues to demonstrate the
professional POEM 2.0 authoring path.

The current desktop-suite slice includes tabs, selects, badges, separators,
toolbars, menus, popovers, tooltips, toasts, spinners, skeleton loading states,
breadcrumbs, pagination, accordions, keyboard-navigable trees, overlay-backed autocomplete,
a civil-date picker, virtualized lists, and data tables. Data tables use a single tab stop and support
row selection with Up, Down, Home, End, Page Up, Page Down, Enter, and Space while keeping the active
row visible. Tab lists likewise use one tab stop; arrow keys wrap across enabled tabs while Home and End
jump to the first and last enabled tab. Focused menu overlays select their first enabled item, support
wrapped arrow navigation, Home, End, typeahead, activation, and focus restoration on dismissal.
Select controls render options in z-ordered overlays instead of outside their layout bounds. Closed
arrow/typeahead navigation publishes controlled values immediately; expanded navigation retains only
the active option until Enter or Space commits it, and Escape restores the owner without a selection.
Date-picker calendars derive their header, weekday, cell, and popup geometry from theme metrics. They
publish a 6-by-7 semantic grid and support day/week arrows, week Home/End, month Page Up/Page Down,
range clamping, Enter/Space selection, and Escape cancellation.
Accordions expose exactly one active header as focused, skip disabled headers during wrapped arrow and
Home/End traversal, and keep focus actions separate from idempotent expand/collapse actions.
Trees preserve their expanded parent/child hierarchy in the semantic tree and retain a roving active
item independently of controlled selection. Arrows, Home/End, and prefix search move selection while
Left/Right collapse, expand, and enter branches using the active item.
Pagination uses one sequential focus stop with a retained active page, allowing repeated arrows and
Home/End to advance across controlled rebuilds while the application remains the owner of selection.
Overlay entries distinguish modal, informational, focused, and owner-anchored lifecycles. Sequential
focus dismisses anchored menus, combo lists, suggestions, and calendars before advancing from their
owner, while informational overlays such as toasts remain visible. Pointer routing preserves clicks
inside the popup or its owner, dismisses on genuine outside clicks before dispatch, and keeps anchored
ancestor layers alive when interacting with a nested popup.
Semantic child targets (tabs, menu items,
breadcrumb destinations, tree items, suggestions, options, page buttons, and
table rows) can be invoked directly by stable automation ID.

Labels support `TypographyBody`, `TypographyCaption`, `TypographyLabel`,
`TypographyTitle`, and `TypographyHeading`. These resolve through theme text
tokens and are carried as styled text runs to the native renderer.

`AppConfig.Typography` accepts a primary font path plus ordered fallback font
paths. On Windows, POEM supplies system fallback candidates when none are
configured. Newly observed Unicode codepoints are rasterized into a bounded
atlas and published to D3D11 through protocol v2 without restarting the window.
Native UTF-8 decoding remains codepoint-correct when a glyph is unavailable.
`Accessibility.TextScale` scales the rasterized font atlas, semantic typography
metrics, and minimum control heights together so larger text is not clipped.

`TextInput` and `TextArea` provide rune-index selection, Shift and word
navigation, mouse-drag selection, replacement editing, and Unicode
copy/cut/paste through the portable clipboard capability. Multiline paste
normalizes Windows line endings; password fields never publish selected text.
Automation can inspect `selection_start`/`selection_end` and set a range with
`POST /select-text` for either control.

Windows `WM_IME_*` messages are transported as bounded protocol-v2 composition
events. Text fields retain the pre-edit range across declarative rebuilds, draw
the measured composition with an underline, and commit or cancel it atomically.
The same lifecycle is available to automation for deterministic visual tests.

## Transient component state

`ApplicationState.TransientState` is a concurrency-safe typed store keyed by
stable component IDs. Controls use it for interaction-only values such as an
autocomplete highlight or displayed calendar month, allowing those values to
survive declarative page rebuilds. Applications still own domain values such as
the selected option or date and receive changes through callbacks.

## Platform capabilities

`platform.Services` defines optional presentation, text shaping, clipboard, IME,
cursor, dialog, accessibility, system-theme, and window capabilities. A missing
service means that capability is unsupported; components must not import native
Windows types.

## Semantics and protocol

Semantic components publish platform-neutral roles, names, values, states,
actions, access keys, bounds, relationships, and optional numeric-range metadata. Relationships
use stable IDs for `LabeledBy`, `DescribedBy`, `Controls`, and `FlowsTo`; tree
validation rejects missing, duplicate, self-referential, or oversized links. POEM flattens that tree
into protocol v2 only when its contents change. The Windows sidecar maps the
latest snapshot to UI Automation fragments and standard Invoke, Value, Toggle,
Selection, SelectionItem, ExpandCollapse, RangeValue, Text, Grid, GridItem,
Table, TableItem, and Scroll patterns. Component containers preserve their
render-tree ownership in the portable semantic tree, so viewport, toolbar,
dialog, and overlay descendants navigate under the correct parent. UIA text ranges
use native UTF-16 offsets while protocol actions convert them to POEM's
rune-indexed selections. Provider actions return to Go as stable-ID semantic
events; Win32 and COM types remain private to the sidecar. Semantic updates
raise UIA property, focus, text, selection, and structure-change notifications
from the Win32 UI apartment. Password controls advertise `IsPassword` but do
not expose ValuePattern, TextPattern, semantic plaintext, or automation
plaintext.

Shortcut parsing and dispatch live in the portable `events` package. Chords are
normalized before registration, and Win32 transmits Control, Shift, and Alt as
modifier flags without leaking native key types into components. Explicit button
mnemonics use `Alt+key`, respect disabled/loading state and active overlay scope,
and surface through UIA `AccessKey`.

Automation commands can resolve a unique semantic target by role, accessible
name, and positive or negative state flags. Stable IDs remain supported. The
resolver operates on the same validated portable tree published to UIA, rejects
ambiguous or empty selectors, returns the resolved `target_id`, and applies
bounded HTTP request and selector limits.

`NewSlider` is controlled when an `OnChange` callback is supplied: its `Value`
remains application-owned across declarative rebuilds and legacy `SliderValues`
storage cannot overwrite it. Struct-literal sliders remain temporarily
uncontrolled for migration compatibility and will be removed with the legacy API.

On Windows these portable links map to UIA `LabeledBy`, `DescribedBy`,
`ControllerFor`, and `FlowsTo` properties. `LabeledBox` publishes its visible
title as a semantic text node and automatically labels its child control.

After building `cmd/gallery` as `POEM_gallery_uia.exe` and the Release sidecar,
run `powershell -ExecutionPolicy Bypass -File cpp_sidecar/test_uia.ps1` to verify
discovery, accurate range metadata, and native-to-Go actions against the live
gallery, including delivered property/text notifications, Unicode text-range
selection, password redaction, tab/table selection, grid coordinates, column
headers, read-only cell values, and containing-grid relationships.
It also verifies a semantic label relationship against a live provider object;
protocol round-trip and limit tests cover description, controller, and
reading-flow relationship payloads.
The probe also scrolls the live gallery viewport to 100% and observes its UIA
property-change notification. Scroll containers translate and clip descendant
semantic bounds into viewport coordinates and mark fully clipped controls
offscreen.

Go and C++ protocol changes must always be made and tested together.

## Downstream migration checkpoint

Library's desktop shell is migrated to the POEM 2.0 Modern Light foundation.
Its sidebar, header, catalog, reader, search, and wishlist use Grid, FlexBox,
ScrollView, semantic variants, accessible labels, and scoped content colors.
Legacy particles, glass panels, raw ordinary-control palettes, arbitrary header
bounds, and the app-local stack layout were removed. Its uncached Windows
workflow drives navigation and book opening through semantic automation selectors.

HamsterGame's conventional chrome is migrated without absorbing game rendering
into POEM. F10 pause and win/loss results use modal dialogs, focus scopes,
semantic buttons, mnemonics, and the Windows-derived theme; room selectors use
themed disabled/accessible states. The raycaster, HUD, CRT effects, minimap, and
cash-out particles remain product-owned custom drawing.

Dialogs apply the active theme even when its manager revision is zero, wrap
messages into theme-native semantic text lines, and size action buttons from
their labels. This keeps modal content readable under every foundation and
prevents translated or descriptive actions from being silently ellipsized.

WritingStudio's native shell now opts into the Editorial foundation, prefers
system UI typography on Windows, gates automation behind
`WRITINGSTUDIO_POEM_AUTOMATION`, and uses local theme helpers for dashboard,
settings, vault, dossier, history, and character-dialog chrome. Its product
identity remains editorial, but ordinary controls no longer depend on mandatory
particles, glass, monospaced typography, or hardcoded button palettes.

GenEngine's POEM studio now opts into Modern Dark, prefers system UI typography,
and uses theme-native panels, semantic button variants, selected states,
progress, and explicit async repaint requests for the main dashboard plus
profile and private-workspace setup flows. Remaining local colors are constrained
to product-semantic status text, model-health chips, and descriptive copy that
will migrate with the next readout/content-styling pass.
