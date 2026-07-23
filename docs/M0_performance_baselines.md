# M0 — Performance Baselines

Prerequisite for [Plan.txt](Plan.txt): the migration's performance gates compare
against these numbers, so they must exist before any migration work starts.
Captured 2026-07-23 on the Windows host (Release), 1475x975 backbuffer at 120 DPI
for the gallery, 1125x875 at 120 DPI for MAPPS.

## Headline

**The 120 Hz targets in the plan are not met today on the map scene, by a wide
margin, and that has nothing to do with the migration.** This is exactly why the
primary gate is *no regression against these baselines* and the absolute targets
are tracked separately. Had the absolute numbers been the gate, the migration
would have been blocked on a pre-existing condition it was never scoped to fix.

| Scene | Go build+render+serialize p95 | Budget | Frame total p95 | One 120 Hz frame |
|---|---:|---:|---:|---:|
| Gallery — tab switching | **1.14 ms** | 2.08 ms | 3.98 ms | 8.33 ms |
| MAPPS — map camera changes | **31.8 – 49.8 ms** | 2.08 ms | 57.0 – 82.9 ms | 8.33 ms |

Gallery is inside budget. MAPPS is 15–24x over on Go work alone.

## Gallery — tab-switch scene

600 samples. Driven by cycling the four gallery tabs (`controls`, `inputs`,
`navigation`, `feedback`), each click forcing a full page rebuild. ~110 draw
commands per frame.

| Metric | mean | p50 | p95 | p99 | max |
|---|---:|---:|---:|---:|---:|
| frame total | 1.309 | 0.995 | 3.977 | 6.868 | 8.095 |
| go build+render+serialize | 0.258 | 0.000 | 1.140 | 1.810 | 4.550 |
| — build pages | 0.043 | 0.000 | 0.390 | 1.000 | 3.590 |
| — render pipeline | 0.092 | 0.000 | 0.560 | 1.430 | 2.210 |
| — serialize | 0.123 | 0.000 | 0.630 | 1.510 | 4.040 |
| — write (transport) | 0.278 | 0.000 | 1.250 | 2.980 | 5.880 |

All values in milliseconds. The p50 zeroes are real: most repaints do
near-zero work in a given phase, and the split figures are quantized (see
Instrumentation gaps).

## MAPPS — map camera scene

Two independent runs, driven by alternating Zoom in / Zoom out against the live
Docker stack (gateway, Martin, Pelias, Valhalla, Elasticsearch, all healthy).
~96 draw commands per frame.

| Metric | run A (n=500) p95 | run A p99 | run B (n=606) p95 | run B p99 |
|---|---:|---:|---:|---:|
| frame total | 57.015 | 90.427 | 82.883 | 109.573 |
| go build+render+serialize | 31.810 | 59.550 | 49.780 | 77.170 |
| — write (transport) | 31.890 | 41.770 | 41.380 | 66.060 |

Run A recorded a 1044 ms outlier as its max, almost certainly first-scene
generation or a tile-fetch stall; run B's max was 189 ms.

**These two runs differ by ~50%, which is itself a finding.** The zoom sequence
does not return to a fixed camera, so successive runs cover different geometry
densities. Treat MAPPS numbers as indicative, not as a locked baseline, until
the scene is pinned (below).

## Instrumentation gaps

Four gaps blocked turning these into an automated gate. **Gaps 1–3 are now
closed**; the numbers above were captured before that work and remain the
recorded pre-migration baseline.

1. ~~**No native-side frame timing exists at all.**~~ **Closed.**
   `cpp_sidecar/src/perf.{h,cpp}` now times `present` (geometry compilation plus
   draw submission on the UI thread, excluding GPU execution), `decode_frame`,
   `decode_map_scene`, and `apply_map_scene`, and reports percentiles over the
   `NativeDebugResponse` channel. Read it with `GET /perf/native`.
2. ~~**The tracker exposes only avg and max.**~~ **Closed.** A dedicated
   4096-sample frame ring backs `frames.percentiles` on `GET /perf/state`,
   independent of the 256-entry event trace. No polling-faster-than-the-ring
   required.
3. ~~**The phase splits are quantized to 0.01 ms.**~~ **Closed.** Frame events
   carry `build_pages_ms`, `render_pipeline_ms`, `serialize_ms`, `write_ms`, and
   `command_count` as full-precision fields alongside the `%.2f` `details` text.
4. **There is no repaint clock.** Still true, and not a defect: POEM repaints on
   change, so an idle app produces zero frames and "120 Hz" is not an observable
   steady state. Every baseline is per-interaction and the gate must be phrased
   that way.

### First native measurement

Gallery tab-switch scene, 211 frames, same driving loop as above:

| Native phase | count | mean | p50 | p95 | p99 | max |
|---|---:|---:|---:|---:|---:|---:|
| `present` | 211 | 0.886 | 0.448 | **0.948** | 11.112 | 21.701 |
| `decode_frame` | 211 | 0.009 | 0.007 | 0.019 | 0.026 | 0.054 |

`present` p95 is 0.95 ms against the plan's 4.17 ms budget — comfortably inside.
**The tail is the concern, not the median:** p99 is 11.1 ms and max 21.7 ms,
both past a full 8.33 ms frame. Protocol decode is negligible.

### Remaining precision caveat

Go-side timings on Windows come back quantized to roughly 0.5 ms steps
(`0.5182`, `1.0028`, `1.5149`…), which is coarse against a 2.08 ms budget. This
predates the percentile work and needs confirming before the Go-side gate is
tightened; the native timings do not show the same quantization.

## Scenes — now defined

`Plan.txt` referred to "the defined gallery and MAPPS scenes" when no such
definition existed. **Closed.** They are fixtures in `scenes/`, replayed by
`cmd/poemdrive`, and validated by that command's test suite so a typo in a
fixture fails the build:

- `scenes/gallery-tabs.json` — cycles `controls → inputs → navigation →
  feedback`, 600 iterations after a 20-iteration warmup.
- `scenes/mapps-camera.json` — a **balanced** zoom cycle returning to its
  starting zoom, which is the fix for the ~50% run-to-run variance recorded
  above: unbalanced relative zoom clicks left each run at a different depth
  over different geometry. It carries no budgets deliberately (see below).

Re-measuring the gallery scene through `poemdrive` gives materially lower
numbers than the table above — `go.total` p95 ≈ 0.55 ms rather than 3.98 ms —
because percentiles now come from the server-side ring instead of being
reconstructed by polling `/perf/events` during the run. **The figures in this
document remain the recorded pre-migration baseline**, but they include the
polling load that reconstruction required; `poemdrive` numbers are not
comparable to them and a fresh baseline should be captured with the tool before
gating on it.

### Pacing decides what is being measured

An unpaced driver submits frames faster than the compositor retires them, and
the renderer then blocks on swapchain back-pressure. The gallery scene measures
`native.present` p95 at **5.39 ms unpaced and 0.79 ms paced** to one 60 Hz
frame; `go.total` p95 moves 5.07 → 1.04 ms over the same change, because the
back-pressure propagates across the boundary and stalls the Go writer as well.

The unpaced figure is not wrong, it answers a different question — throughput
under saturation, not per-frame cost. The plan's budgets are per-frame numbers,
so scenes that gate on them pace by default (`pace_ms`, 16 ms).

This also explains why the earlier polling-based measurements above looked
*better* on the native channel than the first `poemdrive` runs: PowerShell's
per-request overhead was accidentally pacing them.

### The floor matters more than the tolerance

Two consecutive `poemdrive` runs of an **unchanged** gallery binary drifted
19–35% at p95/p99. That is not instability in the app: Go-side timings on
Windows quantize to roughly 0.5 ms (the caveat above), the gallery's p95 sits
at ~0.55 ms, and a single-bucket move therefore reads as a large percentage.
A regression must now clear an absolute floor (`regression_floor_ms`, default
0.5) as well as the percentage, which makes consecutive runs pass while a
doctored baseline representing a 4x regression still fails on six metrics.

Gating a sub-millisecond metric on percentage alone produces a gate that fails
on unchanged code, and such a gate gets switched off.

## Reproducing

Gallery, with the packaged build running and automation on `127.0.0.1:47831`:

```bash
curl -X POST http://127.0.0.1:47831/perf/reset
```

Then drive the tab cycle and poll `/perf/events` faster than the ring wraps,
de-duplicating on `timestamp`. MAPPS is the same procedure after
`scripts\build-desktop.ps1 -Run -Inspection`, driving the zoom buttons
(`trellis-root/content/0/1/0` and `/1`), and requires the Docker stack up.
