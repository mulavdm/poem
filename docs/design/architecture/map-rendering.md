# GPU Vector Map Rendering

POEM draws interactive vector maps itself, on the GPU, on every native target — there is no MapLibre, no web map library, and no per-backend copy of the map maths. `pkg/cartography` turns raw vector tiles into a retained scene; `shared/poem/*.h` holds the projection, lighting, fog and shadow maths **once**; and each presenter (D3D11, GLES3, and the WASM/WebGL2 bridge) contributes only its own entry points and buffer plumbing. This is the same "write the maths once, transcribe the irreducible dialect per backend" discipline the [cross-platform C++ framework TDD](../../reference/protocol.md) prescribes.

## Camera interaction contract

`MapViewportNode` is controlled by `MapCamera`: every committed interaction dispatches
`OnCameraChange` with latitude, longitude, zoom, bearing, and pitch together. One finger or a
primary-button drag pans; pinch or the mouse wheel zooms around the focal point; two-finger
centroid movement or a secondary-button drag changes bearing horizontally and pitch vertically.
The viewport publishes live updates while a gesture is active so the retained scene is rebuilt
against the current camera rather than jumping only after release. Its `MinZoom`/`MaxZoom` and
`MinPitch`/`MaxPitch` bounds are enforced before dispatch.

Input routing is screen-space and gesture-captured. Containers transform the pointer into their
children's coordinate space (notably `ScrollView`'s content offset), and the component selected
at begin retains ownership until end/cancel. This prevents a visually scrolled map from losing
pinch/pan to its page container or dropping the gesture when a contact crosses its edge.
The viewport's in-progress interaction is also transient application state keyed by its stable
semantic ID, rather than mutable fields on one component instance. `BuildPages` may replace that
instance between input frames; the next instance must still be able to continue and finish the
same drag or pinch.

## The cartography pipeline (`pkg/cartography`)

A frame flows through pure, WASM-safe Go before any GPU is involved:

- **Decode** — `mvt.go` parses Mapbox Vector Tiles; `feature.go`/`projection.go` place geometry in normalized Web-Mercator world coordinates. `drape.go` and `terrain.go`/`elevation.go` sample terrain so extrusions and the ground sit at real elevation.
- **Style** — `style.go`/`default_style.go` resolve semantic layers (water, waterway, land, building, road, label, route, traffic, marker) against the caller's `MapStyle`. Colours come from the theme's `Cartography` tokens, never the UI accent — a data canvas must recede, not read as an interactive surface.
- **Scene build** — `scene.go` assembles the ordered draw batches, splitting GPU-owned geometry (ground + extrusions, projected and depth-tested) from the CPU composite (dots, lines, textured label quads).
- **Declutter** — `declutter.go` thins POI dots deterministically: at most one dot per 24-unpitched-px screen cell (first feature in stable tile order wins) with a linear ground-distance fade to a hard cull, so a pitched camera never accumulates an opaque marker band at the horizon. The fade is written into per-vertex alpha, so no protocol or presenter change is needed, and culled dots drop out of pick records naturally.
- **Labels** — `label_source.go`/`labels.go`/`text.go`/`atlas.go` shape glyphs and place labels by screen-space collision. Labels carry an explicit **hierarchy** (place, district, neighbourhood, street, POI) with distinct sizes, weights and priorities; low-priority POIs thin before structural labels. Glyph weight participates in atlas identity and deterministic rasterization. Same-text candidates within 320 logical px collapse to one, so a name spanning a tile seam is labelled once rather than four times.
- **Sun** — `sun.go` derives the solar azimuth/elevation used for shading and shadow-cascade framing.

The result is packed into a `MapSceneDelta` and shipped over the [presenter protocol](../../reference/protocol.md); the native runtime (`pkg/app/desktop/map_runtime.go`) reconciles scenes per viewport, generation-fenced, and honours the `MapCachePolicy`.

## The shared maths (`shared/poem/*.h`)

Three headers are the single source of truth, so no backend re-derives the model:

- **`map_view.h`** — `poem::mapview::Make` builds the camera transform (projection + solar shading + depth ordering) that both the CPU side and the contract tests use.
- **`map_shader.h`** — the canonical GPU shader body (`MapLocal`, `MapDepth`, `MapClip`, `MapShade`, `MapFog`, plus `MapLightCoord`/`MapCascadeRadius`/`MapShadowFactor` and `MapMarkerClip`/`MapMarkerAlpha`) with two small dialect *preludes*. Each backend supplies only its entry points — the irreducible part (HLSL semantics/cbuffers vs. GLSL attribute/varying/uniforms). This replaced third and fourth hand-transcribed copies of the projection/lighting/fog formulas, which is exactly the divergence the header was extracted to prevent.
- **`map_gpu.h`** — uniform packing (`MakeUniforms`), batch selection (`IsGpuBatch`, `HasGpuBatches`), marker expansion (`AppendMarkerVertices`, `ReadSourceVertex`), and shadow framing (`MakeShadowSetup`). Contract tests pin the uniform packing (including that the hi/lo camera split recombines to double precision), the batch filter, and marker bounds safety.

## The GPU depth path

Ground and building extrusions are **projected on the GPU and depth-tested**, not flattened to 2D on the CPU and painter-sorted. Buildings occlude correctly by construction — the earlier translucent, interpenetrating blocks are gone. Untextured triangle batches draw from hash-keyed immutable GPU buffers uploaded once per generation, so camera moves no longer re-project or re-upload the ~500k-vertex city scene.

Two precision details are load-bearing:

- **Camera centre split hi/lo** — mercator coordinates at city zoom lose metre-scale bits in single precision on the GPU, so the centre is carried as a hi/lo float pair and recombined in the shader.
- **Depth convention "bigger = closer"** (`GREATER_EQUAL`, cleared to 0). Depth is normalised over the full `[-19·cameraDistance, +cameraDistance)` range the perspective divide can produce; a naive mapping saturates into z-ties and renders as dense stipple. Coplanar walls shared by adjacent buildings are lit with `abs(dot(n, sun))` so both windings shade identically and the remaining tie is invisible. Per-face normals are recovered on the GPU from `ddx/ddy` of the local-position varying (the derivative of a linear varying is the true face plane), so no normal attribute is stored.

**GPU-only for map geometry, by design.** The CPU painter cannot carry tens of thousands of reprojected triangles per frame on mobile, so it is not a fallback for ground/extrusions: the Android host negotiates a GLES3 context (derivatives and 32-bit indices are core there) and a shader failure is *reported*, never silently degraded. The CPU composite still carries dots, lines and textured label quads on top.

## Lighting, fog, shadows, markers

Layered onto the depth path, each shared and quality-tiered:

- **Height fog** and **solar lighting** move into the pixel shader; fog constants pass from the shared header via cbuffer/uniform so no backend hard-codes them.
- **Sun shadow cascades** — buildings cast onto the street and onto each other. Each cascade is a depth-only pass from the sun's point of view into a 1024² depth map; the main pass projects each fragment into the same light space and compares. Cascade count follows the quality tier (Battery Saver 0, Balanced 1, High 2), carried on `MapSceneDelta.ShadowCascades` appended after the fog fields. Cascades are **concentric on the camera** so they stay stable as the view pans. Only extrusions cast (ground casting onto itself is acne); both receive; shadowed surfaces darken by a strength factor rather than to black, so facades stay readable. Note the two opposing depth conventions: the main pass is `GREATER_EQUAL` (cleared to 0) while the shadow pass is conventional `LESS` (cleared to 1), which is why they are separate depth states.
- **Depth-tested marker billboards** — POI markers draw in the GPU depth pass as billboards: the world position is projected on the GPU so the marker is depth-tested against buildings, then the quad corner is offset in screen space so markers keep a constant on-screen size. A POI behind a building is hidden by it instead of floating over the skyline. Markers read depth but do not write it, so they never occlude one another. **POI category** travels in the unused tail of the existing 32-byte map vertex and renders as a distinct shared-shader silhouette, preserving category meaning without colour alone or a protocol-stride change.

Labels stay on the CPU composite and are deliberately **unoccluded** — legibility beats strict correctness for text, matching MapLibre and Google Maps.

## Verified by measurement, not by eye

The shadow work is the cautionary tale: a first look suggested the scene was uniformly darker, but an A/B pixel diff against a cascades-off build showed *zero* difference — the cascade count was decoded from the protocol but never retained on the scene, so it silently defaulted to 0. After fixing the plumbing, a high midday sun still cast almost nothing (0.2% of pixels); forcing a 05:30 sun produced long directional shadows over 17.2%. The eye would have accepted the broken build; the diff would not. Map-rendering changes are gated by pixel-level visible verification, not sight.
