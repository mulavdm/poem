---
type: concept
title: Architecture Overview
description: The core.App[S] model, the Node IR, and why Trellis's event model uses named messages instead of closures.
tags: [architecture, core, node, app]
timestamp: 2026-07-22T00:00:00Z
---

> **Ported from the Trellis/GopherWeb bundles at the 2026-07-16 consolidation.** Historical names map as: "Trellis" = `pkg/app`; "GopherWeb" = `pkg/web` (CSS prefix now `poem-`); "POEM" as a sibling project = the `pkg/render` engine layer. All are now this repository.
# Architecture Overview

An application author writes exactly one thing against Trellis: a `core.App[S]`
(`pkg/app/app.go`):

```go
type App[S any] struct {
	Init   S
	View   func(state S) Node
	Update func(state S, msg Msg) (S, Cmd)
}
```

`View` is a pure function computing a declarative `Node` tree from the current state. `Update`
is a pure reducer: given the current state and a fired `Msg`, it returns the next state. Two
backends — `poem.Run` and `web.Run` — each drive the same `App[S]` value as a real running
interface. See [POEM Backend](/concepts/app/desktop-driver.md) and
[Web Backend](/concepts/app/web-backend.md) for how each does it.

## The `Node` IR

`core.Node` (`pkg/app/node.go`) is a closed interface with twenty concrete kinds today:
`TextNode`, `ButtonNode`, `TextInputNode`, `TextAreaNode`, `CheckboxNode`, `SwitchNode`,
`SelectNode`, `RadioGroupNode`, `SliderNode`, `BadgeNode`, `ProgressBarNode`, `TableNode`,
`AccordionNode`, `TabsNode`, `ImageNode`, `ImageViewportNode`, `ResponsiveNode`, `ContainerNode`, `ModalNode`, and `OverlayNode`. Each backend's `render.go`
translates a `Node` tree into that backend's real component tree — POEM's `components.Button`/
`components.TextArea`/`components.Checkbox`/`components.Switch`/`components.Select`/
`components.Radio`/`components.Slider`/`components.Badge`/`components.ProgressBar`/
`components.DataTable`/`components.Accordion`/`components.Tabs`/`components.Modal`/
`layout.FlexBox`/`layout.Overlay`, GopherWeb's `action.Button`/`form.Textarea`/`form.Checkbox`/`form.Switch`/
`form.Select`/`form.RadioGroup`/`form.Range`/`feedback.Badge`/`feedback.Progress`/`data.Table`/
`widget.Disclosure`/`widget.FormTabs`/`widget.Modal`/`form.Input`. Node field names deliberately reuse the vocabulary POEM and
GopherWeb already converged on (`Text`/`Label`/`Value`/`Options`/`Disabled`/`Variant`), so both
translation functions are close to 1:1 field copies rather than needing their own
reconciliation layer. Which of each catalog's components are ported, portable, or deliberately
backend-specific is tracked in [Component Parity](/concepts/app/component-parity.md).

`Variant` (`pkg/app/node.go`) — carried by `BadgeNode` and any future semantic-status kind — is
its own small illustration of the reuse-the-converged-vocabulary rule: its members are exactly
the intersection of POEM's `theme.Variant` and GopherWeb's `components.Variant`. POEM's extra
`Subtle` is left out on purpose, because a shared IR should only name a treatment *both*
backends can actually honor; each backend's `render.go` maps the shared members onto its own
type with a total switch.

Adding a new `Node` kind means adding it here, then adding the matching case to both
backends' `render.go`. There is no partial-coverage state: a `Node` kind that only one
backend understands is not a supported kind.

`SelectNode` deliberately has no `Open`/`OnOpenChange` field, even though POEM's own `Select`
has one: reading POEM's `Select.setOpen`/`OnMouseDown` (`pkg/render/components/desktop.go`)
confirmed those are optional *controlled*-mode hooks — left unset, POEM's `Select` already
manages its own open/closed popup state internally, keyed by the node's stable path-derived
ID, exactly the way a native HTML `<select>` manages its own open/closed state without needing
anything from the page that embeds it. The shared IR doesn't need to represent state a backend
already owns for itself.

## `Msg`: named messages, not closures

`Msg{Name string, Payload string}` (`pkg/app/node.go`) is how an interactive `Node` describes
"what happens when I fire." This is not a Go closure, and that's deliberate, not a
simplification taken for convenience: POEM's `Button.OnClick` is an in-process closure with
direct access to the whole running app, but GopherWeb's browser-side click has to cross an
HTTP boundary to reach the server, and a closure cannot travel over HTTP. A shared event model
has to use the more restrictive form — a named, serializable message — uniformly, even on the
POEM target where the richer closure form would technically work. Each backend's driver
translates a fired `Msg` into a call to `app.Update`, then re-renders from the resulting
state; neither backend lets an `OnClick`/`OnChange` handler skip `Update` and mutate state
directly.

## `Cmd`: typed asynchronous work without concurrent state mutation

`Update` may return `Cmd{Name, Run}`. `Run(context.Context) (any, error)` executes outside the
serialized reducer/view path; its completion returns as `Msg{Name: cmd.Name, Value: value,
Err: err}`. `Value` and `Err` are runtime-local and are never populated from an HTTP form.
Browser events therefore keep the restricted `Name`/`Payload` wire shape, while command results
can carry route structures, image bytes, or other typed application values without encoding them
through `Payload`.

The command name is also its concurrency key. Different names may execute concurrently. Starting
a newer command with the same name cancels the older context and advances a generation; a late
older result is discarded even when that command ignored cancellation. Reducers and views remain
serialized on every target, commands may chain by returning another command from their completion,
and in-flight work is process-local rather than durable across restart.

### Non-`string` payloads: one explicit convention, not a type system

`Msg.Payload` stays `string` — it has to, to stay serializable across the web backend's HTTP
boundary. `CheckboxNode` was the first `Node` kind whose real payload is a `bool` (POEM's own
`Checkbox.OnChange` is `func(bool, *types.ApplicationState)`), so booleans travel as the
literal strings `"true"`/`"false"`, via `BoolPayload(bool) string` and `Msg.Bool() bool`
(`pkg/app/node.go`). This is a real constraint the `Text`/`Button` kinds never exercised, stated
plainly rather than smoothed over: any future non-string-payload `Node` kind needs its own
explicit encode/decode convention here, the same way this one does — there is no generic
typed-payload mechanism, by design, since that would reopen the serializability constraint
`Msg` exists to satisfy.

`SliderNode` is the second kind to exercise this, with a `float64` payload (POEM's `Slider`
is `func(float32, ...)`): `FloatPayload(float64) string`/`Msg.Float() float64` encode it as a
plain decimal — no scientific notation — so both the web transport's string round trip and an
`<input type="range">`'s own posted value parse back to the same number. It is a separate
convention from `BoolPayload`, not a generalization of it: that `Msg` now has two such pairs
is the design working as intended, each numeric kind declaring exactly what it needs, rather
than the two being folded into one typed-payload abstraction.

On the web backend specifically, a checkbox's posted form field is a second wrinkle beyond the
encoding: an HTML checkbox is *absent* from the POST body when unchecked, not
present-with-value-`"false"`, unlike every other field kind. `pkg/app/web/render.go`'s `fieldKind`
(`valueField` vs. `checkboxField`) exists because of this — presence, not a posted value, *is*
the payload for a checkbox field.

`ImageViewportNode` adds one strict JSON payload convention for its controlled
`ImageTransform{OffsetX, OffsetY, Scale}`. Offsets are viewport fractions, positive values move
the image right/down, and zero scale normalizes to one. `ImageTransformPayload` and
`Msg.ImageTransform` reject unknown fields, trailing data, non-finite numbers, and values outside
the transport envelope; the node's default interactive scale range is 0.5–8.

The viewport also supports mutually exclusive activation and navigation. A short background
click/tap emits a strict normalized `ViewportPoint`; a drag or pinch emits only the transform;
an `ImageMarker` hit emits that marker's stable ID. Marker positions follow the transformed image
while their accessible hit targets stay at least 44 logical pixels. Web field commits are resolved
against the current node tree, so forged marker IDs and stale paths are rejected.

`ResponsiveNode` selects Compact or Wide children from assigned width,
with a 600 logical-pixel default breakpoint. Native layout makes the choice during measurement and
draw. Web renders both branches but keeps only one enabled; Compact is the usable no-JavaScript
baseline and a small `matchMedia` enhancement switches branches without application state.

`OverlayNode` is the other structural node: it z-stacks floating `OverlayLayer`s over a `Base`
within one box. The base fills the box and alone determines its measured size; each layer is sized
to itself and pinned to one of nine anchors, inset from the edges it hugs, so it floats without
affecting layout. Layers paint last-on-top and are hit-tested before the base — a floating control
wins the pointer over the canvas beneath it, a miss falls through. It is the non-modal counterpart
to `ModalNode`, for controls that act *on* a canvas (map zoom/fit clusters, a floating action
button) rather than beside it. See [Desktop Driver](/concepts/app/desktop-driver.md) for how each
backend renders it and why the underlying canvas must fit its viewport.

## `ModalNode`: the first `Node` that isn't part of `App[S]` at all

Every other `Node` kind's interactivity flows through `Msg` → `app.Update` → a new `View`.
`ModalNode` (`pkg/app/node.go`) doesn't: it has no `Msg` field of its own, because its open/closed
state legitimately isn't application state — it's UI chrome both backends already have their
own correct, idiomatic way to manage without any help from a shared model:

- On POEM, real apps (`cmd/gallery`) already open a `Modal` from directly inside a button's
  `OnClick` — a one-shot call to `state.OpenModal(...)` triggered by an actual click, never
  from `BuildPagesFn`'s own per-repaint execution. `pkg/app/desktop/render.go`'s `buildModal` does the
  same thing: the trigger button's `OnClick` closure calls `rstate.OpenModal(...)` itself.
  This matters beyond just matching convention — calling `OpenModal` again on every repaint
  while a modal is already open would replace the whole `OverlayEntry`
  (`pkg/render/types/overlay.go`, confirmed via `overlay_test.go`'s
  `TestOverlayManagerReplacesStableIDAndRestoresTopOrder`), resetting `RestoreFocusID` and
  stealing focus from wherever the user just tabbed to. Since a `ButtonNode`'s `OnClick` only
  ever *runs* on an actual click regardless of how many times `BuildPagesFn` rebuilds the
  surrounding tree, wiring the trigger the same way sidesteps that failure mode for free — no
  open-modal-ID tracking needed anywhere in `poem.Run`.
- On the web backend, `widget.Modal` opens and closes entirely client-side via
  `data-poem-modal-open`/`-close`, with zero server round trip, exactly as GopherWeb intends.

**The consequence, stated plainly**: a `ModalNode`'s content is a snapshot from the moment it
was opened, not continuously live against later unrelated state changes, on both backends —
exactly matching how real POEM apps already use `Modal`/`Dialog` today. `Msg`-bearing content
*nested inside* the modal (a `Checkbox`, a `Select`, a real submit `Button`) still fires
through the normal `Update` pipeline once submitted; it's only the modal's own toggle that
never touches `App[S]`.

`AccordionNode` is the second kind built this way: which sections are expanded is backend-local
chrome, not `App[S]`. The web backend uses GopherWeb's native `<details>` (client-side toggle,
no round trip); POEM's `Accordion` self-manages expansion in its transient store when
uncontrolled — a POEM engine change made for this, so the state survives the per-frame rebuild
the same way `Select` popups and `Tabs` selection already do. Unlike a modal, an accordion
section's content stays continuously live against `App[S]` (it's rebuilt every frame, only its
visibility is chrome). See [Component Parity](/concepts/app/component-parity.md) for the general
pattern and which composite components remain.

`TabsNode` is the deliberate counter-example: its `Selected` **is** part of `App[S]`, because an
application legitimately needs to set the active tab (unlike a modal's or an accordion's open
state). Whether a composite's state is chrome or `App[S]` is a per-component trade — instant
client-side toggling versus author control at the cost of a web round trip — not a property
forced by the backends differing. See
[Composite Components](/concepts/decisions/composite-components.md) for that decision and its
consequences.
