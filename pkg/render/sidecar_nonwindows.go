//go:build !windows

package render

import "fmt"

func resolveSidecarPath() (string, error) {
	return "", fmt.Errorf("POEM sidecar auto-resolution is currently implemented for Windows only")
}
