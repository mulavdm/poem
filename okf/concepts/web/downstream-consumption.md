---
type: concept
title: Downstream Consumption
description: How a project outside this repository imports the component families and serves their assets.
tags: [components, go, integration, assets]
timestamp: 2026-07-23T00:00:00Z
---
# Downstream Consumption

The component library is usable two ways: through the `pkg/app` web driver,
which renders a semantic node tree into these components automatically, or
directly from hand-built server-rendered pages. This concept covers the second
case — what an external project must import and serve.

## Importing

Import the family packages you actually render, not a catch-all:

```go
import (
    "github.com/mulavdm/poem/pkg/web/components"
    "github.com/mulavdm/poem/pkg/web/components/action"
    "github.com/mulavdm/poem/pkg/web/components/form"
)
```

The root package supplies `Variant`, `Size`, and `Attrs`; each family supplies
its own renderers. `pkg/web/components/internal/markup` is unavailable outside
this repository by construction — its escaping is reachable only through the
renderers, which is what keeps the guarantee in
[Trusted HTML](/concepts/web/trusted-html.md) intact.

## Serving assets

The CSS and JavaScript are embedded in the binary, so there is no separate build
step and no asset pipeline to configure. Mount the handler and reference the two
files:

```go
mux.Handle("GET /components/static/",
    http.StripPrefix("/components/static/", components.AssetHandler()))
```

```html
<link rel="stylesheet" href="/components/static/components.css">
<script src="/components/static/components.js" defer></script>
```

`/components/static/` is the path this repository's own web driver uses
(`pkg/app/web/driver.go`, with the tags emitted by `shell.go`). Nothing in the
library requires that prefix — it is a convention worth matching so examples
transfer.

Use `AssetHandlerWithOptions` to set `Cache-Control`. Only use immutable caching
behind versioned URLs; see [Asset Delivery](/concepts/web/component-asset-delivery.md).

## Initialising behavior

`components.js` calls `PoemWebComponents.init(document)` on `DOMContentLoaded`.
Applications that inject markup afterwards — HTMX swaps, client-side routing,
templates rendered into a container — must call `PoemWebComponents.init(root)`
again for the new subtree. The call is idempotent: bindings are tracked in a
`WeakMap`, so re-initialising an already-bound element does not attach a second
listener. See [Vanilla Class-Based JS](/concepts/web/vanilla-class-based-js.md).

## What you get without JavaScript

Every widget keeps a working non-JavaScript baseline: disclosures use native
`details`, forms use browser validation, tables stay readable, and links and
buttons remain ordinary HTML controls. A downstream project that never serves
`components.js` still renders a usable, accessible page — the script only adds
enhancement. See [Accessibility](/concepts/web/accessibility.md).

## Version expectations

The exported Go API, the `poem-` CSS classes, and the `data-poem-*` attributes
are pre-v1 contracts, described in
[Public Component API](/concepts/web/public-component-api.md). They may change
before v1; breaking changes are recorded in this bundle's log, and downstream
projects are expected to update in step rather than pin indefinitely.
