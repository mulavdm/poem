---
type: Concept
title: Single-Process Windows Host
description: The versioned Go DLL ABI, secure native loader, in-memory transport, lifecycle, and packaging contract.
tags: [windows, cgo, dll, d3d11, packaging, msix]
timestamp: 2026-07-18T00:00:00Z
---
# Single-Process Windows Host

M7 packages one application-specific Go c-shared library as adjacent `poem_app.dll` beside a generic product-named C++ host. The Windows loader uses the host executable's absolute directory and restricted `LoadLibraryExW` search flags. It resolves and validates ABI v1, all required exports, bounded JSON metadata, identity, title, and preferred dimensions before starting the engine. The module is intentionally never unloaded because it contains the Go runtime.

The exported contract is `PoemWindowsABIVersion`, `PoemWindowsMetadata`, `PoemWindowsStart`, `PoemHostRead`, `PoemHostWrite`, and `PoemWindowsStop`. `pkg/hosted` provides a shared bounded lifecycle and crossed in-memory pipe pair for Windows and Android. Partial transfers are legal; the native host retains the existing four-byte little-endian frame protocol and enforces a 64 MiB message limit.

The window host owns Win32, D3D11, UI Automation, IME, audio, DPI, dialogs, automation capture, and the message pump. The Go DLL owns the application, reducer, asynchronous commands, layout, semantics, font atlas, and frame generation. They are different native languages and runtimes inside exactly one OS process, not a Go-to-C++ translation.

Closing the window sends the normal protocol close event, cancels the engine cooperatively, closes transport endpoints to release blocking reads, and waits for bounded Go cleanup. Startup failures are reported before or alongside window creation and cannot leave a presenter child or extracted file.

`windows_host/build.ps1` produces a portable EXE plus DLL folder/ZIP and a packaged Win32 full-trust MSIX. Signing is optional and externally configured. Development certificate creation and certificate trust installation are separate commands so package construction never silently changes trust stores.

## See also

- [Protocol](/concepts/protocol.md)
- [Public API](/concepts/public-api.md)
- [Android Presenter](/concepts/architecture/android-presenter.md)

# Citations

- [README](../../../README.md)
- [M7 migration guide](../../../MIGRATION_M7.md)
