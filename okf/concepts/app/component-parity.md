---
type: concept
title: Component Parity
description: What the pkg/app Node set covers across desktop, web, and Android — per-kind status including the Android touch-sweep verification — and which catalog components are portable, excluded, or backend-specific.
tags: [architecture, parity, node, roadmap]
timestamp: 2026-07-22T00:00:00Z
---

> **Ported from the Trellis/GopherWeb bundles at the 2026-07-16 consolidation.** Historical names map as: "Trellis" = `pkg/app`; "GopherWeb" = `pkg/web` (CSS prefix now `poem-`); "POEM" as a sibling project = the `pkg/render` engine layer. All are now this repository.
# Component Parity

The `pkg/app` promise is write-once-run-everywhere, and that promise is only as wide as the
`Node` set (`pkg/app/node.go`). This concept is the honest measure of how wide that is: a
`Node` kind exists only when *every* target can render it (see
[Overview](/concepts/app/overview.md) — "There is no partial-coverage state").

The targets are not symmetrical in what a kind costs them. Desktop and Android share the
`pkg/render` component built by `pkg/app/desktop/render.go` — the Android presenter draws the
same engine components over the wire, so a new kind needs **no Android-specific rendering
work**. What Android *does* need per kind is interaction verification (touch instead of a
mouse, IME instead of WM_CHAR, back-gesture instead of Escape). The web target needs its own
`pkg/web` renderer per kind. Adding a kind therefore means: the node in `pkg/app/node.go`,
cases in both drivers' `render.go`, and a touch pass on Android.

This is a living document; move rows in the same change that moves the code.

## Ported

These `Node` kinds run on all three targets today. "Android" records the touch-sweep
verification of 2026-07-16 (API 36 emulator, preferences/settings examples).

| Node kind        | Engine component (desktop + Android) | Web renderer (`pkg/web`) | Android touch status | Notes |
| :--------------- | :------------------ | :------------------ | :---- | :---- |
| `TextNode`       | `Label`             | `<p>`               | ✅ renders | |
| `ButtonNode`     | `Button`            | `action.Button`     | ✅ tap (counter milestone) | |
| `TextInputNode`  | `TextInput`         | `form.Input`        | ✅ focus summons IME; typed/backspace via key events | |
| `TextAreaNode`   | `TextArea`          | `form.Textarea`     | ✅ same IME path, multi-line | Multi-line sibling of `TextInputNode`. |
| `CheckboxNode`   | `Checkbox`          | `form.Checkbox`     | ✅ tap toggles | First `bool` payload — see Overview. Tap exposed the `Slider.OnMouseUp` engine bug (fixed). |
| `SwitchNode`     | `Switch`            | `form.Switch`       | ✅ tap toggles | `bool`, same posting mechanics as `CheckboxNode`. |
| `SelectNode`     | `Select`            | `form.Select`       | ✅ popup opens via overlay, option pick closes | |
| `RadioGroupNode` | `Radio` (one per option) | `form.RadioGroup` | ✅ tap selects | Single-choice-from-a-list, reuses `Option`. |
| `SliderNode`     | `Slider`            | `form.Range`        | ✅ touch drag; `ProgressBar` tracked live | First **float** payload — `FloatPayload`/`Msg.Float`. |
| `BadgeNode`      | `Badge`             | `feedback.Badge`    | ✅ renders, variant color correct | First `Variant`-carrying kind; non-interactive. |
| `ProgressBarNode`| `ProgressBar`       | `feedback.Progress` | ✅ renders, live-updates | Display-only; supports indeterminate. |
| `TableNode`      | `DataTable`         | `data.Table`        | ✅ renders read-only | Row selection/sorting deliberately not surfaced yet. |
| `AccordionNode`  | `Accordion`         | `widget.Disclosure` | ✅ header tap expands; nested control dispatches | Expand state is backend-local chrome, not `App[S]`. Known cosmetic overlap after expansion (TASK.md). |
| `TabsNode`       | `Tabs` (controlled) | `widget.FormTabs`   | ✅ tab tap switches panel | Selection is `App[S]`-owned so the app can set it. |
| `ImageNode`      | `ImageView`/`DrawImage` | data-URI `<img>` | ✅ renders through GLES texture cache | Encoded bytes and alt text. |
| `ImageViewportNode` | `ImageViewport` | clipped `<img>` + Pointer Events | ✅ pan + two-pointer pinch + tap/markers | Controlled transform, normalized point activation, and stable accessible markers. Fills its container rather than freezing at a laid-out size (`UI101`). |
| `MapViewportNode` | `ImageViewport` (retained map scene) | `poem-map-viewport` canvas + WebGL2/WASM | ✅ GPU vector map, camera + feature messages, raster fallback | See [GPU Vector Map Rendering](/concepts/architecture/map-rendering.md). |
| `ResponsiveNode` | `Responsive` | compact/wide fieldsets + `matchMedia` | ✅ width-selected branch | Compact is the no-JS baseline; inactive web fields are disabled. |
| `ContainerNode`  | `FlexBox`           | `<div>` flex        | ✅ layout correct at density | |
| `OverlayNode`    | `Overlay`           | `position:relative` box + absolutely-positioned layers | ✅ base fills; layers anchor + hit-test above base | Non-modal z-stack for canvas controls; see [Desktop Driver](/concepts/app/desktop-driver.md). |
| `ModalNode`      | `Modal` (overlay)   | `widget.Modal`      | ✅ trigger opens overlay; nested control dispatches; back gesture (→Escape) dismisses | Open state is not `App[S]`; content is an open-time snapshot — verified visibly on Android. |

## Portable

Both catalogs have an equivalent, and the component's interaction model already fits the
`Msg`/`Update` pipeline. In practice the components that pass this bar are **single controls
that fire one `Msg` or display one value** — which is exactly why every kind ported so far did.

That set is now fully ported: the interactive single controls (`TextInput`, `TextArea`,
`Checkbox`, `Switch`, `Select`, `RadioGroup`, `Slider`) and the display-only ones (`Text`,
`Badge`, `ProgressBar`, read-only `Table`) are all in the **Ported** table above, as are all
three composites (`Modal`, `Accordion`, `Tabs`).

What remains is smaller than it once looked, because a chunk of the old "doesn't fit" list was
mis-triaged (see below): `Breadcrumbs`/`Pagination` are probably portable and reopened,
`Table` row selection extends an already-ported kind, and `Toolbar` is genuinely not worth it.

### Engine-blocked, now cleared

Several POEM controls had no GopherWeb equivalent. Per the workspace rule that a missing
primitive is added at the engine boundary before a one-off shortcut, each was ported by
*adding the GopherWeb component first, then the `Node` kind* — never hand-rolling bespoke
markup inside Trellis. Four were added this way: `form.Switch` (unblocked `SwitchNode`),
`form.RadioGroup` (`RadioGroupNode`), `form.Range` (`SliderNode`, Trellis's first float
payload), and `feedback.Progress` (`ProgressBarNode`). No engine-blocked single control
remains outstanding.

## Not yet ported, and why

These have an equivalent in both catalogs but were not drop-ins. Two of them are here because
of a **triage error worth recording**: `Breadcrumbs` and `Pagination` were once excluded as
"href routing doesn't fit a single-page model", but that reasoned from GopherWeb's *current
implementation* rather than from the component's concept. Both are ordinary `App[S]` state
(a path, a page index) with `Msg`-firing controls, and both targets can express that. The
general lesson: a component is only a genuine misfit when the **shared API can't express it**,
not when one catalog happens to ship a different flavour of it.

| Component     | POEM              | GopherWeb              | Why it isn't a drop-in |
| :------------ | :---------------- | :--------------------- | :--------------------- |
| `Breadcrumbs` | `Breadcrumbs`     | `navigation.Breadcrumbs` | **Reopened — probably portable.** The earlier "href routing doesn't fit" call was rationalized from GopherWeb's *implementation* being `<a href>`, not from the concept: a breadcrumb trail whose path lives in `App[S]` and whose items fire a `Msg` works on both targets and needs no URLs. Porting it means adding a `Msg`-capable breadcrumb to GopherWeb at the engine boundary, exactly as `form.Switch`/`Range`/`FormTabs` were added. Not yet built. |
| `Pagination`  | `Pagination`      | `navigation.Pagination` | **Reopened — probably portable**, same reasoning: the page index is ordinary `App[S]`, and clicking a page fires a `Msg` carrying it. No routing required. Not yet built. |
| `Toolbar`     | `Toolbar` (child container) | `layout.Toolbar` (title + typed `[]action.Button`) | GopherWeb's `Toolbar` accepts only `action.Button`s, not arbitrary child nodes, so it can't wrap a rendered `Node` subtree; and it adds nothing a titled `ContainerNode` doesn't already give. **Intentionally excluded** — use `Container`. |
| `Alert`/`Toast` | `Toast`         | `feedback.Alert`/`Toast` | Genuinely portable as a *display* `Node` (like `Badge`); the open question is only whether dismissal is `App[S]` or client-side chrome (cf. `ModalNode`). A candidate once a driving use case appears. |

### The `ModalNode` pattern and the backend-local-chrome resolution

A `ModalNode`-style resolution — content rendered up front, the open/selected chrome owned
*backend-locally* rather than in `App[S]` — is what makes a composite component portable
despite its open state living client-side on the web but server-side on POEM. The key is that
"backend-local" on POEM means POEM's **transient store** (`ApplicationState.TransientState`),
which survives the per-frame rebuild that both POEM's own `BuildPagesFn` and the Trellis
runtime do — the same place `Select` popups and `Tabs` selection already live.

`AccordionNode` was ported exactly this way, and it required a POEM engine change to get there:
POEM's `Accordion` previously read expansion from an app-owned `Expanded` map and, when
uncontrolled, mutated only that map — which a rebuild discards. It now persists uncontrolled
expansion in the transient store (seeded from the author's initial open set), so a rebuilt
instance rehydrates it. That made expansion genuine backend-local chrome, so `AccordionNode`
carries no `Msg` for open/close, matching the web backend's native `<details>`. `Tabs` is the
next component to get the same treatment (see its row above). Each such change is recorded in
the [log](/log.md) and, being a shared-engine change, in POEM's own log too.

## Deliberately backend-specific (not counted against parity)

These exist on only one side for reasons intrinsic to that backend, and are **not** parity
gaps to close. Listing them keeps "how much is left" honest by excluding what was never
meant to be shared:

- **POEM-only, native/GPU-bound:** `ParticleComponent`, `GlassPanel`, `LineChart`,
  `DatePicker`, `Autocomplete`, `Tree`, `Menu`/`Popover`, `SplitPane`, `VirtualList`,
  `ScrollView`, `Tooltip`, `Spinner`/`Skeleton`. These lean on POEM's own painter, overlay,
  or windowing model; several have no meaningful no-JS web analogue.
- **GopherWeb-only, page-level web idioms:** `CommandPalette`, `StatCard`, `EmptyState`,
  `Navbar`, `Card`. These are web-page composition helpers, not the interactive controls the
  `Node` IR is about.

A component leaving this list (for example, a future `fetch`-based web transport making
`Autocomplete` portable) is a real architectural change and should be recorded in the
[log](/log.md), not a silent edit here.
