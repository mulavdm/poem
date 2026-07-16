# Architecture Concepts

* [Overview](/concepts/architecture/overview.md) — portability boundary and modular polylith topology
* [Android Presenter](/concepts/architecture/android-presenter.md) — the C++/GLES2 in-process presenter, `pkg/mobile` transport, density model, and startup contract
* [Thread Locking](/concepts/architecture/thread-locking.md) — Go runtime vs. the OS thread
* [Win32 Syscall Stabilization](/concepts/architecture/win32-syscalls.md) — the "success" trap
* [Process Isolation Comparison](/concepts/architecture/process-isolation-comparison.md) — GDI/OpenGL vs. Go+C++ sidecar
* [Cosine Spline Charts](/concepts/architecture/cosine-spline-charts.md) — `LineChart` interpolation math
* [Cursor and Layout Sync](/concepts/architecture/cursor-and-layout-sync.md) — dynamic cursor system and pre-hit-test layout pass
* [Scroll Viewports](/concepts/architecture/scroll-viewports.md) — clipping, coordinate translation, drag capture
* [Performance Benchmarks](/concepts/architecture/performance-benchmarks.md) — pprof and Go benchmarks
* [Zero-GC Rendering](/concepts/architecture/zero-gc-rendering.md) — async repaint, zero-alloc buffers, delta physics
* [Reactive Loop & IoC](/concepts/architecture/reactive-loop-and-ioc.md) — the page-rebuild loop and `render.Run`
* [Keyboard Focus Engine](/concepts/architecture/keyboard-focus-engine.md) — focus cycling, scroll centering, hotkeys
* [Sound Engine](/concepts/architecture/sound-engine.md) — synthesized waveform audio pipeline
* [DPI Coordinate Translation](/concepts/architecture/dpi-coordinate-translation.md) — logical vs. physical coordinate spaces
* [Component Catalog](/concepts/architecture/component-catalog.md) — panels, labels, buttons, sliders, inputs
* [FlexBox Layout](/concepts/architecture/flexbox-layout.md) — multi-axis positioning
* [Layout Measurement](/concepts/architecture/layout-measurement.md) — `Measure()`/`ContentSize()` contract
* [Downstream App Example](/concepts/architecture/downstream-example.md) — complete copy-pasteable example
