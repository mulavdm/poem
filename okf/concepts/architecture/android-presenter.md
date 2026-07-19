---
type: concept
title: Android Presenter
description: How the C++/GLES2 android_engine presenter hosts the Go engine in one APK — the in-process transport, density model, and the startup contract.
tags: [architecture, android, presenter, mobile, gles]
timestamp: 2026-07-16T00:00:00Z
---
# Android Presenter

Android is POEM's GLES presenter. Like the M7 Windows host, it co-locates native presentation
and a Go c-shared engine in one process. A **zero-Java `NativeActivity` APK** hosts a C++/EGL/GLES2 presenter
(`android_engine/src`), and the Go engine runs inside the same process as a `c-shared`
library built with NDK clang.

## Transport (`pkg/mobile`)

`pkg/mobile` (android-only build tag) bridges `render.RunHosted` to the C++ host over an
in-memory pipe pair exposed through two exported functions: `PoemHostRead` (engine→presenter
frames) and `PoemHostWrite` (presenter→engine events). The byte protocol and 4-byte length
framing are identical to Windows' in-memory host transport. Call batching
matters: a cgo crossing is expensive, so the presenter reads whole framed messages on a
dedicated transport thread and writes one `EventBatch` per frame, never chatting per command.

`pkg/mobile` also redirects fds 1/2 into logcat (`poem-go` tag): stderr is a black hole
inside an APK, and without the redirect Go panics and the engine's own progress prints are
invisible.

## Renderer (`android_engine/src/renderer_gles.cpp`)

The GLES2 renderer mirrors `cpp_sidecar`'s `RendererD3D11` command interpretation — SDF
rounded rects, glyph quads from the Go-supplied atlas, scissor clipping, offset translation,
shadow/glow companion quads — and compiles `cpp_sidecar/src/protocol.cpp` directly rather
than forking the decoder. GLSL ES note: `half` is a reserved word.

## Density model

The engine lays out in **logical pixels** (surface pixels ÷ display density), the renderer
scales all geometry ×density, and touch coordinates divide back — the same division of labor
as the Windows presenters' DPI `scaleX`/`scaleY`. Glyphs are currently rasterized at logical
size and upscaled, so text is slightly soft at high density; re-rasterizing the atlas at
physical size is tracked in `TASK.md`.

## Touch gestures

Single-finger vertical movement emits phased pan gestures, including inertial updates before the
final commit. Two active pointers emit a true centroid/delta/incremental-scale pinch stream. The
engine routes these to the deepest compatible component, so an `ImageViewport` consumes local
map gestures and an outer `ScrollView` remains the fallback elsewhere. Horizontal one-pointer
drags retain the mouse path for sliders and existing controls.

## Startup contract

The app's Go main package (built `c-shared`) exports `PoemAndroidStart(width, height)`; the
C++ host calls it once the EGL surface exists, then starts the transport thread. **The host
must then send an initial `WindowSize` event**: the engine paints on demand and arms
`NeedsRepaint` from that event, exactly like the Windows host does — without it no frame
is ever produced (a black screen, found the hard way).

## Building

`android_engine/build_apk.sh` produces a signed APK with no Gradle: Go `c-shared` via NDK
clang, the C++ presenter, then aapt2 + zipalign + apksigner. The manifest template uses
`NativeActivity` with `hasCode="false"` and a fullscreen theme (a status-bar overlay
otherwise swallows taps near the top edge).

## Known gaps

Audio: `PlaySound` plays via AAudio (short-lived stream per sound, engine-side debounce, opt-in via `AppConfig.Effects.Audio`). Tracked in `TASK.md`: accessibility bridge (`SemanticTree` dropped; the Windows UIA host is the
reference), arm64 on-device verification. Lifecycle: rotation/pause/resume/process-reuse are handled (see log 2026-07-17).
