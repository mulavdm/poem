//go:build !windows

package render

import "go_native_gpu_gui/pkg/render/platform"

func withDefaultPlatformServices(services platform.Services) platform.Services { return services }
