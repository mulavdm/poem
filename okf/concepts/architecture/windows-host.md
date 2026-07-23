---
type: Concept
title: Single-Process Windows Host
description: The versioned Go DLL ABI, secure native loader, in-memory transport, lifecycle, and packaging contract.
tags: [windows, cgo, dll, d3d11, packaging, msix]
timestamp: 2026-07-23T00:00:00Z
---
# Single-Process Windows Host

M7 packages one application-specific Go c-shared library as adjacent `poem_app.dll` beside a generic product-named C++ host. The Windows loader uses the host executable's absolute directory and restricted `LoadLibraryExW` search flags. It resolves and validates ABI v1, all required exports, bounded JSON metadata, identity, title, and preferred dimensions before starting the engine. The module is intentionally never unloaded because it contains the Go runtime.

The exported contract is `PoemWindowsABIVersion`, `PoemWindowsMetadata`, `PoemWindowsStart`, `PoemHostRead`, `PoemHostWrite`, and `PoemWindowsStop`. `pkg/hosted` provides a shared bounded lifecycle and crossed in-memory pipe pair for Windows and Android. Partial transfers are legal; the native host retains the existing four-byte little-endian frame protocol and enforces a 64 MiB message limit.

The window host owns Win32, D3D11, UI Automation, IME, audio, DPI, dialogs, automation capture, and the message pump. The Go DLL owns the application, reducer, asynchronous commands, layout, semantics, font atlas, and frame generation. They are different native languages and runtimes inside exactly one OS process, not a Go-to-C++ translation.

Closing the window sends the normal protocol close event, cancels the engine cooperatively, closes transport endpoints to release blocking reads, and waits for bounded Go cleanup. Startup failures are reported before or alongside window creation and cannot leave a presenter child or extracted file.

The host presents D3D11 through **DXGI flip-discard**, not the legacy blit/discard swap model. Under blit/discard the presenter rendered a correct backbuffer but DWM kept showing a white redirection surface — a fully composed semantic tree and map that never reached the screen. Flip-discard hands the backbuffer directly to the compositor, which fixed that class of driver/compositor blank-window failure; default and maximized visible-capture gates confirm the real UI and map are on screen.

Line segments are emitted as a quad **rotated onto the segment**, with four explicit corners, matching the Go reference rasterizer and the GLES presenter. The obvious-looking alternative of offsetting the endpoints along the segment normal and handing them to the axis-aligned `AppendQuad` collapses to zero area whenever the offset endpoints share an X or a Y — that is, for every horizontal and vertical line, which is most of them. Under that bug the datepicker's calendar icon drew nothing at all and chart gridlines were invisible, while diagonals rendered as two overlapping axis-aligned boxes shaded by an SDF rect that did not coincide with the emitted geometry. Because `shadeShape`'s rounded-rect SDF is axis-aligned and cannot describe a rotated segment, the shading rect is padded past the quad's bounds so every rasterized fragment shades solid and the geometry alone defines the line; the cost is hard rather than SDF-antialiased edges.

`RendererD3D11::Present` is deliberately **separate** from `Render`, so the
`present` timing channel covers geometry compilation and draw submission only.
`Present` hands the backbuffer to DWM and its cost depends on compositor state,
which is not framework CPU work. The GLES presenter already excluded its
`eglSwapBuffers`, so including this one meant the two platforms' `present`
channels were never measuring the same thing — defeating the reason the
recorder is shared at all.

The engine serializes each frame under `stateMutex` but **releases the lock
before writing it to the presenter pipe**. The pipe is a synchronous `io.Pipe`,
so a write blocks until the host drains it, and the host drains it on the same
reader thread that services a `NativeDebugRequest` — foreground activation,
restore, and other window operations for `/prepare-window` run there. While such
an operation is in flight that thread is not reading, so a frame write issued
under `stateMutex` would block with the lock held and stall every state reader:
`/components` and `/state` would stop answering until the window operation
returned. The repaint loop therefore builds the frame into a buffer under the
lock and flushes it afterward; the flush can still block, but no longer behind
`stateMutex`. This is the Go-side sibling of the C++ reader's "post, never send"
rule (`main.cpp`), which avoids the same class of transport deadlock from the
other direction.

The host records its own frame timings through `shared/poem/perf.{h,cpp}`, reported over the native debug channel and read with `GET /perf/native`. The timed channels are `present` (geometry compilation plus draw submission on the UI thread, excluding GPU execution), `decode_frame`, `decode_map_scene`, and `apply_map_scene`. Before this the host was entirely untimed — every performance figure POEM produced was measured on the Go side of the boundary, so the native cost of a frame was unknown. The recorder is shared with the Android presenter deliberately: identical channel names and one percentile definition are what make a cross-platform comparison mean anything. See [Performance & Latency Profiling](/concepts/automation/perf-profiling.md).

`windows_host/build.ps1` produces a portable EXE plus DLL folder/ZIP and a packaged Win32 full-trust MSIX. Signing is optional and externally configured. Development certificate creation and certificate trust installation are separate commands so package construction never silently changes trust stores.

## See also

- [Protocol](/concepts/protocol.md)
- [Public API](/concepts/public-api.md)
- [Android Presenter](/concepts/architecture/android-presenter.md)

# Citations

- [README](file:///d:/Programming/GUIProject/POEM/README.md)
- [M7 migration guide](file:///d:/Programming/GUIProject/POEM/MIGRATION_M7.md)
