---
type: Concept
title: Native Presenter Protocol
description: The repo-owned binary wire protocol shared by the Go engine and native in-process presenters.
tags: [protocol, transport, windows, android]
timestamp: 2026-07-10T00:00:00Z
---
# Native Presenter Protocol

## Adaptive capability updates

Protocol v3 appends event type 15 for a strict capability snapshot (pointer precision, hover, keyboard/touch/trackpad/stylus, density, text scale, reduced motion, and high contrast). Existing event numbers are unchanged. Invalid enum, unknown-field, and out-of-range text-scale payloads are ignored at the presenter trust boundary.

The Go engine and native presenters use the custom binary protocol in `pkg/render/protocol`.
Windows and Android both carry it through process-local exported read/write calls backed by
crossed in-memory pipes. Protocol v3 preserves every existing event number and appends pan and
pinch gestures carrying phase (begin/update/end/cancel), centroid, two-axis incremental delta,
and incremental scale. The engine can therefore render gestures locally without adding
platform-specific concepts to application nodes.

Protocol changes are cross-language changes: update both the Go and C++ implementations together and add or update round-trip tests in the same change.

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Single-Process Windows Host](/concepts/architecture/windows-host.md)
- [Automation Overview](/concepts/automation/overview.md)

# Citations
- [AGENTS.md](../../AGENTS.md) — Repo-Owned Protocol
