# Application Layer (`pkg/app`)

* [Adaptive Native Design](/concepts/app/adaptive-design.md) â€” semantic nodes and commands, token resolution, platform profiles, adaptation, and design lint

The public application API: `App[S]` (serializable state, pure `View`/`Update`), named
serializable `Msg`s, and the closed `Node` set every target renders. Ported from the Trellis
bundle at the 2026-07-16 consolidation.

* [Overview](/concepts/app/overview.md) — `app.App[S]`, the `Node` IR, why events are named messages instead of closures, and the non-string payload conventions
* [Component Parity](/concepts/app/component-parity.md) — what the `Node` set covers of the desktop (`pkg/render`) and web (`pkg/web`) catalogs, and what is portable, excluded, or backend-specific
* [Native Configuration Driver](/concepts/app/desktop-driver.md) — how `desktop.Configure` adapts `App[S]` for Windows and Android process hosts
* [Web Backend](/concepts/app/web-backend.md) — sessions, the forms-only no-JS event transport, and the page shell
