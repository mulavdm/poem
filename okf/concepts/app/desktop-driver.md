---
type: concept
title: POEM Backend
description: How poem.Run drives a core.App[S] as a real running POEM desktop window.
tags: [architecture, poem, backend]
timestamp: 2026-07-10T00:00:00Z
---

> **Ported from the Trellis/GopherWeb bundles at the 2026-07-16 consolidation.** Historical names map as: "Trellis" = `pkg/app`; "GopherWeb" = `pkg/web` (CSS prefix now `poem-`); "POEM" as a sibling project = the `pkg/render` engine layer. All are now this repository.
# POEM Backend

`poem.Run(app, config)` (`poem/driver.go`) holds the running `S` in a closure and installs it
as `config.BuildPagesFn` before calling `render.Run(config)` — every other `AppConfig` field
passes through unchanged.

## Why no new integration mechanism was needed

POEM's own render loop is already a "view = f(state)" model: `BuildPagesFn` is called once at
startup and again on every repaint, and real POEM apps (`cmd/gallery`, `cmd/engine`) already
throw away and rebuild their whole `Pages[...]` tree from scratch on every call rather than
mutating components in place. `poem.Run`'s `BuildPagesFn` implementation does exactly the same
thing: call `app.View(state)`, walk the returned `Node` tree (`pkg/app/desktop/render.go`'s `build`),
construct real `*components.Button`/`*components.TextInput`/`*layout.FlexBox` instances, and
assign the result into `rstate.Pages[rootPage]`.

Each interactive node's real POEM callback (e.g. `Button.OnClick`) is a closure that calls
`app.Update(state, msg)` and stores the result. No explicit repaint call is needed for this
path: it runs inside POEM's own mouse/key dispatch, which already marks the frame dirty
afterward. A future addition that mutates state from outside POEM's own event dispatch (a
timer, an async I/O callback) would need `render.RequestRepaint()` — POEM's own public,
documented hook for exactly that case — since nothing yet in this backend does that.

## Component IDs

Every real POEM component needs a stable `CompID`. Since the whole tree is rebuilt from
scratch on every call — matching POEM's own convention, not inventing a new one — IDs are
derived from tree position rather than any node identity Trellis tracks itself:
`childPath(parent, index)` produces `"parent/index"` recursively from the root path constant
(`rootPage = "trellis-root"`).

## Current node coverage

`build` (`pkg/app/desktop/render.go`) handles `TextNode` (`render.NewLabel`), `ButtonNode`
(`render.NewButton`, wiring `Disabled`), `TextInputNode` (`render.NewTextInput`, wiring
`Value` and `OnChange`), `TextAreaNode` (`render.NewTextArea`), `CheckboxNode`
(`render.NewCheckbox`, encoding its bool via `core.BoolPayload`), `SwitchNode`
(`render.NewSwitch`, same bool encoding), `SelectNode` (`render.NewSelect`, leaving
`Open`/`OnOpenChange` unset — see [Architecture Overview](/concepts/app/overview.md) for why),
`RadioGroupNode` (one `render.NewRadio` per option in a `FlexBox`, all sharing the group's
`OnChange`), `SliderNode` (`render.NewSlider`, encoding its `float64` via `core.FloatPayload`,
converting to POEM's `float32`), `BadgeNode` (`render.NewBadge`, mapping `core.Variant` onto
POEM's theme variant), `ProgressBarNode` (`render.NewProgressBar`), `TableNode`
(`render.NewDataTable`, read-only), `AccordionNode` (`render.NewAccordion` with a nil
`OnToggle`, so it self-manages expansion in POEM's transient store — see Component Parity),
`TabsNode` (a **controlled** `render.NewTabs` strip — `App[S]` owns `Selected` — plus a
`FlexBox` holding only the active tab's content), `ContainerNode` (`render.FlexBox`, wiring
`Direction`/`Gap`/`Padding`), and `ModalNode` (see below). See
[Architecture Overview](/concepts/app/overview.md) for the full `Node` set and the policy on
adding more.

## `ModalNode` and the overlay system

`ModalNode` is the one kind `build` doesn't return as-is: `buildModal` (`pkg/app/desktop/render.go`)
returns the *trigger* button, since a real `render.Modal` never lives in `Pages[...]` — it
lives in POEM's `OverlayManager`, reached only via `ApplicationState.OpenModal(...)`
(`pkg/render/types/overlay.go:126-129`). Opening happens as a one-shot side effect inside the
trigger's `OnClick`, not from `BuildPagesFn`'s own execution — see
[Architecture Overview](/concepts/app/overview.md) for why that specific placement matters
(it's what avoids resetting the modal's focus on every unrelated repaint). `OnDismiss` is left
`nil`: real keyboard Escape already closes the overlay via `run.go`'s global
`DismissTopOverlay()` handler and `OpenModal`'s `dismissOnEscape=true`, independent of
`Modal.OnDismiss` — and since the modal's open/closed state was never tracked in `App[S]` to
begin with, there's nothing to keep in sync on dismiss.
