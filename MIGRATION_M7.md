# M7 migration: single-process Windows host

M7 deliberately removes the Windows `render.Run` / `desktop.Run` executable path. A Windows application is now a Go c-shared DLL registered with `pkg/windows` and loaded by the generic native host.

Before:

```go
func main() {
    desktop.Run(App, render.AppConfig{Title: "Example", Width: 800, Height: 600})
}
```

After:

```go
func init() {
    config := desktop.Configure(App, render.AppConfig{Title: "Example", Width: 800, Height: 600})
    poemwindows.MustRegister(config, poemwindows.Metadata{
        Identity: "Example.App",
        Title: config.Title,
        Width: config.Width,
        Height: config.Height,
    })
}

func main() {}
```

Build and package through `windows_host/build.ps1`; do not invoke `go build` expecting a runnable Windows UI executable. The script compiles `poem_app.dll`, builds the MSVC host, and emits a portable ZIP plus MSIX.

Removed behavior:

- child presenter spawning and named renderer pipes;
- `POEM_SIDECAR_PATH` and development sidecar resolution;
- embedded presenter extraction and provenance payloads;
- direct `render.Run` and `desktop.Run` entrypoints.

The binary presenter protocol is unchanged and still shared with Android. Commands, reducer state, networking, rendering semantics, and application definitions remain Go; only process hosting and transport changed.
