# POEM Documentation

Documentation for the cross-platform GPU UI framework, organised per the
workspace documentation standard (`EOM repository-documentation-architecture-tdd`):
classify by purpose, namespace by subject, name by document type.

## Design subjects

| Subject | Specification | Covers |
| --- | --- | --- |
| [architecture](design/architecture/TDD.md) | `TDD.md` | The framework core: layout measurement, flexbox, the reactive loop and IoC, zero-GC rendering, thread locking, keyboard focus, DPI translation, Win32 syscalls, the Windows host and Android presenter, map rendering, sound, viewports and charts. |
| [app](design/app/TDD.md) | `TDD.md` | The `App[S]` application layer: adaptive design, component parity, the desktop driver, the web backend. |
| [automation](design/automation/TDD.md) | `TDD.md` | The inspection and automation surface: endpoints, capture modes, inspect flow, perf profiling and the `cmd/poemdrive` scenario runner, troubleshooting. |
| [web](design/web/) | — | The web target: accessibility, asset delivery, downstream consumption, family packages, the public component API, trusted HTML, vanilla class-based JS. |

## Other domains

| Path | What it answers |
| --- | --- |
| [`reference/protocol.md`](reference/protocol.md) | What exactly is the binary render protocol? |
| [`reference/public-api.md`](reference/public-api.md) | What exactly is the public Go API? |
| [`adr/`](adr/) | Why was a significant decision made? |
| [`cross_platform_cpp_framework_tdd.md`](cross_platform_cpp_framework_tdd.md) | The C++ framework design that `shared/poem/*.h` realizes. |
| [`archive/okf-bundle-log.md`](archive/okf-bundle-log.md) | The retired bundle's update ledger. |

This tree replaced an earlier documentation structure. The historical update
ledger is preserved at `archive/okf-bundle-log.md`.
