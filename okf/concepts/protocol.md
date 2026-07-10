---
type: Concept
title: Sidecar Protocol
description: The repo-owned binary wire protocol between the Go orchestrator and the C++ sidecar.
tags: [protocol, ipc, sidecar]
timestamp: 2026-07-10T00:00:00Z
---
# Sidecar Protocol

Cross-process communication between the Go orchestrator and the Windows C++ sidecar uses a custom binary protocol defined in `pkg/render/protocol`, carried over two local Windows named pipes. Protocol v2 carries semantic accessibility snapshots without embedding Windows/UIA-specific concepts directly in Go components.

Protocol changes are cross-language changes: update both the Go and C++ implementations together and add or update round-trip tests in the same change.

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Automation Overview](/concepts/automation/overview.md)

# Citations
- [AGENTS.md](../../AGENTS.md) — Repo-Owned Protocol
