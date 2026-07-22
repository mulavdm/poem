---
type: concept
title: Adaptive Native Design
description: Semantic application nodes, state-derived commands, token resolution, platform profiles, and design linting.
tags: [app, design, adaptive, accessibility, tokens]
timestamp: 2026-07-22T00:00:00Z
---
# Adaptive Native Design

`pkg/design` resolves foundation, semantic, and platform-profile tiers into immutable renderer tokens. `RenderingModeAuto` chooses the Windows, Android, or web profile and falls back to Adaptive House. Window classes are Compact below 600, Medium at 600â€“839, Expanded at 840â€“1199, and Ultra-wide at 1200 or more logical pixels. Density follows pointer precision independently of width.

`App[S]` optionally supplies a `Design` and derives a semantic `Commands` registry from state. Command invocation remains a validated `Msg`; external work remains a keyed `Cmd`. Semantic nodes carry stable IDs, accessible metadata, controlled state, and no application-authored colors or metrics. `AdaptiveNode` carries all four standard branches, while collections and navigation describe intent rather than fixed presentation. `ActionNode` carries a **selected** state independent of its importance: native controls use the theme selection surface and web emits `aria-pressed`, so a segmented choice communicates state without masquerading as a primary action. Sections carry a semantic **`Empty`** state so a zero-result surface renders actionable guidance rather than blank space.

Native `RenderContext` and advanced `pkg/render` components consume the same resolved theme and environment. The web target serializes these tokens as CSS variables with dark, forced-colors, and reduced-motion media behavior. Presenter capability events are appended without changing prior event numbers.

`pkg/app/designlint` exposes guide-coded diagnostics and a test assertion. Its
semantic `Lint` API remains compatible; `UI042` rejects raw colors in semantic
map styles. `LintTheme` checks resolved text, status, focus, and strong-border
token pairs and emits `UI043` below their accessibility contrast thresholds.
`LintLayout(root, viewport, state)` adds post-layout checks: `UI023` reports
text whose wrap-aware intrinsic minimum exceeds rendered bounds, `UI102`
reports an actionable component outside the reachable viewport without a
scroll path, and `UI103` reports sibling focus traversal that contradicts
visual order. `UI072`/`UI073` enforce declared feedback: a running section
without accessible progress, and an empty section without visible guidance.
Checked-in applications should have no diagnostics except
explicit, reasoned allowlists during migration.

M8 completes the workspace composition contract: semantic header/status content surrounds a persistent primary canvas and a tools surface which docks, overlays, or becomes a component-owned bottom sheet by environment. Sections and interactive collection records retain landmarks, stable item identity, commands, running/error state, and accessible actions. Application state owns domain selection and edits; the workspace owns transient sheet snap state.

The workspace resolves four width classes without changing application state:
Compact below 600 uses the bottom sheet; Medium from 600 through 839 uses a
non-modal overlay rail that leaves map context visible; Expanded from 840
through 1199 docks tools beside the canvas; Ultra-wide at 1200 or more adds an
optional, independently scrolling contextual detail column. Detail must be
supplemental rather than the sole home of required actions. Measured geometry
and text scale may intentionally collapse Ultra-wide to Expanded, or a docked
layout to Compact, when the declared canvas minimum cannot coexist with the
other surfaces.

Expanded workspace tools are content-measured within semantic minimum/maximum
metrics while reserving a minimum primary-canvas width. The tools surface owns
its vertical `ScrollView`; scrolling planner/settings controls never translates
the canvas. If text scaling makes the measured tools minimum incompatible with
the canvas minimum, the workspace reflows to its compact sheet instead of
clipping labels or shrinking the map below its usable bound.

Product identity is expressed through immutable light/dark semantic `design.Overrides`, never component-local colors. The resolver recalculates on-accent and focus contrast after overrides while high contrast remains platform controlled. Web CSS is emitted from same-origin endpoints and components contain no required inline style, satisfying the built-in self-only style/script CSP.
