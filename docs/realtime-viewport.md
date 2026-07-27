# RealtimeViewport

`components.RealtimeViewport` reserves a clipped, focusable, accessible native
GPU region inside POEM's normal layout and display list. It is a generic
component: its identifier and optional command bytes have no scene or editor
meaning to POEM.

POEM wire protocol v5 adds a targeted `RealtimeViewport` event carrying up to
1 MiB of opaque bytes. The native host polls events after viewport rendering
and routes them to the matching component on the Go side.

The Windows host continues to accept native viewport ABI v2 plugins. ABI v3
adds `submitCommand` and `pollEvent`, both bounded to 1 MiB. ABI v4 adds a
`depthStencilFormat` field to `FrameInput`: the sidecar renderer creates and
binds its own depth target before calling the plugin's `render()`, matching
the depth format it declares, so hosts (like HamsterEditor) that render real
mesh geometry alongside greybox boxes get correct depth ordering. Structures
carry their byte size and version; invalid pointers, sizes, versions, and
buffer lengths are rejected before a plugin callback is used.

When the display list contains a realtime viewport command, the host clips the
borrowed `FrameInput.viewport` to the window and submits the component's opaque
command before rendering. Keyboard focus remains a POEM component concern.
Hosts without native realtime support draw the component's neutral fallback.

POEM owns the HWND, D3D12 device, queue, command list, final presentation,
layout, focus, semantics, and shutdown order. The plugin records only inside
the borrowed viewport and must not retain frame-scoped objects.

The D3D12 host renders the plugin first and then composites POEM rectangles,
lines, atlas text, focus surfaces, and overlays into the same backbuffer.
