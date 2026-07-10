# OKF Bundle Update Log

## 2026-07-10
* **Migration**: Subsumed `ARCHITECTURE.md` and `GUIDE.md` into 17 concepts under `okf/concepts/architecture/`, and `docs/AUTOMATION.md` into 6 concepts under `okf/concepts/automation/`. Merged the sections both `ARCHITECTURE.md` and `GUIDE.md` duplicated (cursor/layout-sync, scroll viewports, DPI translation, reactive-loop/IoC) into single concepts covering design rationale and usage together. `ARCHITECTURE.md`, `GUIDE.md`, and `docs/AUTOMATION.md` are now short redirect stubs pointing into the bundle. `README.md` and `AGENTS.md` were left as separately-maintained human/agent entrypoints, not subsumed.
* **Creation**: Initialized the POEM `okf/` bundle (root index, concept index, architecture, public API, sidecar protocol, and automation concepts), sourced from `AGENTS.md`, `README.md`, `ARCHITECTURE.md`, and `docs/AUTOMATION.md`.
