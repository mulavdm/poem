# Public API Surface

## M9 vector-map and live-source contracts

`MapViewportNode` owns a controlled `MapCamera` (center, zoom, bearing, pitch), registered `MapSource`, typed semantic `MapStyleSet`, stable feature records, quality/cache policy, load semantics, and camera/feature messages. Sources name a provider and contain no arbitrary URL or credential. `App.MapResources` derives providers from the same serialized state snapshot; native calls them directly and web uses a session-validated same-origin broker.

`App.Subscriptions` derives keyed multi-message sources. Revision changes cancel and supersede the prior generation, and late emissions are rejected under the same serialized reducer lock used for commands. `App.Services` contains host capabilities that are attached only to command/subscription contexts; `ServicesFromContext` exposes location, speech, haptics, notifications, wake, preferences, and secure storage without serializing their implementations or secrets.

`pkg/cartography` provides the shared bounded MVT, camera/tile-cover, typed-style, deterministic scene-buffer, concave polygon/hole/multipolygon tessellation, stable application-overlay, picking, and pinned pure-Go OpenType-shaping core. Polygon rings are normalized and bounded before the pure-Go earcut path; emitted indices must cover exactly the exterior-minus-holes area or the malformed feature is discarded. `BuildLabelCandidates` evaluates bounded typed tile rules with semantic size, weight, filter, and priority, `PlaceLabels` performs deterministic camera-aware grid collision, and `AddLabelPlacements` rasterizes exact shaped glyph IDs into a bounded retained alpha atlas and geographic-anchor/screen-offset quads. Filtered POI categories travel in the existing vertex stride and resolve to distinct shared-shader silhouettes on D3D11 and GLES3. The native runtime uses the actual laid-out map viewport—not the enclosing window—for tile cover and collision, and both presenters cache atlas textures by content hash. Direct GPU reuse of vertex/index resources and the Go/WASM WebGL worker remain release gates.

## Inspection and performance contracts

`render.InspectionAutomationConfig(getenv func(string) string) (*AutomationConfig, error)`
is the single opt-in gate for the control-capable automation surface. It returns
nil unless `POEM_INSPECTION=1`, validates `POEM_INSPECTION_PORT` to 1..65535,
always binds loopback, and sets `Verbose` so an enabled surface announces its
bound address. `render.DefaultInspectionPort` is 47831. Hosts that cannot
express a command line translate their own switch into this environment
contract rather than inventing a per-platform one — Android reads the
`debug.poem.inspection` system property in `pkg/mobile` and calls `os.Setenv`
itself, because the Go runtime snapshots the environment at init.

`PerfFrameStats.Percentiles` (`render.PerfFramePercentiles`) reports
`render.PerfPhaseStats` (mean/p50/p95/p99/max) for `total`, `build_pages`,
`render_pipeline`, `serialize`, `write`, and `go_work` over a retained
4096-sample ring, independent of the 256-entry `PerfEvent` trace. The running
`Avg*`/`Max*` fields remain, but cannot answer a percentile gate. Frame
`PerfEvent`s additionally carry `BuildPagesMS`, `RenderPipelineMS`,
`SerializeMS`, `WriteMS`, and `CommandCount` as full-precision values; the
`Details` string carries the same numbers `%.2f`-formatted and is for humans.

`render.NativePerfState`/`NativePerfPhaseState` expose the host's own timings,
which nothing on the Go side can observe. On the wire these are
`protocol.NativePerfPhase` on `NativeDebugResponse.PerfPhases`, with
`NativeDebugRequest.ResetPerf` clearing the host rings — both appended inside
protocol v4. `mobile.Logf` writes to logcat from application code, for
bootstrap diagnostics emitted before the engine has usable stderr.

Percentile ranks are nearest-rank and defined identically in Go and in
`shared/poem/perf.{h,cpp}`, so a native p95 and a Go p95 are comparable.

## M8 startup and workspace contract

`App[S].Start` is an optional once-only reducer hook. Native drivers invoke it once per hosted engine lifetime; the web driver invokes it only when creating a session, never when restoring one. Immediate state is visible in the first view and its `Cmd` uses the ordinary keyed executor.

`WorkspaceNode` carries semantic title/subtitle, header commands, status, primary content, and tools. Tools dock beside content when expanded and become a component-owned bottom sheet on compact touch layouts; the no-JavaScript web baseline keeps them as an ordinary accessible section. `SectionNode`, action importance/placement/selected state, interactive collection items, and semantic map/location/connection icons preserve intent through native and web lowering. Selected actions lower to native selected control state and web `aria-pressed` without claiming primary-action importance. Stateful sections use semantic `Running` and `Empty` flags; design lint emits `UI072` or `UI073` unless the current tree includes accessible progress or visible empty-state guidance.

`ImageViewportCommit` adds validated logical width/height to the controlled transform. Legacy transform-only payloads remain valid. Applications can use immutable `design.Overrides` through `System.WithOverrides`; resolution recomputes contrast-safe dependent colors and does not override forced/high-contrast system semantics.

## M6 semantic design contract

`App[S]` optionally carries `Design *design.System` and `Commands func(S) []app.Command`. Semantic actions reference state-derived commands by stable ID while invocation remains a transport-safe `Msg`. The semantic catalog covers labelled fields, actions, collections, destinations/navigation, status, progress, disclosure, controlled dialogs, forms, workspaces, media/viewports, and four-class `AdaptiveNode`. Application nodes carry stable accessible identity and no raw visual styling. `pkg/render` remains available for advanced native components and receives the same resolved design environment.

`pkg/app/designlint` keeps semantic `Lint` compatible and adds `LintTheme` for resolved semantic-token contrast plus `LintLayout` for rendered geometry. The corresponding enforcement codes are `UI042` for raw map colors, `UI043` for low-contrast token pairs, `UI023` for clipped text, `UI102` for unreachable actions, and `UI103` for focus order that conflicts with visual order.

Since the 2026-07-16 [consolidation](../adr/0002-consolidation.md), POEM has two
public surfaces at different altitudes:

**`pkg/app` — the application API.** Applications import `github.com/mulavdm/poem/pkg/app`
and write an `App[S]` (serializable state, pure `View`/`Update`, named `Msg`s, keyed asynchronous
`Cmd`s), launched via `pkg/app/web.Run`, registered for Windows through
`desktop.Configure` plus `pkg/windows`, or hosted on Android through `pkg/mobile.Start` from a
`c-shared` main). This is the recommended way to build applications: one definition runs on
every target. See [Application Layer](../design/app/TDD.md).

`Update` returns `(S, Cmd)`. A zero `Cmd` means no work; a non-zero command runs with a
cancellable context and returns a runtime-local typed value or error through a completion
`Msg`. Its name is also its concurrency key: different names may overlap, while a newer command
with the same name cancels and supersedes the older result.

The catalog includes `ImageViewportNode`, a reusable controlled and annotated image viewport. Its
`ImageTransform` uses viewport-relative offsets and scale, normalizes zero scale to one, and
defaults to a 0.5–8 interactive range. `ImageTransformPayload` and `Msg.ImageTransform` are the
strict cross-target encoding contract; malformed, non-finite, and out-of-envelope values are
rejected before they reach reducers.
`ViewportPointPayload`/`Msg.ViewportPoint` provide the same strict validation for normalized
background activation. `ImageMarker` annotations carry stable IDs, normalized image coordinates,
accessible labels, semantic variants, selection/disabled state, and their own activation message.
Tap, marker selection, and drag/pinch are mutually exclusive.

`ResponsiveNode` selects Compact or Wide node groups from available logical width (600 by default).
It is layout-only and adds no application state; the web target keeps Compact as its no-JavaScript
baseline and disables the inactive branch's form controls.

**`pkg/render` — the engine layer.** Advanced native-only apps may create a
`render.AppConfig` with a `BuildPagesFn` (`cmd/gallery` does), then register it through
`pkg/windows`. Its component event handlers are Go closures, so
this surface cannot drive the web target — the reason `pkg/app` exists. `pkg/render`
re-exports subpackage types via aliasing (`render.go`) so consumers never import
`components`/`layout`/`types`/`protocol`/`state` directly. Consumers do not need to know
which native presenter (Windows D3D11 host or Android GLES host) runs underneath.

**`pkg/windows` — the native Windows host ABI.** Windows c-shared mains call
`MustRegister` with configured application state and validated metadata. The package exports
ABI v1 metadata/start/read/write/stop functions to the generic adjacent host. This is a deliberate
M7 replacement for `render.Run`, `desktop.Run`, sidecar resolution, and embedded presenter assets.

`pkg/web` (HTML components + middleware) is public for hand-built server pages, and is what
the web driver renders through.

Treat every exported symbol on these surfaces as a contract: add Go doc comments, and
document deliberate breaking changes in the same commit that makes them.

## See also
- [Application Layer](../design/app/TDD.md)
- [Architecture Overview](../design/app/TDD.md)
- [Native Presenter Protocol](protocol.md)
- [Single-Process Windows Host](../design/architecture/windows-host.md)

# Citations
- [README.md](file:///d:/Programming/GUIProject/POEM/README.md)
- [AGENTS.md](file:///d:/Programming/GUIProject/POEM/AGENTS.md) — Public API Stability
