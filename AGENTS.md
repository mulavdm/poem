# Agent Instructions: POEM

This document contains project-specific architectural rules, debugging knowledge, and operational context for POEM. Agents working on this project should use it together with the workspace-level instructions and the human-facing `README.md`.

## How To Use This File

Treat this file as the repo-level source of truth for agent behavior and guardrails inside `POEM`. Use the root `README.md` for the high-level project overview, `GUIDE.md` for downstream UI authoring, `ARCHITECTURE.md` for deeper runtime notes, and `docs/AUTOMATION.md` for the automation transport and inspection model.

## Architecture & Principles

1. **Go UI Semantics, Native Presentation Boundary**: Layout, page rebuilds, focus, hit-testing, automation semantics, and frame generation live in Go under `pkg/render`. Windowing, D3D11 presentation, DPI handling, and desktop-native capture hooks live behind the Windows C++ sidecar in `cpp_sidecar/`.
2. **Stable Public Surface**: Downstream apps should continue to consume `go_native_gpu_gui/pkg/render` and `render.Run(render.AppConfig{...})`. Do not leak sidecar-specific details into application code unless they are intentionally exposed as reusable engine APIs.
3. **Repo-Owned Protocol**: Cross-process communication between Go and the sidecar uses the custom binary protocol in `pkg/render/protocol`. When adding new native capabilities, update both the Go and C++ protocol implementations together and add or update round-trip tests.
4. **Shared Automation, Not App-Specific Hacks**: If a downstream app needs automation, inspection, screenshots, or lightweight window control, implement it in POEM's shared automation layer rather than baking one-off app helpers into product code.

## Automation Rules

1. **HTTP Is the Recommended Automation Transport**: POEM supports automation modes through `render.AutomationConfig`, but localhost HTTP is the preferred control plane for development, tests, and agent-driven inspection.
2. **Keep the Capture Modes Distinct**:
   - `self-frame`: the app's own internal render output
   - `window-frame`: the app window's client-area presentation
   - `desktop-frame`: what is literally visible on the desktop in the app's screen region
3. **Prefer `/inspect-frame` For Agent Inspection**: This endpoint performs the shared "prepare window, prefer desktop when truly foregrounded, otherwise fall back to self-frame" flow. Agents should use it instead of reconstructing that logic ad hoc.
4. **Window Control Must Stay Narrow**: `prepare-window` and related native debug controls should remain dev/test oriented and limited to safe operations like restore, clamp, and foreground requests. Avoid turning the automation layer into a broad arbitrary window-management API.
5. **Document New Endpoints Immediately**: If automation endpoints, headers, state fields, or inspection semantics change, update `docs/AUTOMATION.md`, `README.md`, and any relevant tests in the same change.

## Development & Validation Workflow

- Format Go changes with `gofmt`.
- Prefer targeted validation:
  - `go test ./pkg/render/...`
  - `go build ./cmd/engine`
- Rebuild the native sidecar when changing `cpp_sidecar/` or the shared protocol:
  - `cmake -S cpp_sidecar -B cpp_sidecar/build`
  - `cmake --build cpp_sidecar/build --config Release`
- If the sidecar binary is locked during rebuild, stop `poem_cpp_sidecar.exe` and any downstream POEM app currently using it before rebuilding.

## Common Gotchas

- **Thread stability still matters**: the Go orchestrator and native window loop assumptions remain sensitive to thread behavior and message-pump timing.
- **Protocol edits are cross-language edits**: do not update only the Go or only the C++ side.
- **Inspection mode matters**: a clean internal render does not prove the app is visible on the user's desktop. Use the correct capture mode for the question being asked.
- **Foreground is observable state**: use native state reporting instead of assuming the app is frontmost.
- **Downstream docs matter**: if an engine change affects GenEngine or another POEM app, update the downstream docs too.

## Additional References

- `README.md`: project overview and build/run entrypoints
- `GUIDE.md`: downstream UI authoring quickstart
- `ARCHITECTURE.md`: deeper runtime and historical architecture notes
- `docs/AUTOMATION.md`: HTTP automation API, capture semantics, and inspection workflow
