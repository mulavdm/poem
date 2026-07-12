# Releasing POEM

POEM is released independently from the workspace as `github.com/mulavdm/poem`.

1. Protect `v*.*.*` tags in the private repository.
2. Build and test the Release CMake sidecar, then run Go tests, race tests, vet, and `go mod verify`.
3. Confirm the embedded sidecar SHA-256 matches `cpp_sidecar/build/Release/poem_cpp_sidecar.exe`.
4. Push an annotated `v0.1.0` tag to start `.github/workflows/release.yml`.

The workflow packages the Windows gallery executable and native sidecar. The first POEM release
must exist before Trellis's tagged release workflow can consume it.
