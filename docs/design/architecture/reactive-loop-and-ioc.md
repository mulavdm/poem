# Core Architecture & Reactive Loop

POEM uses a hosted re-evaluation loop driven over its native presenter protocol. Instead of manually updating widgets, applications write a page builder or semantic `App[S]` view.

The lifecycle:

1. The native presenter captures low-level window interactions and transmits them to the hosted Go engine over in-memory transport.
2. The Go orchestrator processes these events, mutates the global `ApplicationState`, and invokes the Page Builder function to rebuild the component hierarchy from scratch.
3. Go serializes the new drawing commands using the repo-owned binary protocol in `pkg/render/protocol`.
4. The presenter processes the frame asynchronously and flushes draw calls to the GPU.

```
+---------------------+   Input Event Batch        +-------------------+
| Native Process Host | -------------------------> |  Go Engine DLL    |
| (Win32 / D3D11 now) | <------------------------- |    (Game Loop)    |
+---------------------+   Render / Sound Commands  +-------------------+
                                                             |
                                                             v Triggers
                                                   +-------------------+
                                                   |  BuildPagesFn()   |
                                                   | (Rebuilds UI tree)|
                                                   +-------------------+
```

When application-owned state changes from a goroutine after IO, model work, or a service callback, call `render.RequestRepaint()` after publishing the new state. This marks the current frame dirty without enabling continuous animation and without coupling the application to the Windows backend.

## Design for External Integration (IoC)

The library uses inversion of control through `render.RunHosted`, called by `pkg/hosted`. External consumers do not write native Win32 callbacks, event routers, thread locking, or frame tickers. Windows applications register configuration through `pkg/windows`; Android applications start through `pkg/mobile`.

To instantiate the UI, host projects import `"github.com/mulavdm/poem/pkg/render"` and declare their component layout tree inside the `BuildPagesFn` callback, which the library automatically manages and updates dynamically.

## See also
- [Architecture Overview](TDD.md)
- [Public API Surface](../../reference/public-api.md)
- [Downstream App Example](../architecture/downstream-example.md)
