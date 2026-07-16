//go:build android

// Package main builds as a c-shared library embedded in the POEM Android
// presenter APK. The C++ host calls PoemAndroidStart once the EGL surface
// exists; from there the engine runs exactly as it does on desktop, driving
// the same shared settings.App.
package main

import "C"

import (
	"github.com/mulavdm/poem/pkg/mobile"
	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/examples/settings"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

// PoemAndroidStart launches the POEM engine for the settings app against the
// in-process presenter transport. width/height are the surface dimensions in
// physical pixels.
//
//export PoemAndroidStart
func PoemAndroidStart(width, height C.int) {
	config := desktop.Configure(settings.App, render.AppConfig{Title: "Settings"})
	mobile.Start(config, int(width), int(height))
}

func main() {}
