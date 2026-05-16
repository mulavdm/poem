## 🏗️ Architectural Philosophy: The Modular Polylith

1.  **Idiomatic Modularity**: Favor small, focused packages. Keep OS-specific syscalls in `internal/win32`, rendering logic in `internal/render`, and application entry in `cmd/engine`.
2.  **Loose Coupling (Polymorphism)**: The high-level application and UI components must remain agnostic of the rendering backend. All rendering-specific logic must be hidden behind the `UIRenderer` and `Painter` interfaces. 
    *   *Goal*: Switching from CPU to GPU rendering should require only a build tag change, with zero modifications to `main.go` or `components.go`.
3.  **Interface-First Design**: Before implementing a new feature (e.g., layout, input, sound), define its contract via an interface. This allows for mocking in tests and strategy switching.
4.  **Standalone Library Design**: Treat PolyEngine as a consumable package. All public interactions must happen through the `render` package's interfaces. Implementation details from `internal/win32` or specific backends must never leak into the user's application code.

---

## 🛠️ Development & Validation Workflow

### 🧪 Performance-Driven Development
Every major rendering or layout change **MUST** be validated through the internal diagnostic suite:
- **Unit Tests**: Ensure logical correctness (e.g., hit-testing, coordinate calculations).
- **Benchmarks**: New shapes or blending algorithms must include a benchmark in the relevant `_test.go` file. Target: **< 10ms** per frame.
- **Profiling**: Use the `pprof` server (`http://127.0.0.1:6060`) to identify hot loops. Avoid `math.Sqrt` or `reflect` in per-pixel render loops.

### 📦 Build Hygiene
- **Build Tags**: Use `//go:build !gpu` for CPU-only paths and `//go:build gpu` for hardware-accelerated paths.
- **Zero Dependencies**: Maintain the "Zero-Dependency" promise for the CPU path. Only `golang.org/x/...` packages are permitted.

---

## ⚠️ Common Gotchas & Constraints

- **Thread Stability**: `runtime.LockOSThread()` is non-negotiable for window management.
- **Win32 Error Handling**: Check return values (e.g., `ret != 0`) before checking the `error` object.
- **GDI Blitting**: Never use `SetPixel`. Use the `CPUEngine.PixelBuffer` and `StretchDIBits` for bulk transfers.
- **Alpha Blending**: Optimization is key. Manual loops are preferred over generic library calls if it improves per-pixel performance.
- **Pointer Safety**: Ensure Win32 pointers are properly managed to prevent GC interference during syscalls.

---

## 📂 Project Structure Map
- `cmd/engine/`: Main event loop and message handling.
- `internal/win32/`: Low-level OS interaction (No logic here).
- `internal/render/`: UI Engine logic & Component definitions.
- `ARCHITECTURE.md`: Technical deep-dive on the engine's design.