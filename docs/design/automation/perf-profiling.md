# Performance And Latency Profiling

POEM's shared automation layer includes lightweight timing hooks intended for downstream-app debugging and agent-driven diagnosis.

## `GET /perf/state`

Returns JSON with:

- `frames` — rolling frame timings for rebuild/render/serialize/write and total frame cost
- `frames.percentiles` — the distribution over a retained raw-sample window
- `event_batches` — timings for native input batches processed from the presenter
- `automation` — timings for HTTP automation commands handled inside POEM
- `events` — a capped recent event stream for quick inspection

### `frames.percentiles`

The `avg_*`/`max_*` fields alongside it are running aggregates and **cannot
answer a percentile budget**. `percentiles` reports `mean_ms`/`p50_ms`/`p95_ms`/
`p99_ms`/`max_ms` for `total`, `build_pages`, `render_pipeline`, `serialize`,
`write`, and `go_work` (build+render+serialize, the split budgeted separately
from transport write time).

- ranks are nearest-rank over the retained window; `sample_count` saturates at
  `sample_window` (4096 frames), oldest evicted first
- each phase is ranked on its own ordering — a frame that is p95 for
  serialization need not be p95 for layout
- this window is independent of `events`, which is capped at 256 entries shared
  with automation and event-batch entries and therefore wraps after roughly 128
  driven interactions

## `GET /perf/events`

Returns the recent perf event log. Frame entries carry the phase split twice:
`details` is `%.2f`-formatted text for humans and quantizes to 0.01 ms, while
`build_pages_ms`, `render_pipeline_ms`, `serialize_ms`, `write_ms`, and
`command_count` are the full-precision values consumers should read.

## `GET /perf/native`

Returns the **host's own** frame timings, which nothing in `/perf/state` can
see: every figure there is measured on the Go side of the boundary. One entry
per timed native channel, sorted by name so two snapshots can be diffed
positionally:

- `present` — geometry compilation plus draw submission on the UI thread. This
  is the native framework CPU work, excluding GPU execution.
- `decode_frame` — protocol decode of a `RenderFrame` on the transport thread
- `decode_map_scene` / `apply_map_scene` — map-scene decode and retention

Each carries `count`, `mean_ms`, `p50_ms`, `p95_ms`, `p99_ms`, `max_ms`, using
the same nearest-rank definition and the same 4096-sample window size as the Go
tracker, so a native p95 and a Go p95 mean the same thing.

The recorder itself lives in `shared/poem/perf.{h,cpp}`, so the D3D11 and GLES
presenters report the same channel names under the same percentile definition —
a p95 that meant one thing on Windows and another on Android would make the
comparison worthless. Both hosts implement it; the web backend and test drivers
return an error.

## `POST /perf/reset`

Clears accumulated perf counters, the event history, and the retained sample
window. It also asks the host to clear its native rings in the same call, so a
measurement scoped to one driven scene does not blend with native samples from
the previous one. Hosts with no native channel have nothing to clear and this
is not treated as an error.

## `POST /perf/measure-action`

This endpoint is useful when a caller wants "how long until the UI reaches a condition?" instead of only "how long did the command handler run?"

Example body:

```json
{
  "command": "click",
  "id": "wf_z-image-turbo-multi-lora_yaml",
  "timeout_ms": 8000,
  "poll_interval_ms": 20,
  "condition": {
    "component_id": "selected_workflow",
    "contains_text": "z-image-turbo-multi-lora.yaml"
  }
}
```

Supported condition fields:

- `focused_id`
- `current_page`
- `component_id`
- `contains_text`
- `window_width_at_least`
- `window_height_at_least`
- `min_frame_count_delta`

Example:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:47831/perf/measure-action `
  -ContentType 'application/json' `
  -Body '{"command":"click","id":"wf_z-image-turbo-multi-lora_yaml","timeout_ms":8000,"poll_interval_ms":20,"condition":{"component_id":"selected_workflow","contains_text":"z-image-turbo-multi-lora.yaml"}}'
```

## `POST /perf/measure-native-action`

This endpoint is for timing native window-control behavior rather than POEM-internal component actions.

Example body:

```json
{
  "maximize_window": true,
  "bring_to_foreground": true,
  "timeout_ms": 8000,
  "poll_interval_ms": 20,
  "condition": {
    "window_foreground": true,
    "window_not_minimized": true,
    "client_width_at_least": 1400
  }
}
```

Supported request action fields:

- `restore_window`
- `clamp_to_work_area`
- `bring_to_foreground`
- `maximize_window`

Supported native condition fields:

- `window_visible`
- `window_foreground`
- `window_not_minimized`
- `client_width_at_least`
- `client_height_at_least`
- `backbuffer_width_at_least`
- `backbuffer_height_at_least`
- `window_width_at_least`
- `window_height_at_least`
- `min_frame_count_delta`
- `min_event_batch_count_delta`

This is the preferred endpoint when the question is about:

- maximize or restore latency
- window-state readiness
- whether native resize handling produced follow-up frame or event activity
- desktop-facing sluggishness that a synthetic `POST /click` does not exercise

The response includes: `elapsed_ms`, `condition_met`, `frame_delta`, `perf`, `final_state`. This makes it possible to compare raw automation command cost, number of frames produced while the UI settles, and current render timings at the moment the condition is met.

Example:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:47831/perf/measure-native-action `
  -ContentType 'application/json' `
  -Body '{"maximize_window":true,"bring_to_foreground":true,"timeout_ms":8000,"poll_interval_ms":20,"condition":{"window_foreground":true,"window_not_minimized":true,"client_width_at_least":1400}}'
```

## Scenario replay: `cmd/poemdrive`

The endpoints above report *what a run cost*; they say nothing about whether
two runs did the same work. `cmd/poemdrive` supplies that half. It replays a
scenario file against a running app, then gates the timings against budgets and
a recorded baseline.

```bash
go run ./cmd/poemdrive scenes/gallery-tabs.json
go run ./cmd/poemdrive -save-baseline gallery.json scenes/gallery-tabs.json
go run ./cmd/poemdrive -baseline gallery.json scenes/gallery-tabs.json
```

Nothing about any app is compiled in: the scenario supplies `base_url` and the
step list, so the same binary drives the Windows gallery, MAPPS, or an Android
device. General steps are `click`, `focus`, `set-text`, `press-key`, `wait`,
`resize` (`{"width":..,"height":..}`, both required and positive; drives the
`/resize` endpoint to cross a responsive breakpoint deterministically instead
of a platform-specific window-management script), and `capture`. Canvas
scenarios additionally use `wheel`, `pan`, `pinch`, and
`pointer-drag`; these synthesize screen-space protocol events instead of
calling component methods directly, so the run exercises real overlay and
scroll-container routing. `assert-value` checks a component's semantic value
after an action. Targets that legitimately appear only after an earlier step
set `"dynamic": true`: they skip the initial fail-fast scan but are still
validated by the step itself with the automation server's detailed error.
Gesture steps accept an explicit `phase`, allowing begin, update, and end to be
separate HTTP requests with intervening frames. Scenarios should use that form
when validating retained controls: a single request containing every phase can
hide interaction state that was incorrectly stored only on a component instance
and lost by the next `BuildPages` rebuild.

## Functional-only scenarios and richer assertions

A scenario that exists to prove behavior rather than gate frame timing sets
`"functional": true`. The only behavioral difference is that `run()` does not
reject the result when zero frames were recorded — a pass built entirely from
assertion steps, or a click that legitimately repaints nothing, is not a
failure the way it is for a performance gate. Leave `budgets` unset on a
functional scenario; an empty map already checks nothing, so there is no
separate "skip the budget machinery" switch to reach for.

Beyond `assert-value` (substring match on one component's value), four more
assertion steps replace hand-written verification code without a screenshot:

- `assert-exists` / `assert-absent` — `{"id":"component_id"}`; the component
  must (or must not) be present in the current tree.
- `assert-count` — `{"id_prefix":"hierarchy_row_","count":3}`; the number of
  components whose id starts with `id_prefix` must equal `count` exactly
  (`count:0` is a valid, meaningful assertion — "none of these exist").
- `assert-no-overlap` — `{"ids":["a","b","c"]}`; every named id must be
  present, and no two of their bounding rectangles may intersect. This is the
  automatable half of the class of bug where a fixed-size layout slot
  overflows into its neighbor under specific content (wrapped button rows,
  overlapping wrapped text) — a screenshot catches it, but `assert-value` on
  either component's own text cannot, since neither component's *value* is
  wrong, only where it ends up drawn.

A scenario may also declare **how to bring its target up**, which is what makes
an Android run one command from a cold machine:

```json
"launch": {
  "platform": "android",
  "avd": "Medium_Phone",
  "package": "com.poem.prefs",
  "apk": "android_engine/prefs.apk",
  "build": { "script": "android_engine/build_apk.sh", "app_dir": "../examples/preferences/android", "label": "Preferences" },
  "forward_port": 47831,
  "inspection_property": "debug.poem.inspection"
}
```

Windows scenarios use the same block with `platform: "windows"`, an `exe`, an
optional PowerShell `build`, and `prepare_window` to restore and clamp the
window on screen before capturing. Readiness there means a **populated
component tree**, not merely that `/state` answers: a freshly launched app
serves `/state` before its first frame exists.

`prepare_window` only brings the window to the actual OS foreground
(alt-tabbing it in front of whatever else is on screen) when the scenario has
a `capture` step with `"source": "desktop"` — the one capture mode that reads
literal desktop pixels and so genuinely needs nothing drawn on top. Every
other capture source (`native`, the default; `self`; `window`) reads the GPU
backbuffer or internal render output directly and is unaffected by window
z-order, so foregrounding for those would be a needless, intrusive alt-tab —
especially disruptive across a batch of scenarios run back to back, each one
stealing focus for no benefit. `Scenario.NeedsForegroundWindow()` makes this
decision; a scenario that only needs restore-and-clamp still gets it (a
minimized or off-screen window still fails to present), just without the
foreground steal. When foregrounding does happen, it round-trips through the
native debug channel and the surface can stop answering while that is in
flight, so readiness is re-established afterwards rather than assumed.

**Each scene owns a port.** `forward_port` follows the port in `base_url`, and
`inspection_port_property` carries the same number to the device — forwarding
alone is not enough, because the app binds whatever port it was told and a
tunnel onto a different one lands on nothing. Two scenes on distinct ports run
against one machine simultaneously; `scenes/gallery-tabs.json` uses 47831 and
`scenes/android-preferences.json` 47832.

**`-teardown` stops what the run started, and only that.** Omit it while
chaining scenarios so the app or device stays warm, and pass it on the last one.
A launcher records the emulator only if it booted it, the app only if it started
it, and the forward only if it added it — so a scenario that borrowed a
developer's long-running emulator leaves it running, while one that booted its
own shuts it down. Teardown also runs on the failure paths, because a scenario
that dies half way must not strand an emulator; `os.Exit` skips deferred calls,
so the failure exits route through a helper that tears down first. Removing the
forward matters as much as stopping the app: left in place, a stale forward
keeps answering on the port and the next scenario silently drives the wrong app.

Before driving, every component id a scenario names is checked against the live
tree. A wrong id — or the right id against the *wrong app*, which happens when a
stale `adb forward` still owns the port — otherwise surfaces only as a
`400 Bad Request` from the middle of a warmup.

With that block the tool boots the AVD when no device is attached, builds the
APK if it is missing (or `-build` forces it), installs, sets the inspection
property, restarts the activity, and forwards the port — then drives. Use
`-no-launch` to drive something already running. A device that is already
attached always wins, so a physical phone is never displaced by an emulator.

Three details are load-bearing and were found by getting them wrong: the property
must be set **before** the process starts, since `pkg/mobile` reads it at
package init, hence setprop → force-stop → start; and `build_apk.sh` resolves
its app-directory argument against the working directory, so the tool runs it
from the script's own folder as its documented usage does rather than from the
repository root. On Windows, `poemdrive` also selects Git for Windows' Bash
explicitly. A generic `exec.LookPath("bash")` can resolve WSL's
`C:\Windows\System32\bash.exe`; that environment cannot directly use the
Windows Go and Android SDK/NDK tools exposed to the build as `/c/...` paths.

Metrics arrive in one flat namespace: `go.<phase>` from `frames.percentiles`
and `native.<phase>` from `/perf/native`, so a budget reads
`go.total.p95` or `native.present.p95`. Percentiles are taken from the
server-side ring rather than reconstructed by polling `/perf/events`, which is
both exact and free of the polling load that would otherwise perturb what is
being measured.

A `capture` step writes an image through the same surface, so a scenario ends
with both numbers and pixels:

```json
{ "action": "capture", "name": "checked", "source": "native" }
```

A state-changing step records the engine frame counter **before** it executes.
A later capture waits until the counter exceeds that saved boundary (or returns
immediately when it already has), because a click returns when the engine
*accepts* it, not when the resulting frame has been presented. Starting the
wait only when capture begins is too late: the desired frame may already have
arrived and the tool then waits for a nonexistent extra frame. The saved
pre-action boundary is exact where a fixed sleep is a guess.

`source` selects `native` (the presenter's own backbuffer, on either platform),
`go` (the engine-side reference rasterization), or `window`. Captures fire only
on the **final** pass through the step list: repeating them every cycle would
rewrite the same filenames and charge every cycle for a readback and PNG encode,
which measurably perturbs the timings — on the Android preferences scene it
moved `native.present` max from 6.5 ms to 2.0 ms. A scenario with capture steps
refuses to run without `-capture-dir` rather than skipping them silently.

**Steps are paced**, by default to 16 ms — one 60 Hz frame. Driving flat out
submits frames faster than the compositor retires them, so the renderer blocks
on swapchain back-pressure and the measurement becomes queue saturation rather
than framework cost. On the gallery scene that difference is
`native.present` p95 **5.39 ms unpaced against 0.79 ms paced**, and `go.total`
p95 5.07 ms against 1.04 ms — the back-pressure reaches back across the
boundary and stalls the Go writer too. Budgets are per-frame numbers, so a
scene that measures them must produce frames at a sustainable cadence. Set
`pace_ms: 0` deliberately to measure throughput under saturation instead.

Five behaviours are deliberate and worth keeping:

- **Warmup runs before the reset**, so first-paint costs and lazily built
  caches never enter the sample.
- **A budget naming a metric the run did not produce fails.** Passing because
  the measurement is absent is the worst outcome a gate can have, so a
  `native.*` budget against a host with no native debug channel reports exactly
  that rather than succeeding quietly.
- **A regression must exceed both a percentage and an absolute floor**
  (`regression_floor_ms`, default 0.5). Percentage alone is the wrong
  instrument near the timer resolution: Go-side timings on Windows quantize to
  roughly 0.5 ms, and two consecutive runs of an *unchanged* gallery binary
  were measured drifting 19–35% at p95/p99 for that reason alone. A gate that
  fails on an unchanged binary gets switched off.

## See also
- [Performance Benchmarks](../architecture/performance-benchmarks.md) — engine-level pprof/benchmarks
- [HTTP Endpoints](../automation/endpoints.md)
