//go:build !windows

package render

import "github.com/mulavdm/poem/pkg/render/platform"

func withDefaultPlatformServices(services platform.Services) platform.Services { return services }
