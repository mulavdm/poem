---
type: concept
title: Component Parity
description: What Trellis's Node IR covers of the POEM and GopherWeb component catalogs, and which components are portable, engine-blocked, or deliberately backend-specific.
tags: [architecture, parity, node, roadmap]
timestamp: 2026-07-12T00:00:00Z
---

> **Ported from the Trellis/GopherWeb bundles at the 2026-07-16 consolidation.** Historical names map as: "Trellis" = `pkg/app`; "GopherWeb" = `pkg/web` (CSS prefix now `poem-`); "POEM" as a sibling project = the `pkg/render` engine layer. All are now this repository.
# Component Parity

Trellis's promise is write-once-run-on-both, and that promise is only as wide as the
`Node` IR (`pkg/app/node.go`). This concept is the honest measure of how wide that is: a
`Node` kind exists only when *both* backends can render it (see
[Overview](/concepts/app/overview.md) — "There is no partial-coverage state"), so parity
here is the intersection of what POEM and GopherWeb each offer, not the union.

This is a living document. Adding a `Node` kind means moving a row from **Portable** to
**Ported** in the same change that adds it to `pkg/app/node.go` and both `render.go` files.

## Ported

These `Node` kinds run on both backends today.

| Node kind        | POEM component      | GopherWeb component | Notes |
| :--------------- | :------------------ | :------------------ | :---- |
| `TextNode`       | `Label`             | `<p>`               | |
| `ButtonNode`     | `Button`            | `action.Button`     | |
| `TextInputNode`  | `TextInput`         | `form.Input`        | |
| `TextAreaNode`   | `TextArea`          | `form.Textarea`     | Multi-line sibling of `TextInputNode`. |
| `CheckboxNode`   | `Checkbox`          | `form.Checkbox`     | First `bool` payload — see Overview. |
| `SwitchNode`     | `Switch`            | `form.Switch`       | `bool`, same posting mechanics as `CheckboxNode`; `form.Switch` was added to GopherWeb to unblock it (see below). |
| `SelectNode`     | `Select`            | `form.Select`       | |
| `RadioGroupNode` | `Radio` (one per option) | `form.RadioGroup` | Single-choice-from-a-list, reuses `Option`; `form.RadioGroup` added to GopherWeb to unblock it. |
| `SliderNode`     | `Slider`            | `form.Range`        | First **float** payload — `FloatPayload`/`Msg.Float`; `form.Range` added to GopherWeb to unblock it. |
| `BadgeNode`      | `Badge`             | `feedback.Badge`    | First `Variant`-carrying kind; non-interactive. |
| `ProgressBarNode`| `ProgressBar`       | `feedback.Progress` | Display-only; no payload; supports indeterminate. `feedback.Progress` added to GopherWeb to unblock it. |
| `TableNode`      | `DataTable`         | `data.Table`        | Read-only display; row selection/sorting deliberately not surfaced yet (see "Not a clean fit"). |
| `AccordionNode`  | `Accordion`         | `widget.Disclosure` | Composite, resolved the ModalNode way — expand state is backend-local chrome, not `App[S]`. Required a POEM engine change (see below). |
| `TabsNode`       | `Tabs` (controlled) | `widget.FormTabs`   | Composite, resolved the **other** way — selection is `App[S]`-owned so the app can set it, costing a round trip per tab click on web. Required a POEM controlled-selection fix and a new GopherWeb posting tab strip. |
| `ContainerNode`  | `FlexBox`           | `<div>` flex        | |
| `ModalNode`      | `Modal` (overlay)   | `widget.Modal`      | Open state is not `App[S]` — see Overview. |

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
