# Migrating to M8 map-first workspaces

M8 extends the M6 semantic API without reintroducing visual styling in application trees.

- Use `App.Start` for one-time status/bootstrap commands. It runs once per native engine and once for a newly created web session; restored sessions do not rerun it.
- Put the persistent canvas in `WorkspaceNode.Content` and planning/settings surfaces in `WorkspaceNode.Tools`. Supply a semantic title, optional subtitle/status, and header command IDs.
- Group related controls with `SectionNode`. Use command `Importance`, `Placement`, and semantic icons instead of selecting button colors or geometry.
- Prefer `CollectionNode.Items` for interactive result and direction records. Stable item and action IDs preserve accessibility and selection across adaptation.
- Decode viewport changes with `Msg.ViewportCommit()`. New payloads include logical width/height; transform-only M4/M5 payloads remain accepted.
- Apply product semantic colors with immutable `design.System.WithOverrides`. POEM recomputes readable on-accent and focus colors and keeps high-contrast resolution system-owned.

Web deployments must allow the POEM same-origin stylesheet and script endpoints. The built-in driver emits a strict self-only CSP and requires neither `unsafe-inline` directive.
