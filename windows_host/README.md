# POEM Windows host packaging

`build.ps1` compiles a Windows Go main package with `-buildmode=c-shared`, builds the generic MSVC `poem_windows_host`, and emits:

- `portable/<Product>.exe` plus adjacent `portable/poem_app.dll`;
- a portable ZIP;
- an unsigned or explicitly signed x64 MSIX.

Required application mains call `pkg/windows.MustRegister` during `init`. The host always loads the exact adjacent filename `poem_app.dll`; packaging renames only the generic host EXE.

```powershell
.\build.ps1 -ProjectDirectory .. -GoPackage ./cmd/gallery `
  -Identity POEM.Gallery -DisplayName "POEM Gallery" -Version 0.7.0.0 `
  -OutputDirectory ..\dist
```

The builder requires Go, UCRT64 GCC, MSVC/CMake, and Windows SDK MakeAppx. Pass `-SkipMSIX` for a portable-only development build.

For development signing, run `New-DevelopmentCertificate.ps1`; it creates or reuses the named certificate and exports its public `.cer`. Trust is a separate, confirmation-gated operation through `Trust-DevelopmentCertificate.ps1`. Pass `-CertificateThumbprint` to sign from the current-user store.

Production CI may instead pass `-CertificatePath` and provide its password through the environment variable named by `-CertificatePasswordEnvironment` (default `POEM_WINDOWS_CERT_PASSWORD`). Certificate files and secrets must remain outside the repository.
