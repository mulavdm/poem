//go:build !windows

package render

import "fmt"

// startPipeAutomationServer is Windows-only (named pipes); on other platforms
// automation is served over HTTP mode instead.
func startPipeAutomationServer(cfg AutomationConfig) {
	fmt.Printf("POEM automation pipe mode is Windows-only; use HTTP mode (requested pipe %q)\n", cfg.PipeName)
}
