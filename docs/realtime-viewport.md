# RealtimeViewport

`components.RealtimeViewport` reserves a clipped, focusable, accessible native
GPU region inside POEM's normal layout and display list. It is a generic
component: its identifier and optional command bytes have no scene or editor
meaning to POEM.

The Windows host continues to accept native viewport ABI v2 plugins. ABI v3
adds `submitCommand` and `pollEvent`, both bounded to 1 MiB. Structures carry
their byte size and version; invalid pointers, sizes, versions, and buffer
lengths are rejected before a plugin callback is used.

When the display list contains a realtime viewport command, the host clips the
borrowed `FrameInput.viewport` to the window and submits the component's opaque
command before rendering. Keyboard focus remains a POEM component concern.
Hosts without native realtime support draw the component's neutral fallback.

POEM owns the HWND, D3D12 device, queue, command list, final presentation,
layout, focus, semantics, and shutdown order. The plugin records only inside
the borrowed viewport and must not retain frame-scoped objects.
