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

A **zero-length read terminates the transport**, exactly as a negative one does. `PoemHostRead`
returning 0 means the engine closed its end; treating that as "nothing yet, try again" spins
the transport thread at 100% instead of shutting it down, because the condition never clears.
The Windows host's `NativeApp::ReadExact` rejects `count <= 0` for the same reason, and the
two must agree — the framing is shared, so a disagreement about what end-of-stream looks like
is a disagreement about the protocol.

`pkg/mobile` also redirects fds 1/2 into logcat (`poem-go` tag): stderr is a black hole
inside an APK, and without the redirect Go panics and the engine's own progress prints are
invisible.

## Renderer (`android_engine/src/renderer_gles.cpp`)

The GLES2 renderer mirrors `cpp_sidecar`'s `RendererD3D11` command interpretation — SDF
rounded rects, glyph quads from the Go-supplied atlas, scissor clipping, offset translation,
shadow/glow companion quads — and compiles `shared/poem/protocol.cpp` directly rather
than forking the decoder. GLSL ES note: `half` is a reserved word.

Line segments are emitted as a quad rotated onto the segment, matching the Go
reference rasterizer and the D3D11 presenter. Filling the axis-aligned bounding
box instead turned every diagonal — chart splines, spinner spokes, checkmarks —
into a solid block.

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
drags retain the mouse path for sliders and existing controls. Gesture routing carries each
container's pointer-coordinate transform: a viewport inside scrolled content is hit in visible
screen space and receives the corresponding content-space point, with its stable ID captured
from gesture begin through end.

Hardware mouse input bypasses the touch-slop classifier. Android `ACTION_SCROLL` becomes a
screen-positioned wheel event, while primary, secondary, and tertiary buttons retain their
identity. This is what makes wheel zoom and secondary-drag map bearing/tilt behave the same in
the Android presenter as in the Windows host.

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

On Windows the script runs under Git Bash, not WSL Bash. It resolves the native Windows
Go installation to a `/c/...` executable path before invoking `go build`, keeping the
Windows Go and Android SDK/NDK toolchains in one environment.

## Inspection and native timing

The presenter implements the native debug channel (protocol 201/202), so
`/native-state` and `/perf/native` answer on Android as they do on Windows. It
records the shared `poem::perf` channels — `present`, `decode_frame`,
`decode_map_scene`, `apply_map_scene` — from `shared/poem/perf.{h,cpp}`, the
same translation unit the D3D11 host compiles, so a p95 means the same thing on
both. `/native-state` reports the surface as the window (there is no separate
window rect) and density where Windows reports DPI.

Frame capture **is** implemented, by queueing rather than refusing. The request
arrives on the transport thread, but `glReadPixels` needs the GL context, which
is current only on the render thread — so the transport thread queues the
request and waits on a condition variable, and `DrawFrame` serves it. The
readback happens **before** `eglSwapBuffers`, because the default
`EGL_BUFFER_DESTROYED` swap behaviour leaves the back buffer undefined
afterwards, and GL's bottom-up rows are flipped to the top-down RGBA the D3D11
host produces, so callers cannot tell the platforms apart.

The wait is bounded (3s): a paused or surfaceless presenter draws no frames, and
the transport thread must not block forever. Timeouts and GL errors report
themselves rather than returning a blank image that would read as a rendering
failure. `captureFrame` and `capturePresentedFrame` are the same readback here —
the surface *is* the window. `captureDesktopFrame` remains unavailable: it needs
MediaProjection and a user consent dialog, which an automated scenario cannot
answer.

The earlier design refused capture and directed callers to `adb shell
screencap`. That pushed the work out of band into every caller, could not be
driven by a scenario, and captured the *screen* rather than this presenter's own
output.

The automation surface itself is Go-side and platform-neutral, so enabling it is
a configuration matter rather than presenter work — see
[Automation Overview](../automation/TDD.md). Because NativeActivity
has no command line, the opt-in gate is the `debug.poem.inspection` system
property, read by `pkg/mobile` at package init and translated into the shared
`POEM_INSPECTION` environment contract. **That read must happen in Go**: the Go
runtime snapshots the environment at init, so a `setenv()` from this host
afterwards is never visible to `os.Getenv`.

## Known gaps

Audio: `PlaySound` plays via AAudio (short-lived stream per sound, engine-side debounce, opt-in via `AppConfig.Effects.Audio`). Tracked in `TASK.md`: accessibility bridge (`SemanticTree` dropped; the Windows UIA host is the
reference), arm64 on-device verification. Lifecycle: rotation/pause/resume/process-reuse are handled (see log 2026-07-17).
