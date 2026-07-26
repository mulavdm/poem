---
type: Decision
title: External Native UI Hosts
description: How engine-owned applications embed POEM without surrendering platform or presentation ownership.
tags: [native, external-host, game-engine, ownership, c-abi]
timestamp: 2026-07-26T00:00:00Z
---
# External Native UI Hosts

POEM's application-specific Go DLL is already windowless. It owns application
state, reducers, layout, UI commands, semantics, and asynchronous work, while
the caller owns native platform resources. `shared/poem/external_ui_host.h`
names its stable six-function ABI and existing four-byte framed protocol.

A native engine may therefore load the module directly, validate ABI and
metadata, start it with logical dimensions, exchange bounded protocol messages,
and stop it cooperatively. It does not need to launch POEM's C++ Windows host.
The external owner creates the window, graphics device, swap chain, input loop,
frame graph, capture, and presentation.

POEM remains responsible for interface meaning: controls, layout, focus,
dialogue, text input, accessibility semantics, platform-service requests, and
automation-visible state. The external host converts native input into POEM
events and renders POEM draw output in its own UI pass.

This is not a dependency from POEM to a particular engine. POEM exports one
general external-host contract; HamsterEngine is one consumer. Game-driven UI
requirements may improve POEM's general components and semantics, but
game-specific concepts do not enter POEM.

The POEM-owned D3D12 real-time viewport remains an embedding sample for ordinary
applications. It is not the primary HamsterGameRPG launch architecture.

