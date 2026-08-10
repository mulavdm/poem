# Acoustic Native Sound Engine (Phase 4)

POEM integrates a native Acoustic Native Sound Engine that generates and plays real-time synthesized waveforms asynchronously without external dependencies or blocking the main drawing/rendering thread.

```
+------------------------------------+
|       internal/win32/audio.go      | <--- Pure Go 16-bit Mono PCM WAV Synthesizers
+------------------------------------+
                  | (WAV Bytes)
                  v
+------------------------------------+
|       pkg/render/types/audio.go    | <--- Pre-allocated buffer memory on ApplicationState
+------------------------------------+      (PlayHover / PlayClick / PlaySuccess API)
                  | (unsafe Pointer)
                  v
+------------------------------------+
|       internal/win32/win32.go      | <--- PlaySoundW Win32 API binding (winmm.dll)
+------------------------------------+
```

## Zero-Dependency Waveform DSP Synthesis (`internal/win32/audio.go`)

A pure mathematical oscillator in Go generates correct little-endian 16-bit Mono PCM WAV streams entirely in RAM:

- **Binary WAV Header Builder**: dynamically builds the standard 44-byte RIFF/WAVE header (specifying sample rate, block alignment, audio format, and subchunk sizes).
- **Acoustic Waveforms**:
  - **Hover Tick**: a clean sine wave at `1200.0` Hz, lasting `15` milliseconds. Applies an extremely rapid decay envelope (`math.Exp(-t * 220.0)`) for a soft, ultra-responsive interactive cursor click.
  - **Click Chime**: a rich dual-frequency chord combining a `900.0` Hz fundamental with a `1800.0` Hz octave harmonic. Blended and shaped over a `120` millisecond decay envelope (`math.Exp(-t * 28.0)`) for crisp tactile feedback.
  - **Success Melody**: an ascending sci-fi arpeggio sequence spanning `E5` (659.25 Hz), `A5` (880.0 Hz), `C#6` (1109.73 Hz), and `E6` (1318.51 Hz) over a `400` millisecond span.

## Low-Level winmm.dll Bindings (`internal/win32/win32.go`)

Windows' low-level multimedia API (`winmm.dll`) is loaded and its `PlaySoundW` procedure bound:

- **Memory Playback**: plays sound files directly from RAM buffers using standard flag combinations:
  - `SND_MEMORY` (`0x0004`): tells Windows that `pszSound` points to a WAV image loaded in RAM.
  - `SND_ASYNC` (`0x0001`): starts playback asynchronously and returns immediately.
  - `SND_NODEFAULT` (`0x0002`): prevents playing default system beep errors in case of failures.
- **Garbage Collection Safety**: the synthesized WAV slices are stored permanently inside `globalState` during application boot. This guarantees that Go's garbage collector never relocates or collects the memory addresses while active Win32 threads are playing them.

## Decoupled Interface Contract (`pkg/render/types/audio.go`)

To strictly follow POEM's Polylith architectural philosophy, low-level `unsafe` pointers and Win32 interop are hidden behind the `ApplicationState` abstraction:

- Exposes clean public methods `PlayHover()`, `PlayClick()`, and `PlaySuccess()` on `ApplicationState`.
- Allows application layouts (`ui.go`) or components to trigger audio feedback declaratively without importing `internal/win32` or utilizing pointer arithmetic.

## Interactive Mute, Rate-Limiting & Hover Filtering Hooks

Sound triggers are carefully routed inside the core window message loop to optimize user experience:

- **Interactive Hover Filtering**: hooked in `WM_MOUSEMOVE` in `run.go`. Triggers a hover tick ONLY when the cursor enters a new component bounding box (`newHover != "" && newHover != globalState.HoveredID`) AND queries that the target component is focusable (`comp.Focusable() == true`). This guarantees that static text, panels, or layout headers remain silent, focusing ticks exclusively on interactive button, slider, and input fields.
- **Auto-Repeat Debouncing Cooldowns**: exposed dynamic tracking timestamps (`LastHoverTime`, `LastClickTime`, `LastSuccessTime`) to rate-limit playback:
  - **Hovers**: 50ms cooldown gate.
  - **Clicks**: 150ms cooldown gate.
  - **Success Chimes**: 500ms cooldown gate.
  This completely filters Win32 keyboard auto-repeat message floods (e.g. when holding `Enter` or `Ctrl+S`), ensuring single chimes play cleanly without overlapping audio stutter.
- **Button Clicks**: hooked inside `WM_LBUTTONDOWN`, playing a click chime when clicking any active focusable component boundary.
- **Global Save & Input Submit**: plays the success arpeggio on global `"Ctrl+S"` save signals and console command line submissions.
- **Opt-in effects**: audio feedback is controlled through `AppConfig.Effects` and remains an application choice rather than a core-control default.

## See also
- [Architecture Overview](TDD.md)
