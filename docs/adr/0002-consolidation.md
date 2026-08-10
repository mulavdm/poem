# Consolidation: one repo, one application API

As of 2026-07-16, the three sibling projects are one. GopherWeb's component/middleware
library lives at `pkg/web` (branding stripped: `gw-` → `poem-`, module path retired);
Trellis's application layer lives at `pkg/app` (the `App[S]`/`Msg`/`Node` model) with its
backend drivers at `pkg/app/desktop` and `pkg/app/web`; the Android path uses `pkg/mobile` +
`android_engine`. The examples (`examples/counter`, `preferences`, `settings`) each run on
all three targets from one Go definition. The Trellis and GopherWeb repositories are archived
at their final `v0.2.0` tags.

## What this supersedes

- **`gopherweb-parity.md`** ("mirrored parallel APIs, permanently, two codebases"): the
  separation it defended stopped paying for itself once Trellis existed — three repos meant
  sibling CI checkouts, a private cross-repo token, tag-ordering contracts, and a
  hand-maintained parity matrix whose entire content was "POEM's catalog, mirrored." Its two
  *structural* findings survive and are honored here, not overturned (see below).
- **Trellis's `sister-project.md`** ("a third module, separate from both engines"): the
  layer was right; the project boundary was the overhead.

## The API decision

The single public application API is **`App[S]`/`Node`** (`pkg/app`), not the component API.
The component API (`pkg/render`) remains public but is documented as the engine layer.

The deciding constraint is the one `gopherweb-parity.md` recorded as the state-model blocker:
component event handlers are Go closures carrying `*types.ApplicationState`. Closures cannot
travel the web backend's HTTP boundary and cannot be serialized for durable sessions — so a
component-API-everywhere design would have cost the web target its signed-cookie sessions,
`FileStateStore`, and restart persistence, and required a per-session native runtime.
`App[S]`'s serializable state and named `Msg`s are the documented solution to that blocker,
which is why the layer survived the merge even as its repo did not.

Consequences:

- Layering is `developer → pkg/app (one API) → engine layer per target` — components +
  presenters natively, `pkg/web` components on the web. Adding engine capability to the app
  API is expected, ordinary work (tracked in-repo), no longer cross-repo parity discipline.
- The trust-boundary finding also survives: `pkg/web` keeps its caller-trusted
  `...HTML template.HTML` fields with the same documented sanitization obligations, and the
  `Node` IR still exposes no raw-markup injection point.
- One module, one CI, one release tag. The sibling checkout token and tag-ordering contract
  in the old `RELEASING.md`s are dead.
