---
type: decision
title: Composite Components
description: How components whose open/selected state differs between the backends (Modal, Accordion, Tabs) are ported, and which catalog components are intentionally excluded from the Node IR.
tags: [architecture, decisions, parity, node, poem, gopherweb]
timestamp: 2026-07-12T00:00:00Z
---

> **Ported from the Trellis/GopherWeb bundles at the 2026-07-16 consolidation.** Historical names map as: "Trellis" = `pkg/app`; "GopherWeb" = `pkg/web` (CSS prefix now `poem-`); "POEM" as a sibling project = the `pkg/render` engine layer. All are now this repository.
# Composite Components

Most `Node` kinds are single controls: they fire one `Msg` or show one value, and each backend
has a 1:1 component for them (see [Component Parity](/concepts/app/component-parity.md)). A
smaller set — `Modal`, `Accordion`, and the still-unported `Tabs` — are **composite**: they
own child content *and* a piece of open/selected state. That state is where the two backends
genuinely diverge, and this decision records how Trellis handles it, so future composite ports
follow one pattern instead of re-litigating it each time.

## The problem

For a composite component, "which section/tab/dialog is open" lives in a different place on
each backend:

- On the **web** backend it is client-side chrome — GopherWeb's `widget.Modal`
  (`data-poem-modal-*`), native `<details>` for `widget.Disclosure`, `data-poem-tabs` for
  `widget.Tabs`. It never round-trips to the server.
- On **POEM** it must live somewhere that survives the per-frame rebuild (`BuildPagesFn`, and
  the Trellis runtime, both rebuild the whole component tree every repaint).

If that state were put in `App[S]`, the web backend would carry state it never needs, and every
unrelated `Update` would have to preserve it. So it must *not* be `App[S]`.

## The decision: open/selected state is backend-local chrome

A composite `Node` carries **no `Msg` for its own open/selected state**. Each backend manages
that state itself:

- **Web:** the backend renders all child content up front and lets GopherWeb's own client-side
  mechanism toggle visibility (`widget.Modal`, `<details>`, `widget.Tabs`). Zero server round
  trip.
- **POEM:** the state lives in `ApplicationState.TransientState` — POEM's rebuild-surviving
  store, the same place `Select` popups and `Tabs` selection already live — **not** in the
  component instance and **not** in `App[S]`.

`Msg`-bearing content *nested inside* a composite still fires through the normal `Update`
pipeline; only the composite's own chrome is exempt. Both `pkg/app/web/render.go` collectors
(`collectFields`, `collectMessages`) therefore recurse into composite content, and the web
backend renders all of it into the form up front so a nested control submits regardless of the
chrome's open state.

### Precedents

- **`ModalNode`** was the first. Its content is a snapshot from open-time; the trigger opens it
  via a one-shot `OpenModal` side effect on POEM and `data-poem-modal-open` on the web. See
  [Architecture Overview](/concepts/app/overview.md).
- **`AccordionNode`** was the second, and the first to need a **POEM engine change**: POEM's
  `Accordion` previously mutated only its instance `Expanded` map when uncontrolled, which a
  rebuild discards. It now persists uncontrolled expansion in `TransientState` (seeded from the
  author's initial open set), so it survives rebuilds like `Tabs`/`Select` do. Unlike a modal,
  accordion section content stays continuously live against `App[S]`; only visibility is chrome.
- **`TabsNode` deliberately went the other way**, and it is the exception that defines the rule
  below: its selection lives in `App[S]` and travels as an `OnChange` `Msg` carrying the
  activated tab's ID. See "Not every composite is chrome".

The engine-boundary rule applies here exactly as it does for GopherWeb primitives: when a
backend lacks the mechanism to keep composite state backend-local, add it *to that engine*
(POEM's transient-store self-management) rather than smuggling the state into `App[S]` or
tracking it inside Trellis.

## Not every composite is chrome: `TabsNode` and the real trade

The backend-local-chrome pattern above is not automatic — it buys instant, round-trip-free
toggling on the web at the cost of the application being unable to **read or set** the state.
For `Modal` and `Accordion` that price is right: an app rarely needs to force a section open.
For `Tabs` it is not — "jump to the Audio tab after saving" is an ordinary requirement — so
`TabsNode` puts `Selected` in `App[S]` with an `OnChange` `Msg`, accepting a full page round
trip per tab click on the web's forms-only transport (acceptable on a no-JS baseline that
already reloads on every button press).

Framing this as "the two backends are incompatible" would be wrong, and the earlier versions of
this bundle overstated it. The backends never run at the same time; each `Run` produces one
artifact. The only thing genuinely *forced* by having two targets is that `Msg` must be
serializable rather than a Go closure (see [Architecture Overview](/concepts/app/overview.md)) —
because the same `View`/`Update` source has to work on the most restrictive target. Everything
else here, including chrome-vs-`App[S]`, is a **design trade about what the shared API should
expose**, decided per component:

| | Backend-local chrome (`Modal`, `Accordion`) | `App[S]`-owned (`Tabs`) |
| :-- | :-- | :-- |
| Author can read/set it | no | yes |
| Web cost | none (client-side toggle) | one round trip per change |
| Content rendered | all of it, up front | only the active panel |

Consequences of the `App[S]` choice, stated plainly:

- Only the selected tab's content is rendered on **either** backend, so a hidden tab's controls
  post nothing and fire nothing. `TabsNode.ActiveIndex()` is the single shared helper both
  backends *and* the web field collectors use, so they cannot disagree about what is live.
- On the web the tab strip is GopherWeb's `widget.FormTabs` — added at the engine boundary
  because `widget.Tabs` toggles client-side and posts nothing, which cannot work when the
  server owns the selection. Each tab is a submit button posting the strip's own field name, so
  the existing `valueField` path dispatches `OnChange` with no new transport machinery.
- On POEM the strip is a **controlled** `Tabs` (non-nil `OnChange`). This required a POEM engine
  fix: `Tabs.activeIndex` preferred its transient roving position over `SelectedID`
  unconditionally, so an application-driven selection change was silently ignored on any tab the
  user had previously clicked — exactly the capability `App[S]` ownership exists to provide. It
  now distinguishes "the application moved the selection" from "the user roved" and adopts the
  former. See POEM's own log.

## Intentionally excluded (not parity gaps)

Some components in both catalogs are **not** ported and are not tracked as gaps, because they
do not fit a single-page, no-routing `Msg`/`Update` model:

- **`Breadcrumbs`, `Pagination`** — GopherWeb ships these as `<a href>` / `?page=` navigation
  for multi-page apps. Trellis has no URL routes; reframing them as `Msg`-firing controls would
  be a different component than either catalog ships. Excluded unless Trellis grows real routing.
- **`Toolbar`** — GopherWeb's `layout.Toolbar` takes a typed `[]action.Button` with no
  child-node slot, and adds nothing a titled `ContainerNode` doesn't. Use `Container`.

Reversing any of these exclusions (for example, a `fetch`-based web transport, or Trellis
gaining routing) is a real architectural change and is recorded in the [log](/log.md), not a
silent edit.
