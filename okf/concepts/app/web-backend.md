---
type: concept
title: Web Backend
description: How web.Run drives a core.App[S] as a real, stateful GopherWeb-style web app — a forms-only, no-JavaScript-required state transport, plus GopherWeb's own vanilla JS for backend-local UI chrome like Modal.
tags: [architecture, web, github.com/mulavdm/gopherweb, backend, session]
timestamp: 2026-07-10T00:00:00Z
---

> **Ported from the Trellis/GopherWeb bundles at the 2026-07-16 consolidation.** Historical names map as: "Trellis" = `pkg/app`; "GopherWeb" = `pkg/web` (CSS prefix now `poem-`); "POEM" as a sibling project = the `pkg/render` engine layer. All are now this repository.
# Web Backend

`web.Run(app, addr, title)` (`pkg/app/web/driver.go`) starts a real `net/http` server. `web.NewHandler`
builds the same handler without binding a listener and accepts explicit timeouts, request limits,
cookie policy, session expiry, and a stable HMAC signing key. Unlike the POEM
backend, this side required real new machinery, because GopherWeb itself has no server-side
state or session concept — see
[Sister Project](/concepts/decisions/consolidation.md) for why that departure is scoped to this
module only, not proposed back to GopherWeb itself.

## Session state

Identity is a signed cookie (`pkg/app/web/session.go`): a random session ID plus an HMAC-SHA256
signature over it, using only `crypto/hmac`/`crypto/rand`/`encoding/hex` — no new dependency.
A tampered or missing cookie is rejected rather than trusted, and a fresh ID is minted.
`sessionStore[S]` (`web/store.go`) holds one `S` per session ID behind a mutex and delegates
state persistence to the public `StateStore[S]` contract. The default store is bounded and
single-process; `FileStateStore` provides atomic JSON persistence for a single host, while
multi-instance deployments require a transactional external implementation.

Each session also owns a process-local keyed-command executor. The loading-state update is saved
before a command starts; completion re-enters the reducer under the same serialized session path
and saves the final state. Session expiry or bounded-store eviction cancels active work. Commands
are not durable jobs: a process restart loses in-flight work even when application state uses a
durable `StateStore`.

`sessionMiddleware[S]` follows the same `func(http.Handler) http.Handler` shape and
unexported-context-key pattern as GopherWeb's own `middleware.RequestID` — the one piece of
GopherWeb's own middleware convention this backend deliberately mirrors, even though
GopherWeb's own middleware package is not used to build it (session state isn't a concept
`github.com/mulavdm/gopherweb/middleware` has).

The built-in store is bounded and expires idle sessions, but remains single-process and does not
survive a restart. `NewHandlerWithStore` accepts a `StateStore[S]`; the standard-library
`FileStateStore` survives process restarts on one host, while a transactional database-backed
implementation is required for multi-instance deployments. Production callers should provide a
stable signing key and enable secure cookies.

## Forms-only event transport — no JavaScript required

Every interactive node renders as part of a real HTML form (`pkg/app/web/render.go`): a `ButtonNode`
becomes a real GopherWeb `action.Button` with `Type: "submit"` and `name`/`value` attributes
carrying its `Msg` name (via `components.Attrs`, so escaping and validity match GopherWeb's
own rules exactly — Trellis never emits an attribute GopherWeb's own `markup` package would
have rejected). A `TextInputNode`'s or `SelectNode`'s current value simply travels as an
ordinary form field on the next submit. A `CheckboxNode` is different in a way worth calling
out explicitly: an HTML checkbox's field is *absent* from the POST body entirely when
unchecked, not present with value `"false"` — so `pkg/app/web/render.go`'s `fieldKind`
(`valueField` vs. `checkboxField`) exists specifically to read a checkbox's state from
*presence*, not from a value. Because a lone checkbox or select has no way to submit itself,
every view with one needs its own submitting button (see the `examples/preferences` app —
its `Save` button exists for exactly this reason and would not be needed on the POEM backend,
where `Checkbox`/`Select` fire `OnChange` immediately on interaction).

`POST /__event` (`pkg/app/web/driver.go`'s `eventHandler`) enforces a per-session CSRF token, bounds the
request body, validates the submitted message against enabled controls in the current view, and
then decodes the submission, applies every
changed field's `Msg` first — via `collectFields` walking the just-computed `View` and
branching on `fieldKind` — then the submitting button's `Msg`, saves the resulting state to
the session, and redirects back to `/` (303, avoiding form-resubmission-on-refresh).

If that event started a command, the redirected page renders the application's loading state and
includes a conditional one-second `<meta http-equiv="refresh">`. The marker remains only while
that session has pending commands, so completion becomes visible without adding a fetch, polling,
or SSE endpoint and without requiring JavaScript. Completion-only message names are not accepted
from forms because event validation still allows only messages present on enabled nodes.

`ImageViewportNode` is progressively enhanced by `/__poem/image-viewport.js`: Pointer Events
apply drag and true two-pointer pinch transforms locally, wheel zoom is debounced, and only the
final controlled value is committed. Short background taps commit a strict normalized point;
accessible marker buttons commit only an ID that exists at their current node path. The
CSRF-protected `POST /__field` route revalidates the current node path, enabled state, field kind,
strict payload, and marker identity before dispatching
the node's current `OnChange` message. It then uses the same redirect/loading path. Without
JavaScript the image and application-provided zoom/fit buttons remain usable.

`ResponsiveNode` renders Compact as the no-JavaScript baseline. `/__poem/responsive.js` uses
`matchMedia` to toggle Compact and Wide fieldsets, setting both `hidden` and `disabled` so controls
in the inactive branch cannot submit. Both branch paths are collected from the current View, which
allows resize without a server round trip while preserving normal event validation.

This is a deliberate design choice, not a placeholder: it matches GopherWeb's own
no-JS-baseline philosophy exactly, and it needed no client-side event protocol to design or
secure. A `fetch`-based progressive-enhancement layer — intercepting the same form submissions
to avoid a full page reload — is real, tracked future work that this baseline does not depend
on. "No JavaScript required" describes this state transport specifically, not a rule that
Trellis avoids JavaScript altogether — see `ModalNode` below, and the corrected framing in
`okf/log.md`: this transport was originally verified with `curl` alone during development,
which is precisely why the two bugs below went uncaught until a real browser was used.

## `ModalNode`: GopherWeb's own vanilla JS, not a Trellis-authored equivalent

`ModalNode` (`pkg/app/web/render.go`) renders via `widget.Modal{ID, Title, ContentHTML, Trigger:
action.Button{Text: n.Trigger}}.HTML()` — GopherWeb's own component handles the backdrop, the
`hidden` toggle, the focus trap, Escape, and its own `×` close button entirely client-side, via
GopherWeb's existing `components.js`, not anything Trellis wrote. Its open/closed state never
round-trips through `POST /__event` at all (see
[Architecture Overview](/concepts/app/overview.md) for why that's correct, not a gap) — the
trigger renders as `type="button"` (GopherWeb's default when `Type` is unset), so it's
harmless nested inside the page's own state-carrying `<form>`; it never submits it. `Msg`-
bearing content nested inside the modal (a `Checkbox`, a `Select`, a real submit `Button`)
still goes through the normal `collectFields`/`eventHandler` path — `collectFields` has a
`core.ModalNode` case that walks `n.Content` exactly like `ContainerNode`'s does, so fields
are discovered regardless of whether the modal happens to be open or closed at submit time.

## Two bugs a real browser caught that `curl` couldn't

Building `ModalNode` required verifying it with an actual browser (see the settings example's
verification in `okf/log.md`) rather than `curl` alone, and that surfaced two real,
pre-existing bugs in every prior example, both now fixed in `pkg/app/web/shell.go`:

1. **No `<form>` element at all.** `{{.Body}}` was placed directly in `<body>` — every
   `type="submit"` button Trellis had ever rendered (`Increment`, `Save`) was inert in a real
   browser, since a submit button outside a `<form>` submits nothing. Invisible to every prior
   `curl`-based check because `curl` posts directly to `/__event` regardless of what the
   rendered HTML actually contains. Fixed by wrapping `{{.Body}}` in
   `<form method="post" action="/__event">`.
2. **`components.js` was never linked**, only `components.css` — so none of GopherWeb's own
   JS behaviors, `bindModals` included, were ever wired up; a modal's trigger was inert
   regardless of the form fix. Fixed by adding `<script src="/components/static/components.js"
   defer></script>` to the shell. `components.js` self-initializes on `DOMContentLoaded`
   (`GopherWebComponents.init(document)`), so no extra call was needed once it was linked.

Both are exactly the class of bug this stress-testing approach exists to find: real, load-
bearing gaps that only show up when the thing is actually driven the way an end user would
drive it, not when it's exercised through a shortcut (`curl`) that happens to bypass the part
that was broken.

## Page shell

GopherWeb's own `internal/render`, `internal/router`, `internal/handlers`, and `internal/app`
packages are genuinely Go `internal/` and cannot be imported from a separate module — this
isn't a style choice Trellis could ask GopherWeb to relax, it's a compiler-enforced boundary.
`pkg/app/web/shell.go` therefore owns a small, independent HTML document shell, and links GopherWeb's
real public `components/static/components.css` and `components/static/components.js` (served
by mounting GopherWeb's own `components.AssetHandlerWithOptions` unmodified) so a
Trellis-driven page looks and behaves consistently with a hand-written GopherWeb one.
