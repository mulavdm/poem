# M0 — Duplication Inventory

Input for [Plan.txt](Plan.txt) step 2 ("Create the shared native project"). The plan
asserts that host loops, geometry compilation, caches, input, and services are
duplicated between the Windows and Android presenters, and prices a shared
`poem_native_core` off that assertion. This document measures it.

**Method.** Function-level pairing between `cpp_sidecar/src/renderer_d3d11.cpp`
and `android_engine/src/renderer_gles.cpp`, and between `cpp_sidecar/src/native_app.cpp`
and the transport section of `android_engine/src/main.cpp`. Each pair was read and
classified, not diffed mechanically — normalized line-identity ran 15–66% across
pairs and is too noisy to size anything.

## 1. Corpus

Excludes `build*/` artifacts and tests.

| Component | Lines | Notes |
|---|---:|---|
| `cpp_sidecar/src` | 4,594 | `renderer_d3d11.cpp` 1,858 · `accessibility.cpp` 1,208 · `main.cpp` 898 · `native_app.cpp` 195 · `audio.cpp` 125 · headers 310 |
| `android_engine/src` | 2,656 | `renderer_gles.cpp` 1,191 · `main.cpp` 841 · `ime_jni.cpp` 259 · `audio_aaudio.cpp` 142 · headers 223 |
| `shared/poem` | 1,432 | already shared: `protocol` 768, `map_gpu.h` 279, `map_shader.h` 202, `map_view.h` 183 |
| C++ tests | 469 | 3 CTest targets |
| **Total host C++** | **9,151** | of which 1,432 (16%) already shared |

`rust_engine/` (6,728 lines) is **excluded**: it is the older engine that
predates the C++ port and is superseded, not a third duplicate. It is correctly
out of scope for step 2 — but it is still referenced from `README.md` and
`okf/concepts/web/index.md`, so those pointers want revisiting when step 7
removes stale documentation.

## 2. Classification

Android is the smaller host, so its line count sets the extraction ceiling.

### Tier A — pure logic, duplicated near-verbatim (~270 Android lines)

Extractable with no design decisions. Mechanical.

| Function | GLES | D3D11 | Notes |
|---|---:|---:|---|
| `DecodeUtf8` | 52 | 45 | identical UTF-8 decoder, copy-pasted |
| `MapHashKey` | 9 | 4 | 32-byte hash → string key |
| `HoverSamples` / `ClickSamples` / `SuccessSamples` | ~50 | ~50 | identical PCM synthesis; `TASK.md` records the mirroring as deliberate to preserve the sidecar provenance manifest |
| framing: `ReadExact` / `WriteFramed` / `FlushEvents` | ~90 | ~60 | same 4-byte length prefix, same batching |
| envelope dispatch switch | ~70 | ~60 | same message set, same decode calls |

### Tier B — same algorithm, API-parameterized (~425 Android lines)

This is the plan's `CompiledFrame` boundary and it is real — but every pair below
has drifted (§3), so extraction means picking a winner for each divergence, not
lifting code.

| Function | GLES | D3D11 |
|---|---:|---:|
| `BuildGeometry` | 160 | 156 |
| `AppendMapGeometry` | 103 | 132 |
| `ApplyMapScene` (retention, generation, resource lifetime) | 54 | 46 |
| `EnsureMapGeometryBuffer` | 24 | 29 |
| atlas → `GlyphInfo` bookkeeping | 23 | 25 |
| camera/uniform prep inside the three GPU-map functions | ~60 | ~60 |

Note the projection math is *already* shared in `shared/poem/map_gpu.h`. What
remains inside `DrawGpuMapBatches`, `RenderShadowCascades`, and `DrawMapMarkers`
is predominantly buffer binding, which is Tier C.

### Tier C — irreducibly platform-bound (~1,960 Android lines)

Shaders (GLSL ~230 vs HLSL ~410 — semantically parallel, but both must exist and
neither can be generated from the other without a shader IR the plan explicitly
forbids in its dependency assumption), `Init`/`CreateShaders`/
`CreateDeviceAndSwapchain`, EGL vs DXGI surface handling, FBO vs DSV, texture
upload calls, `Resize`, all of `android_native_app_glue` plumbing
(`InitDisplay`, `TermDisplay`, `HandleCmd`, `HandleInput`, `CheckSurfaceSize`),
`ime_jni.cpp`, and the Win32 `WndProc` / dialog / capture / UIA surface.

### Ceiling

**~695 Android lines (Tier A + B), against ~800 on the Windows side.** Roughly
**8% of the host C++ corpus**, or 26% of `android_engine`.

Step 2 asks for `Runtime`, `ApplicationTransport`, `RendererBackend`,
`PlatformServices`, `CapabilitySet`, `LifecycleEvent`, `InputEvent`,
`Result`/`Error`, a null backend, and contract tests. That is plausibly
1,500–2,500 lines of *new* code to delete ~700 lines of duplicate.

**Deduplication alone does not justify the program.** §3 does.

## 3. Silent divergence — the actual finding

`android_engine/src/renderer_gles.h:5` states the contract: the two presenters
interpret commands "with the same semantics … so a frame renders the same on
either presenter." That is already false in nine places, and nothing in the tree
detects it.

1. **`DrawLine` was broken on both presenters, in different ways.** *(Fixed —
   see §5.)* The reference semantics are the Go rasterizer's true segment
   (`automation.go:1704` → `drawLine`). Neither presenter matched it:
   - **D3D11** offset the endpoints along the segment normal but passed them to
     `AppendQuad`, which expands an *axis-aligned* extent into
     `(x1,y1)-(x2,y2)`. For a horizontal line the two offset endpoints share a
     Y, for a vertical line an X — so the quad collapsed to zero area and
     **every axis-aligned line rasterized nothing**. Diagonals produced two
     overlapping axis-aligned boxes shaded by an SDF rect (`len × thickness`,
     anchored at the bounding-box corner) that did not coincide with the
     emitted geometry.
   - **GLES** filled the axis-aligned bounding box, so axis-aligned lines were
     correct and every diagonal became a solid block.

   Affected callers: chart gridlines and fill columns
   (`pkg/render/components/chart.go:68,152`), chart splines (`chart.go:168`),
   spinner spokes (`feedback.go:44`), checkmarks (`selection.go:55`),
   datepicker icon strokes (`datepicker.go:121`), composition underline
   (`components.go:710`), particle links (`types/particles.go:101`).
2. **Raycaster/billboard modes absent on Android.** Radius sentinels `-999` and
   `-991…-998` select ~120 lines of HLSL shading plus two append functions on
   D3D; GLES comments them out as "plain rects."
3. **Vertex formats differ.** D3D carries 20 floats including
   `shadowOffsetX/Y/Softness/padding`; GLES carries 16 and smuggles shadow blur
   through the `glow` slot. Shadows are approximations of each other.
4. **Scale model differs.** D3D derives independent `scaleX`/`scaleY` from
   backbuffer ÷ frame; GLES uses one `scale_` from display density. Non-uniform
   scaling exists on one host only.
5. **`SetClip` conventions differ.** D3D stores right/bottom as
   `(x1 + w) * scaleX`; GLES stores width/height and applies the system-bar inset
   separately.
6. **Image caching is inverted.** GLES caches uploads by content hash
   (`UploadImageCached`); D3D re-uploads through `EnsureImageTexture` and copies
   image bytes into every `DrawRange`. The desktop path re-uploads photo-sized
   RGBA per frame.
7. **Draw-range coalescing exists only on GLES.** It merges adjacent compatible
   ranges; D3D pushes one per command. Identical frames produce different
   draw-call counts — directly relevant to the plan's 120 Hz budgets.
8. **Transport limits and failure modes differ.** Windows caps messages at 64 MiB
   and throws (`native_app.cpp:13`); Android caps at 256 MiB and logs-then-breaks
   (`main.cpp:180`). Worse, Android's `ReadExact` returned false only on `n < 0`
   — a zero-length read **spun the transport thread** rather than terminating
   it, where Windows' `ReadExact` fails on `count <= 0`. *(Spin fixed — see §5;
   the differing size caps remain.)*
9. **`SemanticTree` is dropped on Android** — known and tracked in `TASK.md`.

## 4. What this changes in the plan

- **Justify the shared core on divergence control, not line count.** The plan's
  stated rationale ("reduces duplication") is worth ~700 lines and is a weak
  trade. The defensible rationale is that the presenters are contractually
  identical, are not, and drift undetected. Rewrite step 2's motivation
  accordingly, or the program will be judged against a number it cannot hit.
- **Build the conformance corpus before the core.** Captured protocol streams
  plus a golden `CompiledFrame` is what makes divergence detectable — and it is
  the *same* harness step 4's parity gate requires. Currently sequenced after the
  core; it should come first, and it independently pays for itself by catching
  items 1–7 today.
- **Tier B extraction is nine decisions, not a refactor.** Each divergence needs
  a winner chosen and verified on both hosts. Several change visible output
  (1, 2, 3, 4). Budget them as behavior changes with screenshot review, not as
  mechanical moves.
- **`rust_engine/` needs no decision** beyond documentation cleanup (§1).

## 5. Fixed in this pass

Items 1 and 8 were isolated enough to fix without waiting on the migration.

- **`DrawLine` (both presenters).** `AppendLineQuad` now pushes four explicit
  rotated corners instead of routing through the axis-aligned `AppendQuad`, so
  the segment geometry matches the Go reference on both backends
  (`renderer_d3d11.cpp:1463`, `renderer_gles.cpp:684`). Because both fragment
  shaders shade `drawType 0` with an *axis-aligned* rounded-box SDF that cannot
  describe a rotated segment, the shading rect is padded 2px past the quad's
  bounds; every fragment the quad rasterizes then lands well inside it and
  shades solid, leaving the geometry to define the line. Consequence: line edges
  are hard rather than SDF-antialiased. Giving lines a proper capsule SDF would
  fix that, but it means a shader and vertex-format change on both backends —
  correctly a Tier B decision, not part of this fix.
- **Android transport spin.** `ReadExact` now fails on `n <= 0`
  (`android_engine/src/main.cpp:167`).

**Verification — Windows: confirmed by frame capture.** The packaged gallery was
run and captured through the HTTP automation surface (`/self-frame`, the D3D11
backbuffer) at baseline and with the fix, same build recipe, same view:

| Subject | Baseline | Fixed |
|---|---|---|
| Spinner spokes (`feedback.go:44`, mixed angles) | scattered horizontal dashes; no vertical spokes; not recognizable as a spinner | clean 10-spoke radial burst with the intended opacity falloff |
| Datepicker calendar icon (`datepicker.go:121`, four axis-aligned strokes) | **nothing rendered at all** | all four strokes render |
| Checkbox checkmark (`selection.go:55`, two diagonals) | — | renders |

This confirms the analysis above: axis-aligned `DrawLine` output was entirely
invisible on the D3D11 presenter, and diagonals were misplaced. Release build is
clean and CTest passes 3/3.

**Verification — Android: not done.** `renderer_gles.cpp` and `main.cpp`
syntax-check against NDK 30.0.15729638 for `aarch64-linux-android30`, which is
compile coverage only. **The GLES line fix and the transport-spin fix have not
been seen running** — both need an emulator or device pass before they are
trusted.

Note what this exercise required: building and packaging the app, driving it
over HTTP, and eyeballing zoomed crops. Nothing in the test suite would have
caught either defect, and nothing will catch the next one. That is the argument
for the conformance corpus in §4, and the reason it should land before the
shared core rather than after it.
