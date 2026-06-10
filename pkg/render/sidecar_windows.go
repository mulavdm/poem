//go:build windows

package render

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sidecarExeName = "poem_cpp_sidecar.exe"

//go:embed assets/windows_amd64/poem_cpp_sidecar.exe
var embeddedSidecarWindowsAMD64 []byte

func resolveSidecarPath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("POEM_SIDECAR_PATH")); envPath != "" {
		if fileExists(envPath) {
			return envPath, nil
		}
	}

	if exePath, err := os.Executable(); err == nil {
		localPath := filepath.Join(filepath.Dir(exePath), sidecarExeName)
		if fileExists(localPath) {
			return localPath, nil
		}
	}

	if wd, err := os.Getwd(); err == nil {
		candidates := []string{
			filepath.Join(wd, "cpp_sidecar", "build", "Release", sidecarExeName),
			filepath.Join(wd, "cpp_sidecar", "build", "Debug", sidecarExeName),
			filepath.Join(wd, "cpp_sidecar", "build", sidecarExeName),
			filepath.Join(wd, "..", "POEM", "cpp_sidecar", "build", "Release", sidecarExeName),
			filepath.Join(wd, "..", "POEM", "cpp_sidecar", "build", "Debug", sidecarExeName),
			filepath.Join(wd, "..", "..", "POEM", "cpp_sidecar", "build", "Release", sidecarExeName),
			filepath.Join(wd, "..", "..", "POEM", "cpp_sidecar", "build", "Debug", sidecarExeName),
		}
		for _, candidate := range candidates {
			if abs, err := filepath.Abs(candidate); err == nil && fileExists(abs) {
				return abs, nil
			}
		}
	}

	return extractEmbeddedSidecar()
}

func extractEmbeddedSidecar() (string, error) {
	if len(embeddedSidecarWindowsAMD64) == 0 {
		return "", fmt.Errorf("embedded Windows sidecar payload is missing")
	}

	cacheRoots := make([]string, 0, 2)
	if cacheRoot, err := os.UserCacheDir(); err == nil && strings.TrimSpace(cacheRoot) != "" {
		cacheRoots = append(cacheRoots, cacheRoot)
	}
	cacheRoots = append(cacheRoots, os.TempDir())

	sum := sha256.Sum256(embeddedSidecarWindowsAMD64)
	versionSuffix := filepath.Join("poem", "sidecar", "windows-amd64", hex.EncodeToString(sum[:8]))

	var mkdirErr error
	for _, cacheRoot := range cacheRoots {
		versionDir := filepath.Join(cacheRoot, versionSuffix)
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			mkdirErr = err
			continue
		}

		sidecarPath := filepath.Join(versionDir, sidecarExeName)
		if fileExists(sidecarPath) {
			return sidecarPath, nil
		}

		tmpPath := sidecarPath + ".tmp"
		if err := os.WriteFile(tmpPath, embeddedSidecarWindowsAMD64, 0o755); err != nil {
			mkdirErr = err
			continue
		}
		if err := os.Rename(tmpPath, sidecarPath); err != nil {
			_ = os.Remove(tmpPath)
			if fileExists(sidecarPath) {
				return sidecarPath, nil
			}
			mkdirErr = err
			continue
		}

		return sidecarPath, nil
	}

	return "", fmt.Errorf("extract embedded sidecar: %w", mkdirErr)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
