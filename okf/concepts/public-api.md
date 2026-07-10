---
type: Concept
title: Public API Surface
description: The stable contract downstream apps consume from pkg/render.
tags: [public-api, contract, pkg-render]
timestamp: 2026-07-10T00:00:00Z
---
# Public API Surface

Downstream apps import `go_native_gpu_gui/pkg/render` and call `render.Run(render.AppConfig{...})`. This is POEM's one stable public contract; consumers do not need to know whether native presentation is implemented in C++, Rust, or another sidecar underneath it.

`pkg/render` re-exports subpackage types, constants, and options via Go type aliasing (`render.go`) so consumers never import subpackages (`components`, `layout`, `types`, `protocol`, `state`) directly.

Treat every exported symbol on this surface as a contract: add Go doc comments to it, and document deliberate breaking changes in the same commit that makes them.

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Sidecar Protocol](/concepts/protocol.md)

# Citations
- [README.md](../../README.md)
- [AGENTS.md](../../AGENTS.md) — Public API Stability
