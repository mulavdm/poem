---
type: concept
title: Public Component API
description: The exported Go surface of pkg/web/components and the pre-v1 contracts it commits to.
tags: [components, go, api, contracts]
timestamp: 2026-07-23T00:00:00Z
---
# Public Component API

The root `github.com/mulavdm/poem/pkg/web/components` package holds the shared
vocabulary every renderer family speaks. It deliberately contains no renderers
of its own — those live in the family packages described in
[Family Packages](/concepts/web/family-packages.md).

## Shared vocabulary

`types.go` defines the enums and attribute carriers used across every family:

| Type | Values | Purpose |
|---|---|---|
| `Variant` | `primary`, `secondary`, `danger`, `success`, `warning`, `neutral` | semantic visual treatment; `neutral` is the default |
| `Size` | `small`, `medium`, `large` | visual size; `medium` is the default |
| `Attrs` | `map[string]string` | additional HTML attributes on components that accept them |

`Attrs` is a trust boundary, not a passthrough. Event-handler attributes such as
`onclick` and syntactically invalid attribute names are dropped rather than
rendered, so a caller cannot inject script through the attribute map. Content
that must carry markup goes through the separate, explicitly caller-trusted
fields described in [Trusted HTML](/concepts/web/trusted-html.md).

## Asset surface

Three exported functions deliver the embedded CSS and JavaScript:

- `Assets() fs.FS` — the embedded `static/` tree.
- `AssetHandler() http.Handler` — serves it with no cache directive.
- `AssetHandlerWithOptions(AssetOptions) http.Handler` — same, with an explicit
  `CacheControl` header.

`AssetOptions.CacheControl` is left empty by default on purpose. Immutable
caching is only correct behind versioned URLs; defaulting to it would serve
stale component CSS after an upgrade. See
[Asset Delivery](/concepts/web/component-asset-delivery.md).

## What is contractual

Per the package documentation, the exported Go API **and** the `poem-` prefixed
CSS classes and `data-poem-*` attributes are intentional pre-v1 contracts. The
attributes are part of the API surface because the JavaScript layer binds to
them by selector — renaming `data-poem-modal-open` breaks callers exactly as
renaming an exported function would. Breaking changes to any of the three are
recorded in this bundle's log.

`pkg/web/components/internal/markup` holds the shared escaping and validation
helpers. Being under `internal/`, it is unavailable to downstream projects by
construction; the escaping guarantees are reachable only through the renderers.

## Relationships

- [Family Packages](/concepts/web/family-packages.md) — how renderers are grouped
- [Trusted HTML](/concepts/web/trusted-html.md) — the escaped/trusted field split
- [Downstream Consumption](/concepts/web/downstream-consumption.md) — importing and serving it
