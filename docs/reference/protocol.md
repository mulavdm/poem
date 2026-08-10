# Native Presenter Protocol

## Retained vector scenes (protocol v4)

Protocol v4 appends `MapSceneDelta` and `MapCamera` messages, an explicit `DrawMapScene` frame command, and map camera/feature/failure events without changing earlier numeric values. Scene deltas carry a viewport generation, hash-keyed vertex/index/glyph/icon resources, upload/release operations, ordered draw batches, camera and lighting uniforms. The draw command associates an accepted scene with its clipped component rectangle and carries fallback pixels for presenters without that scene. Decoders bound resource counts, draw counts, and aggregate bytes and reject trailing data. Presenters must discard generations older than the newest accepted generation for that viewport.

The Go codec and C++ enum boundary are updated. Both decoders validate buffer strides, texture dimensions/byte counts, opacity, aggregate bounds, and trailing data. Native scene submission omits bytes for unchanged hash/descriptor pairs, emits deterministic releases for resources no longer referenced, and rejects late generations before transport. D3D11 and GLES3 apply generation-fenced retained resources, project accepted scene geometry into the requested viewport, cache that geometry in a dedicated immutable/static GPU buffer, and interleave texture-aware ranges at `DrawMapScene`; hash-keyed label atlases upload once and release with the scene resource. Unchanged frames do not re-project or upload it. Hash-resource-level vertex/index GPU buffers, the WebGL2 bridge, and the Go/WASM worker remain required.

The lighting model grew inside v4 without a version bump, by **appending** uniform fields so existing numeric positions are preserved: `MapSceneDelta` carries height-fog density and colour, and then `ShadowCascades` after the fog fields (count 0/1/2 by quality tier). POI category likewise travels in the previously-unused tail of the existing 32-byte map vertex, so distinct marker silhouettes need no stride change. Decoders that predate a field see the earlier layout unchanged; newer decoders read the appended tail. How the presenters consume all of this — the shared projection/lighting/shadow maths, the GPU depth path, marker billboards, and the cartography pipeline that produces the delta — is [GPU Vector Map Rendering](../design/architecture/map-rendering.md).

## Adaptive capability updates

Protocol v3 appends event type 15 for a strict capability snapshot (pointer precision, hover, keyboard/touch/trackpad/stylus, density, text scale, reduced motion, and high contrast). Existing event numbers are unchanged. Invalid enum, unknown-field, and out-of-range text-scale payloads are ignored at the presenter trust boundary.

The Go engine and native presenters use the custom binary protocol in `pkg/render/protocol`.
Windows and Android both carry it through process-local exported read/write calls backed by
crossed in-memory pipes. Protocol v3 preserves every existing event number and appends pan and
pinch gestures carrying phase (begin/update/end/cancel), centroid, two-axis incremental delta,
and incremental scale. The engine can therefore render gestures locally without adding
platform-specific concepts to application nodes.

## Native timing on the debug channel

The `NativeDebugRequest`/`NativeDebugResponse` pair (message types 201/202) grew
inside v4 by the same append rule. The request appends a `resetPerf` flag; the
response appends a count-prefixed list of native timing phases, each carrying a
name, sample count, mean, p50, p95, p99, and max in milliseconds.

This exists because nothing in the C++ hosts timed frame CPU work — every
figure the Go tracker reports is measured on the Go side of the boundary, so the
native presentation cost was unmeasured. The host currently reports `present`
(geometry compilation plus draw submission on the UI thread), `decode_frame`,
`decode_map_scene`, and `apply_map_scene`. Percentiles use the same nearest-rank
definition and the same 4096-sample window as the Go tracker, so a native p95
and a Go p95 are comparable. The decoder bounds the phase count so a malformed
response cannot drive a large allocation. Consumed via `GET /perf/native` — see
[Performance & Latency Profiling](../design/automation/perf-profiling.md).

Protocol changes are cross-language changes: update both the Go and C++ implementations together and add or update round-trip tests in the same change.

## See also
- [Architecture Overview](../design/app/TDD.md)
- [Single-Process Windows Host](../design/architecture/windows-host.md)
- [Automation Overview](../design/automation/TDD.md)

# Citations
- [AGENTS.md](file:///d:/Programming/GUIProject/POEM/AGENTS.md) — Repo-Owned Protocol
