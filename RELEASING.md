# Releasing POEM

POEM is released independently as `github.com/mulavdm/poem`. M7 ships Windows applications through the generic single-process native host; there is no embedded or separately resolved sidecar artifact.

1. Protect `v*.*.*` tags in the repository.
2. Run `gofmt`, all Go tests (including race tests), `go vet`, `go mod verify`, the CMake Release build, and CTest.
3. Verify an application DLL with `go build -buildmode=c-shared` and confirm all six ABI v1 exports are present.
4. Run `windows_host/build.ps1` to construct the portable folder/ZIP and unsigned MSIX. The portable bundle contains the product-named host EXE and adjacent `poem_app.dll`; the MSIX exposes one installed full-trust Win32 application identity.
5. For development signing, run `New-DevelopmentCertificate.ps1`, then pass its thumbprint to the builder. Trust installation is intentionally separate and opt-in through `Trust-DevelopmentCertificate.ps1`.
6. Production signing supplies a certificate path or thumbprint and certificate password environment variable from CI secrets. Do not commit PFX files or passwords.
7. Push an annotated `vX.Y.Z` tag. `.github/workflows/release.yml` builds and publishes the portable ZIP and MSIX. Signing hooks activate only when release CI provides external credentials.

Example package build:

```powershell
windows_host\build.ps1 -ProjectDirectory . -GoPackage ./cmd/gallery `
  -Identity POEM.Gallery -DisplayName "POEM Gallery" -Version 0.7.0.0 `
  -OutputDirectory .\dist
```

“Single process” describes the OS process topology, not a source-language translation: the installed process contains the C++ Windows presenter and the Go application/engine DLL with its embedded Go runtime.
