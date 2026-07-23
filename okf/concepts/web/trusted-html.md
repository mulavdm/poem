---
type: concept
title: Trusted HTML Contract
description: Rules for passing rich HTML into public components.
tags: [components, security, html]
timestamp: 2026-07-10T00:00:00Z
---

> **Ported from the Trellis/GopherWeb bundles at the 2026-07-16 consolidation.** Historical names map as: "Trellis" = `pkg/app`; "GopherWeb" = `pkg/web` (CSS prefix now `poem-`); "POEM" as a sibling project = the `pkg/render` engine layer. All are now this repository.
# Trusted HTML Contract

Ordinary component strings are escaped before rendering. Fields ending in `HTML`, such as `Card.BodyHTML`, are intentionally caller-trusted `template.HTML`; callers must sanitize or construct that markup from trusted application code before passing it to GopherWeb.

Where practical, components provide escaped text alternatives such as `Card.Body`, `Modal.Content`, `Disclosure.Text`, and `TabItem.Text`. Use those alternatives for ordinary text.

## Relationships

- [Public Component API](/concepts/web/public-component-api.md)
- [Downstream Consumption](/concepts/web/downstream-consumption.md)
