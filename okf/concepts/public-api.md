---
type: Concept
title: Public API Surface
description: The stable contract downstream apps consume from pkg/render.
tags: [public-api, contract, pkg-render]
timestamp: 2026-07-10T00:00:00Z
---
# Public API Surface

Since the 2026-07-16 [consolidation](/concepts/decisions/consolidation.md), POEM has two
public surfaces at different altitudes:

**`pkg/app` — the application API.** Applications import `github.com/mulavdm/poem/pkg/app`
and write an `App[S]` (serializable state, pure `View`/`Update`, named `Msg`s), launched via
`pkg/app/desktop.Run`, `pkg/app/web.Run`, or the Android path (`pkg/mobile.Start` from a
`c-shared` main). This is the recommended way to build applications: one definition runs on
every target. See [Application Layer](/concepts/app/index.md).

**`pkg/render` — the engine layer.** Advanced native-only apps may import
`github.com/mulavdm/poem/pkg/render` directly and call `render.Run(render.AppConfig{...})`
with a `BuildPagesFn` (`cmd/gallery` does). Its component event handlers are Go closures, so
this surface cannot drive the web target — the reason `pkg/app` exists. `pkg/render`
re-exports subpackage types via aliasing (`render.go`) so consumers never import
`components`/`layout`/`types`/`protocol`/`state` directly. Consumers do not need to know
which native presenter (C++ sidecar, Android GLES host) runs underneath.

`pkg/web` (HTML components + middleware) is public for hand-built server pages, and is what
the web driver renders through.

Treat every exported symbol on these surfaces as a contract: add Go doc comments, and
document deliberate breaking changes in the same commit that makes them.

## See also
- [Application Layer](/concepts/app/index.md)
- [Architecture Overview](/concepts/architecture/overview.md)
- [Sidecar Protocol](/concepts/protocol.md)

# Citations
- [README.md](../../README.md)
- [AGENTS.md](../../AGENTS.md) — Public API Stability
