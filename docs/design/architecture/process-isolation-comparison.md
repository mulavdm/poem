# Windows Hosting Evolution

| Architectural vector | M6 child-sidecar runtime | M7 single-process host |
| :--- | :--- | :--- |
| Process topology | Go executable plus C++ child | One product-named C++ process loading Go DLL |
| Application implementation | Compiled Go | Same compiled Go application and embedded runtime |
| Presentation | Win32, D3D11, UIA, IME, audio | Same native presenter code |
| Protocol transport | Two fixed renderer named pipes | Crossed in-memory pipes behind exported blocking calls |
| Deployment | Go EXE, resolved/extracted presenter | Portable host EXE + adjacent `poem_app.dll`, or MSIX identity |
| Failure boundary | Either process could orphan or disconnect | Native failure terminates the application process; cooperative close cancels Go |
| Native loading | Spawn/path/environment resolution | Absolute adjacent DLL, restricted search flags, ABI/export validation |
| Renderer protocol | POEM binary frames | Byte-identical POEM binary frames |

M7 trades process crash isolation for a simpler lifecycle, lower transport overhead, no extracted executable, and one observable application process. It does not translate application Go into C++ and does not duplicate product logic.

## See also

- [Single-Process Windows Host](../architecture/windows-host.md)
- [Native Presenter Protocol](../../reference/protocol.md)
- [DPI Coordinate Translation](../architecture/dpi-coordinate-translation.md)
