---
type: concept
title: Adaptive Native Design
description: Semantic application nodes, state-derived commands, token resolution, platform profiles, and design linting.
tags: [app, design, adaptive, accessibility, tokens]
timestamp: 2026-07-18T00:00:00Z
---
# Adaptive Native Design

`pkg/design` resolves foundation, semantic, and platform-profile tiers into immutable renderer tokens. `RenderingModeAuto` chooses the Windows, Android, or web profile and falls back to Adaptive House. Window classes are Compact below 600, Medium at 600â€“839, Expanded at 840â€“1199, and Ultra-wide at 1200 or more logical pixels. Density follows pointer precision independently of width.

`App[S]` optionally supplies a `Design` and derives a semantic `Commands` registry from state. Command invocation remains a validated `Msg`; external work remains a keyed `Cmd`. Semantic nodes carry stable IDs, accessible metadata, controlled state, and no application-authored colors or metrics. `AdaptiveNode` carries all four standard branches, while collections and navigation describe intent rather than fixed presentation.

Native `RenderContext` and advanced `pkg/render` components consume the same resolved theme and environment. The web target serializes these tokens as CSS variables with dark, forced-colors, and reduced-motion media behavior. Presenter capability events are appended without changing prior event numbers.

`pkg/app/designlint` exposes guide-coded diagnostics and a test assertion. Checked-in applications should have no diagnostics except explicit, reasoned allowlists during migration.

M8 completes the workspace composition contract: semantic header/status content surrounds a persistent primary canvas and a tools surface which docks, overlays, or becomes a component-owned bottom sheet by environment. Sections and interactive collection records retain landmarks, stable item identity, commands, running/error state, and accessible actions. Application state owns domain selection and edits; the workspace owns transient sheet snap state.

Product identity is expressed through immutable light/dark semantic `design.Overrides`, never component-local colors. The resolver recalculates on-accent and focus contrast after overrides while high contrast remains platform controlled. Web CSS is emitted from same-origin endpoints and components contain no required inline style, satisfying the built-in self-only style/script CSP.
