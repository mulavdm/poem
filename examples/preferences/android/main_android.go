//go:build android

// Package main builds as a c-shared library embedded in the POEM Android
// presenter APK. The C++ host calls PoemAndroidStart once the EGL surface
// exists; from there the engine runs exactly as it does on desktop, driving
// the same shared preferences.App.
package main

import "C"

import (
	"os"

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
func PoemAndroidStart(width, height C.int, dataDir *C.char) {
	_ = dataDir // preferences resolve via HOME (set by the host)
	base := render.AppConfig{Title: "Preferences", Effects: render.EffectsConfig{Audio: true}}
	// Opt-in only: the host exports POEM_INSPECTION when
	// `debug.poem.inspection` is set. An invalid port is a configuration
	// mistake worth surfacing, but it must not stop the app from running.
	if automation, err := render.InspectionAutomationConfig(os.Getenv); err != nil {
		mobile.Logf("inspection disabled: %v", err)
	} else {
		base.Automation = automation
	}
	config := desktop.Configure(preferences.App, base)
	mobile.Start(config, int(width), int(height))
}

func main() {}
