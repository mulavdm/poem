---
type: Decision
title: GopherWeb Parity
description: POEM and GopherWeb converge naming and shape wherever their public component APIs solve the same problem, but stay two separate codebases rather than merging into a shared UI representation.
tags: [architecture, decisions, gopherweb, components, cross-project]
timestamp: 2026-07-10T00:00:00Z
---
# GopherWeb Parity

POEM (this repo, a native GPU desktop UI engine) has a sibling project, GopherWeb
(`../GopherWeb`, a server-rendered Go HTML component library). Both expose a public
declarative component API with a similar shape — variant/size tokens, item-list components,
form-style controls — because both grew out of the same instinct for what a clean component
API should look like, independently.

The question this decision settles: should the two converge into one shared representation
that a single downstream app could target to get both a desktop and a web app from one
codebase, or should they stay two independent APIs that happen to look alike where it's
natural for them to?

## Two structural blockers rule out a shared representation

**State model.** POEM is a long-lived, stateful, immediate-mode-ish component tree —
interaction state that outlives a single frame (a highlighted autocomplete option, an open
overlay, IME composition state) lives in `pkg/render/state.TransientState`, keyed by stable
component ID, and `types.OverlayManager` tracks every open modal/menu/combobox across the
whole running app. GopherWeb is a pure per-request function: every component's `HTML()` is
`struct → template.HTML`, executed once per HTTP request, with zero server-side state —
`Tabs`/`Modal`/sort/filter interactivity all live in hand-written vanilla JS reacting to DOM
events after the fact. A shared representation would have to pick one model or express both,
and expressing both means either giving POEM's Go orchestrator no real state (defeating the
point of a native app) or giving GopherWeb server-side per-request state it deliberately
doesn't have today.

**Trust boundary.** POEM's component surface has no raw-content injection point anywhere —
the only escape hatch is `components.StyleOverride` (typed colors and geometry, never markup).
GopherWeb deliberately exposes caller-trusted `...HTML template.HTML` fields
(`Card.BodyHTML`, `Modal.ContentHTML`, `TabItem.Content`, etc.) because it is assembling real
HTML for a browser and some callers legitimately need to compose raw markup. A shared
representation needs an explicit policy on this that neither side's current design gives it
for free.

A shared IR would therefore have to compromise one platform's identity to paper over one or
both of these — either flatten POEM down to what static HTML can express, or grow GopherWeb
into a client-state framework, which GopherWeb's own `AGENTS.md` rules out ("Do not add a Go
web framework... JavaScript framework... without explicit approval").

## What "convergence" means instead: mirrored parallel APIs, permanently

Four rounds of real work (see `okf/log.md`, 2026-07-08 through 2026-07-10) tested this by
actually doing it, not just reasoning about it:

1. **Token convergence** — `theme.Variant`'s `Destructive`→`Danger`, `Default`→`Neutral`
   renamed to match GopherWeb's `components.Variant`, which already had the right names.
   `VariantSubtle` stayed POEM-only; GopherWeb's Button CSS has no borderless treatment to
   match it to.
2. **Field-shape convergence** — `Button.Label`→`Text`, `TextInput`/`TextArea.Text`→`Value`,
   `Badge.Role TextRole`→`Variant theme.Variant`, `TableColumn.Title`→`Label`. In every case
   the rename also fixed an internal POEM inconsistency (Button was the one component saying
   `Label` when every sibling said `Text`; Badge was straining `TextRole` to fake a background
   it doesn't model) — the GopherWeb-parity rename and the POEM-internal-consistency fix were
   the same fix, not a coincidence, because both projects independently reach for the same
   vocabulary when a component solves the same problem.
3. **Feature trades** — POEM's `Modal` gained self-contained Escape-to-close (fixing a real
   automation bug in the process) and an `OpenModal` convenience, borrowed from GopherWeb's
   `widget.Modal`; `LabeledBox` gained a help/error text row, borrowed from GopherWeb's
   `form.Field`. GopherWeb's `Toast` gained dismiss behavior and `Pagination` gained page
   windowing, both borrowed from POEM. POEM's focus trap and initial-focus/focus-restore
   turned out to already exist generically in `ApplicationState.CycleFocus`/`openOverlay` —
   nothing to borrow there.
4. **Verified-not-assumed exclusions** — `Select`, `Tabs`, `Breadcrumbs` needed zero changes;
   their overlapping fields already agreed. `Accordion.Item.Title` vs. `Disclosure.Summary`
   were checked and found correctly divergent: POEM's `Title` matches its own convention
   (`Dialog.Title`, `Chart.Title`), GopherWeb's `Summary` names the literal `<summary>` tag it
   renders. `TableRow`'s shape (POEM: `{ID, Values}`; GopherWeb: bare `map[string]string`) is
   structural — POEM needs row identity for `SelectedID`/keyboard navigation, GopherWeb's
   stateless render has no selection to key against.

Every gap that was actually a naming or shape mismatch converged cheaply, usually with
GopherWeb needing zero changes because it already had the right name. What's left standing
after four rounds (`OnClick` vs. `Href`, IME composition vs. a no-JS baseline, caller-trusted
HTML vs. no injection points, `Open`/`OnOpenChange` vs. nothing) is the two structural
blockers above, not naming drift.

## Consequences

- There is no shared component-definition package between POEM and GopherWeb, and none is
  planned. A downstream app that wants both a desktop and a web surface writes each screen
  twice, once against each project's own API.
- When adding or changing a public component's fields or enum values in either project, check
  the sibling project for the same concept before picking a name. Converge naming and shape
  when the underlying need is genuinely the same (same semantic role, compatible value set).
  Leave it alone when the difference is structural — driven by the state model, the trust
  boundary, or a native platform capability one side has and the other doesn't.
- Known real gaps intentionally left open by this round, candidates for a future targeted
  pass rather than blanket unification: POEM's `DataTable` has no column-sort UI (GopherWeb's
  `Table` does, via `Sortable`); GopherWeb's `Toast` has no `Title` field (POEM's does).
- This decision should be revisited only if the evidence changes — e.g. if a genuinely
  stateless subset of both component sets (layout primitives, static content) grows large
  enough to justify its own narrow shared package, without touching either project's stateful
  or trust-boundary-crossing majority.

See also: `../GopherWeb/okf/decisions/gopherweb-parity.md` for the matching record on the
GopherWeb side.
