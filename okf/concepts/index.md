# Concepts

* [Application Layer](/concepts/app/index.md) — the public `pkg/app` API: `App[S]`, serializable `Msg`s, the `Node` set, and the desktop/web drivers
* [Architecture](/concepts/architecture/index.md) — the Go engine / native-presenter split, threading, rendering, and UI subsystems (including the Android presenter)
* [Web Engine](/concepts/web/index.md) — the `pkg/web` server-rendered component library and its trust boundary
* [Public API Surface](/concepts/public-api.md) — what downstream apps consume, and the app-layer vs. engine-layer contract
* [Sidecar Protocol](/concepts/protocol.md) — the binary wire protocol between the Go engine and native presenters
* [Automation](/concepts/automation/index.md) — the HTTP inspection/automation surface
* [Decisions](/concepts/decisions/index.md) — architectural decisions and their rationale
