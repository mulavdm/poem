//go:build android

// Package main builds as a c-shared library embedded in the POEM Android
// presenter APK. The C++ host calls PoemAndroidStart once the EGL surface
// exists; from there the engine runs exactly as it does on desktop, driving
// the same shared preferences.App.
package main

import "C"

import (
	"github.com/mulavdm/poem/pkg/mobile"
	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/examples/preferences"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

// PoemAndroidStart launches the POEM engine for the preferences app against the
// in-process presenter transport. width/height are the surface dimensions in
// physical pixels.
//
//export PoemAndroidStart
func PoemAndroidStart(width, height C.int) {
	config := desktop.Configure(preferences.App, render.AppConfig{Title: "Preferences", Effects: render.EffectsConfig{Audio: true}})
	mobile.Start(config, int(width), int(height))
}

func main() {}
