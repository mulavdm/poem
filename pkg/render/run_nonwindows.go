//go:build !windows

package render

import "io"

// connectPresenter spawns the platform's out-of-process presenter. Only
// Windows has one (the D3D11/Rust sidecar); on other platforms the presenter
// hosts the engine in-process and enters through RunHosted instead.
func connectPresenter(config AppConfig) (toPresenter, fromPresenter io.ReadWriteCloser, cleanup func()) {
	panic("render.Run requires the Windows presentation sidecar; embed the engine via render.RunHosted on this platform")
}
