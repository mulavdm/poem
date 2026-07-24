//go:build android

// Package main builds as a c-shared library embedded in the POEM Android
// presenter APK. The C++ host calls PoemAndroidStart once the EGL surface
// exists; from there the engine runs exactly as it does on desktop, driving
// the component gallery.
package main

import "C"

import (
	"os"

	"github.com/mulavdm/poem/pkg/mobile"
	"github.com/mulavdm/poem/pkg/render"
)

// PoemAndroidStart launches the POEM engine for the component gallery against the
// in-process presenter transport. width/height are the surface dimensions in
// physical pixels.
//
//export PoemAndroidStart
func PoemAndroidStart(width, height C.int, dataDir *C.char) {
	_ = dataDir
	config := render.AppConfig{
		Title:        "POEM Component Gallery",
		Theme:        themes,
		BuildPagesFn: buildGallery,
		Accessibility: render.AccessibilityConfig{
			FollowSystemTheme: false,
		},
	}
	if automation, err := render.InspectionAutomationConfig(os.Getenv); err != nil {
		mobile.Logf("inspection disabled: %v", err)
	} else {
		config.Automation = automation
	}
	mobile.Start(config, int(width), int(height))
}

func main() {}
