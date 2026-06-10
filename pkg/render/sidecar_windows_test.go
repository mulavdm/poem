//go:build windows

package render

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSidecarPathExtractsEmbedded(t *testing.T) {
	originalEnv, hadEnv := os.LookupEnv("POEM_SIDECAR_PATH")
	if hadEnv {
		defer os.Setenv("POEM_SIDECAR_PATH", originalEnv)
	} else {
		defer os.Unsetenv("POEM_SIDECAR_PATH")
	}
	_ = os.Unsetenv("POEM_SIDECAR_PATH")

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer os.Chdir(originalWD)

	sidecarPath, err := resolveSidecarPath()
	if err != nil {
		t.Fatalf("resolveSidecarPath: %v", err)
	}
	if !fileExists(sidecarPath) {
		t.Fatalf("resolved sidecar path does not exist: %s", sidecarPath)
	}
	if filepath.Base(sidecarPath) != sidecarExeName {
		t.Fatalf("unexpected sidecar filename: %s", sidecarPath)
	}
}
