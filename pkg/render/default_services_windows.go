//go:build windows

package render

import (
	"go_native_gpu_gui/pkg/render/platform"
	platformwindows "go_native_gpu_gui/pkg/render/platform/windows"
)

func withDefaultPlatformServices(services platform.Services) platform.Services {
	if services.Clipboard == nil {
		services.Clipboard = platformwindows.NewClipboard()
	}
	if services.Preferences == nil {
		services.Preferences = platformwindows.NewSystemPreferences()
	}
	return services
}
