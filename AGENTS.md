# Agent Instructions: POEM

This document contains project-specific architectural rules, debugging knowledge, and operational context for POEM. Agents working on this project should use it together with the workspace-level instructions and the human-facing `README.md`.

## How To Use This File

Treat this file as the repo-level source of truth for agent behavior and guardrails inside `POEM`. Use the root `README.md` for the high-level human-facing project overview. For deeper runtime, UI-authoring, and automation detail, go directly to the OKF bundle: [okf/concepts/architecture/index.md](okf/concepts/architecture/index.md) and [okf/concepts/automation/index.md](okf/concepts/automation/index.md). `ARCHITECTURE.md`, `GUIDE.md`, and `docs/AUTOMATION.md` no longer exist — their content was fully subsumed into that bundle.

## Universal Engineering Principles

- **Strict Scope Control**: Keep changes strictly scoped to the user's request. Do not execute gratuitous refactoring or rewrite unrelated code while fixing a specific issue.
- **Contextual Blending**: Before writing any code, read all relevant files. Your changes must blend in with the surrounding codebase's existing patterns.
- **Safe Operations**: Preserve user changes and avoid destructive git or filesystem operations unless explicitly requested.
- **Anti-Indirection**: Prefer straightforward modules, simple functions, plain data structures, and native types over heavy indirection or framework-heavy abstractions.
- **Minimal Dependencies**: Start with the standard library of the language in use. Avoid tiny helper dependencies for trivial code (no micro-packages). Reject dependencies that force the repo toward an unwanted framework shape.
- **Clear Data Ownership**: Keep data flow, ownership, lifecycle, and side effects explicit and easy to follow.
- **Immutability**: Treat shared state and blackboards as immutable once published. Produce new data rather than mutating in place.
- **Measure Everything**: Always implement profiling, benchmarking, and maintain zero/low-allocation routing in hot paths. Measure before merging.
- **Security by Design**: Incorporate defensive engineering, red-team testing, or attack vector simulations for any newly added feature.
- **Robust Lifecycles**: Do not block routing threads. Native hosts must use cooperative cancellation, bounded shutdown, and explicit startup-failure reporting; never unload an active Go runtime.
- **Strict Locking Discipline**: Use language-appropriate read-write locks correctly—prefer read locks for reads, and isolate write locks strictly to mutation operations.
- **Atomic Updates**: When APIs, configuration schemas, or behaviors change, update the relevant documentation (`README.md`, architecture docs) in the exact same commit. Documentation is part of the code.
- **Truthful & Living Documentation**: Treat architecture documents as living specifications. Remove stale references to deprecated features or deleted projects. Do not turn brainstorms or speculative plans into present-tense guarantees.
- **Proximity of Information**: Keep implementation details near the code or project it describes. Do not stuff specific technical workflows into global workspace documents.
- **Root Versus Project Scope**: Keep workspace-level documents strictly generic. Project-specific architecture, workflow, and implementation guidance belongs exclusively in the relevant project directory.
- **Targeted Validation**: Always run the most relevant formatter, linter, and targeted tests for the specific module you are changing. Clearly report any verification that could not be run.
- **Human Accountability**: A pull request is a long-term commitment. AI must act purely as an assistant. Do not generate invasive subsystems the human cannot explain or maintain.
- **No Autonomous Overreach**: Agents must not unilaterally commit, push, or open Pull Requests on the user's behalf without explicit direction.
- **Professional Interfaces**: Maintain a professional, accessible, and vanilla visual theme. Avoid over-the-top styling (like "cyberpunk" or heavy neon aesthetics). Prioritize clean typography and functionality.
- **Systematic Layout Design**: Never use ad-hoc layout bypasses or hardcoded Y/X coordinate offsets to fix visual text or container clipping. Position and baseline issues must be resolved systematically within the rendering engine's layout components (like FlexBox or Grid) or component-level metric calculations, and parent container dimensions must be properly sized to fit their contents.
- **Design System Conformance**: POEM and every downstream app share one cross-platform design language kept in the Git-ignored `adaptive-ui-ux-guide/` checkout at the workspace root (principles, color, tokens, adaptation, typography, per-component and per-platform specs, and the `DESIGN_LINTING.md` diagnostic catalog). UI work must conform to it, not reinvent styling ad hoc. Conformance is enforceable, not aspirational: `pkg/app/designlint` (`Lint`, `LintTheme`, `LintLayout`) implements the guide's `UI###` diagnostics, and checked-in apps should have no findings outside explicit, reasoned migration allowlists — wire it into acceptance tests. The relationship is **bidirectional**: when you discover a real, generalizable design rule or anti-pattern while building (e.g. the layout-output-as-intrinsic-size anti-pattern), dogfeed it back into the guide (and, if enforceable, add the diagnostic) in the same change — the spec improves from practice, it is not frozen. The guide is reference material like `OpenKnowledgeFormat/`; consult it before designing UI, and edit it deliberately when improving the language rather than guessing.
- **Public API Stability**: Treat exported symbols on the `pkg/render` surface (and any other package intentionally exposed to downstream apps) as contracts. Document deliberate breaking changes in the same commit that makes them, not as a follow-up. Add Go doc comments for every exported type, field, constant, and function on that surface.
- **Trust Boundaries**: Clearly separate trusted/sanitized data from raw external or user input at every boundary (protocol messages from the sidecar, automation HTTP payloads, downstream app input). Never let unsanitized input reach a sensitive sink without explicit validation at that boundary.
- **Defensive Enum Handling**: Validate enum-like or variant values (config modes, capture modes, protocol message kinds) before they drive behavior. Unrecognized values must fall back to a documented default rather than silently misbehaving or panicking.

## Git Hygiene

- Do not revert or discard unrelated user changes; work carefully with a dirty working tree.
- Keep commits thematic and small enough to review; separate behavior changes, wiring, and docs-only changes when that preserves a meaningful history.
- Use clear, conventional, subsystem-oriented commit messages (e.g. `feat(render): ...`, `fix(sidecar): ...`, `docs: ...`).
- Do not commit generated binaries, build artifacts, or caches; respect `.gitignore`.
- Before committing, inspect `git status`, review the diff, and run `git diff --check` to catch stray whitespace or conflict markers.


## Architecture & Principles

1. **Go UI Semantics, Native Presentation Boundary**: Layout, page rebuilds, focus, hit-testing, automation semantics, and frame generation live in Go under `pkg/render`. Windowing, D3D11 presentation, DPI handling, and desktop-native capture hooks live in the generic C++ Windows host. The host and application-specific Go DLL share one OS process.
2. **Stable Public Surface**: Downstream apps consume `pkg/app`, prepare native configuration through `desktop.Configure`, and register the Windows c-shared entrypoint through `pkg/windows`. The versioned six-function C ABI and adjacent `poem_app.dll` name are contracts.
3. **Repo-Owned Protocol**: Windows and Android preserve the custom binary protocol in `pkg/render/protocol` across their in-memory transports. When adding native capabilities, update both Go and C++ implementations and round-trip fixtures together.
4. **Shared Automation, Not App-Specific Hacks**: If a downstream app needs automation, inspection, screenshots, or lightweight window control, implement it in POEM's shared automation layer rather than baking one-off app helpers into product code.
5. **One Application API, Three Layers**: Since the 2026-07-16 consolidation ([okf/concepts/decisions/consolidation.md](okf/concepts/decisions/consolidation.md)), the former siblings live in this repo: `pkg/app` is the single public application API (`App[S]`/`Msg`/`Node`), `pkg/render` is the native engine layer, `pkg/web` is the server-rendered HTML component library (formerly GopherWeb). When adding or changing a public component in `pkg/render/components` or `pkg/web/components`, check the other catalog for the same concept and converge naming/shape when the need is genuinely the same; leave it alone when the difference is structural (`pkg/render`'s stateful closure-based tree vs. `pkg/web`'s per-request rendering and caller-trusted HTML fields — see the superseded-but-still-true findings in [okf/concepts/decisions/gopherweb-parity.md](okf/concepts/decisions/gopherweb-parity.md)). New app-facing capability is exposed by adding a `Node` kind in `pkg/app` plus a case in **both** drivers' `render.go` — there is no partial-coverage state; track coverage in [okf/concepts/app/component-parity.md](okf/concepts/app/component-parity.md).

## Automation Rules

1. **HTTP Is the Recommended Automation Transport**: POEM supports automation modes through `render.AutomationConfig`, but localhost HTTP is the preferred control plane for development, tests, and agent-driven inspection.
2. **Keep the Capture Modes Distinct**:
   - `self-frame`: the app's own internal render output
   - `window-frame`: the app window's client-area presentation
   - `desktop-frame`: what is literally visible on the desktop in the app's screen region
3. **Prefer `/inspect-frame` For Agent Inspection**: This endpoint performs the shared "prepare window, prefer desktop when truly foregrounded, otherwise fall back to self-frame" flow. Agents should use it instead of reconstructing that logic ad hoc.
4. **Window Control Must Stay Narrow**: `prepare-window` and related native debug controls should remain dev/test oriented and limited to safe operations like restore, clamp, and foreground requests. Avoid turning the automation layer into a broad arbitrary window-management API.
5. **Document New Endpoints Immediately**: If automation endpoints, headers, state fields, or inspection semantics change, update the relevant `okf/concepts/automation/` concept, `README.md`, and any relevant tests in the same change.
6. **Don't Leak Internals Over HTTP**: Automation endpoint handlers should return appropriate status codes and error bodies, never raw panic output, stack traces, or internal error details.

## Development & Validation Workflow

- Format Go changes with `gofmt`.
- Prefer targeted validation:
  - `go test ./pkg/render/...`
  - `go test -race ./pkg/render/...` for changes touching concurrency, locking, or the sidecar protocol
  - `go build -buildmode=c-shared -o poem_app.dll ./cmd/engine`
- Rebuild the native Windows host when changing `cpp_sidecar/` or the shared protocol:
  - `cmake -S cpp_sidecar -B cpp_sidecar/build`
  - `cmake --build cpp_sidecar/build --config Release`
- If the host binary is locked during rebuild, stop the product-named host application currently using it.
- Stop any process you started solely for manual verification (engine binary, sidecar, HTTP automation server) once you're done — don't leave orphaned instances running.

## Documentation-as-Code (OKF)

- POEM keeps a local OKF bundle at `okf/` (root `okf/index.md`, concept index at `okf/concepts/index.md`, ledger at `okf/log.md`). Treat it as part of the implementation, not optional documentation. Engine API, protocol, automation-surface, or architecture changes require corresponding OKF updates in the same thematic commit.
- `OpenKnowledgeFormat/` is Git-ignored reference material (see `.gitignore`). Never modify it for POEM work — inspect it for the spec when changing OKF structure, frontmatter conventions, or link policy, rather than guessing.
- `adaptive-ui-ux-guide/` (workspace root, Git-ignored) is the shared cross-platform design language — see **Design System Conformance** under Product Standards. Consult it before UI work; conform via `pkg/app/designlint`; improve it bidirectionally when you learn a generalizable rule. Unlike `OpenKnowledgeFormat/`, editing it *is* expected when the language genuinely advances.
- `okf/index.md` is the bundle root and carries only its required `okf_version` frontmatter; directory-level `index.md` files (e.g. `okf/concepts/index.md`) carry no frontmatter.
- `okf/log.md` is a chronological documentation ledger, not a release changelog — add an entry under today's date for every bundle change.
- Every non-reserved concept file starts with valid YAML frontmatter (non-empty `type`, plus title, description, tags, timestamp metadata) and uses bundle-absolute links (e.g. `/concepts/protocol.md`).
- Keep relationships between architecture, protocol, automation, and decisions discoverable through index pages and cross-links.
- `okf/concepts/architecture/`, `okf/concepts/automation/`, `okf/concepts/public-api.md`, and `okf/concepts/protocol.md` are the canonical detail on POEM's runtime, UI authoring, and automation surface. `ARCHITECTURE.md`, `GUIDE.md`, and `docs/AUTOMATION.md` were removed once their content was fully subsumed here — do not recreate them as duplicate prose; edit the `okf/` concept instead.
- `README.md` remains a separately-maintained human-facing overview (product intent, build/run instructions, current status) and is not subsumed into `okf/`. Keep it current per **Atomic Updates** above when the facts it states change; do not let its automation/layout-measurement summaries silently diverge from the corresponding `okf/` concepts.

## Common Gotchas

- **Thread stability still matters**: the Go orchestrator and native window loop assumptions remain sensitive to thread behavior and message-pump timing.
- **Protocol edits are cross-language edits**: do not update only the Go or only the C++ side.
- **Inspection mode matters**: a clean internal render does not prove the app is visible on the user's desktop. Use the correct capture mode for the question being asked.
- **Foreground is observable state**: use native state reporting instead of assuming the app is frontmost.
- **Downstream docs matter**: if an engine change affects GenEngine or another POEM app, update the downstream docs too.

## Additional References

- `README.md`: project overview and build/run entrypoints
- `okf/concepts/architecture/index.md`: runtime architecture, UI authoring, and component/layout concepts (replaces the removed `ARCHITECTURE.md` and `GUIDE.md`)
- `okf/concepts/automation/index.md`: HTTP automation API, capture semantics, and inspection workflow (replaces the removed `docs/AUTOMATION.md`)
- `okf/concepts/public-api.md`, `okf/concepts/protocol.md`: public contracts and the shared presenter protocol
- `okf/index.md`: local OKF knowledge bundle root
