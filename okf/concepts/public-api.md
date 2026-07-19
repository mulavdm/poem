---
type: Concept
title: Public API Surface
description: The stable contract downstream apps consume from pkg/render.
tags: [public-api, contract, pkg-render]
timestamp: 2026-07-10T00:00:00Z
---
# Public API Surface

## M8 startup and workspace contract

`App[S].Start` is an optional once-only reducer hook. Native drivers invoke it once per hosted engine lifetime; the web driver invokes it only when creating a session, never when restoring one. Immediate state is visible in the first view and its `Cmd` uses the ordinary keyed executor.

`WorkspaceNode` carries semantic title/subtitle, header commands, status, primary content, and tools. Tools dock beside content when expanded and become a component-owned bottom sheet on compact touch layouts; the no-JavaScript web baseline keeps them as an ordinary accessible section. `SectionNode`, action importance/placement, interactive collection items, and semantic map/location/connection icons preserve intent through native and web lowering.

`ImageViewportCommit` adds validated logical width/height to the controlled transform. Legacy transform-only payloads remain valid. Applications can use immutable `design.Overrides` through `System.WithOverrides`; resolution recomputes contrast-safe dependent colors and does not override forced/high-contrast system semantics.

## M6 semantic design contract

`App[S]` optionally carries `Design *design.System` and `Commands func(S) []app.Command`. Semantic actions reference state-derived commands by stable ID while invocation remains a transport-safe `Msg`. The semantic catalog covers labelled fields, actions, collections, destinations/navigation, status, progress, disclosure, controlled dialogs, forms, workspaces, media/viewports, and four-class `AdaptiveNode`. Application nodes carry stable accessible identity and no raw visual styling. `pkg/render` remains available for advanced native components and receives the same resolved design environment.

Since the 2026-07-16 [consolidation](/concepts/decisions/consolidation.md), POEM has two
public surfaces at different altitudes:

**`pkg/app` — the application API.** Applications import `github.com/mulavdm/poem/pkg/app`
and write an `App[S]` (serializable state, pure `View`/`Update`, named `Msg`s, keyed asynchronous
`Cmd`s), launched via `pkg/app/web.Run`, registered for Windows through
`desktop.Configure` plus `pkg/windows`, or hosted on Android through `pkg/mobile.Start` from a
`c-shared` main). This is the recommended way to build applications: one definition runs on
every target. See [Application Layer](/concepts/app/index.md).

`Update` returns `(S, Cmd)`. A zero `Cmd` means no work; a non-zero command runs with a
cancellable context and returns a runtime-local typed value or error through a completion
`Msg`. Its name is also its concurrency key: different names may overlap, while a newer command
with the same name cancels and supersedes the older result.

The catalog includes `ImageViewportNode`, a reusable controlled and annotated image viewport. Its
`ImageTransform` uses viewport-relative offsets and scale, normalizes zero scale to one, and
defaults to a 0.5–8 interactive range. `ImageTransformPayload` and `Msg.ImageTransform` are the
strict cross-target encoding contract; malformed, non-finite, and out-of-envelope values are
rejected before they reach reducers.
`ViewportPointPayload`/`Msg.ViewportPoint` provide the same strict validation for normalized
background activation. `ImageMarker` annotations carry stable IDs, normalized image coordinates,
accessible labels, semantic variants, selection/disabled state, and their own activation message.
Tap, marker selection, and drag/pinch are mutually exclusive.

`ResponsiveNode` selects Compact or Wide node groups from available logical width (600 by default).
It is layout-only and adds no application state; the web target keeps Compact as its no-JavaScript
baseline and disables the inactive branch's form controls.

**`pkg/render` — the engine layer.** Advanced native-only apps may create a
`render.AppConfig` with a `BuildPagesFn` (`cmd/gallery` does), then register it through
`pkg/windows`. Its component event handlers are Go closures, so
this surface cannot drive the web target — the reason `pkg/app` exists. `pkg/render`
re-exports subpackage types via aliasing (`render.go`) so consumers never import
`components`/`layout`/`types`/`protocol`/`state` directly. Consumers do not need to know
which native presenter (Windows D3D11 host or Android GLES host) runs underneath.

**`pkg/windows` — the native Windows host ABI.** Windows c-shared mains call
`MustRegister` with configured application state and validated metadata. The package exports
ABI v1 metadata/start/read/write/stop functions to the generic adjacent host. This is a deliberate
M7 replacement for `render.Run`, `desktop.Run`, sidecar resolution, and embedded presenter assets.

`pkg/web` (HTML components + middleware) is public for hand-built server pages, and is what
the web driver renders through.

Treat every exported symbol on these surfaces as a contract: add Go doc comments, and
document deliberate breaking changes in the same commit that makes them.

## See also
- [Application Layer](/concepts/app/index.md)
- [Architecture Overview](/concepts/architecture/overview.md)
- [Native Presenter Protocol](/concepts/protocol.md)
- [Single-Process Windows Host](/concepts/architecture/windows-host.md)

# Citations
- [README.md](../../README.md)
- [AGENTS.md](../../AGENTS.md) — Public API Stability
