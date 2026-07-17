---
type: concept
title: Component Family Packages
description: Public Go package boundaries for reusable component renderers.
tags: [components, go, packages, api]
timestamp: 2026-07-10T00:00:00Z
---

> **Ported from the GopherWeb bundle at the 2026-07-16 consolidation.** "GopherWeb" = `pkg/web`; CSS prefix is now `poem-`.
# Component Family Packages

The root `github.com/mulavdm/poem/pkg/web/components` package owns shared tokens and asset delivery. Renderers are grouped by responsibility: `action`, `form`, `layout`, `feedback`, `navigation`, `data`, and `widget`. Shared HTML escaping and validation live in `pkg/web/components/internal/markup`, which downstream projects cannot import.

This package layout keeps public imports explicit, avoids a catch-all renderer file, and lets family-level tests focus on their own contracts.

## Relationships

- [Public Component API](/concepts/web/public-component-api.md)
- [Downstream Consumption](/concepts/web/downstream-consumption.md)
