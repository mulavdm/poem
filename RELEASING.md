# Releasing POEM

POEM is released independently as `github.com/mulavdm/poem`. Since the 2026-07-16
consolidation it is the only released module — the former GopherWeb/Trellis sibling-tag
ordering is gone.

1. Protect `v*.*.*` tags in the private repository.
2. Build and test the Release CMake sidecar, then run Go tests, race tests, vet, and `go mod verify`.
3. If `cpp_sidecar/` sources changed since the embedded sidecar was last built: rebuild
   Release, copy `cpp_sidecar/build/Release/poem_cpp_sidecar.exe` to
   `pkg/render/assets/windows_amd64/`, and regenerate the provenance manifest in the same
   commit:

   ```pwsh
   $current = (git ls-files -s cpp_sidecar) -join "`n"
   $hash = (Get-FileHash -InputStream ([IO.MemoryStream][Text.Encoding]::UTF8.GetBytes($current)) -Algorithm SHA256).Hash
   Set-Content -NoNewline -Encoding ascii pkg/render/assets/windows_amd64/poem_cpp_sidecar.provenance $hash
   ```

   CI verifies this manifest against the committed sources (git blob hashes), proving the
   embedded binary was built from the sources in the tree. It deliberately does **not**
   bit-compare against a fresh runner build — MSVC image updates change codegen, so that
   comparison can never stay green.
4. Push an annotated `vX.Y.Z` tag to start `.github/workflows/release.yml`, which packages
   the Windows gallery executable and a freshly built sidecar.
