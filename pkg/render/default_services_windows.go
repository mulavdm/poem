//go:build windows

package render

import (
	"github.com/mulavdm/poem/pkg/render/platform"
	platformwindows "github.com/mulavdm/poem/pkg/render/platform/windows"
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
