# Application Layer (`pkg/app`)

The public application API: `App[S]` (serializable state, pure `View`/`Update`), named
serializable `Msg`s, and the closed `Node` set every target renders. Ported from the Trellis
bundle at the 2026-07-16 consolidation.

* [Overview](/concepts/app/overview.md) — `app.App[S]`, the `Node` IR, why events are named messages instead of closures, and the non-string payload conventions
* [Component Parity](/concepts/app/component-parity.md) — what the `Node` set covers of the desktop (`pkg/render`) and web (`pkg/web`) catalogs, and what is portable, excluded, or backend-specific
* [Desktop Driver](/concepts/app/desktop-driver.md) — how `desktop.Run` drives the native engine through `BuildPagesFn`
* [Web Backend](/concepts/app/web-backend.md) — sessions, the forms-only no-JS event transport, and the page shell
