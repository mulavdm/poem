---
type: concept
title: Component Accessibility
description: Semantic and keyboard behavior promised by the pkg/web components.
tags: [components, accessibility, javascript]
timestamp: 2026-07-10T00:00:00Z
---

> **Ported from the GopherWeb bundle at the 2026-07-16 consolidation.** "GopherWeb" = `pkg/web`; CSS prefix is now `poem-`.
# Component Accessibility

Components render semantic HTML first. JavaScript then enhances modals with focus management, tabs with arrow-key navigation, command palettes with keyboard selection, and tables with sort state. The public `pkg/web (formerly GopherWeb)Components.init(root)` function is idempotent and may be called after application DOM changes.

Widgets retain a meaningful non-JavaScript baseline: disclosures use native `details`, forms use browser validation, tables remain readable, and links and buttons remain ordinary HTML controls.

## Relationships

- [Vanilla Class-Based JS](/frontend/vanilla-class-based-js.md)
- [Public Component API](/concepts/web/public-component-api.md)
