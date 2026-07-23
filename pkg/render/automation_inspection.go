package render

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultInspectionPort is the loopback port POEM's inspection surface binds
// when enabled. It matches the desktop convention so one tooling script can
// drive either platform after an `adb forward` of the same number.
const DefaultInspectionPort = 47831

// InspectionAutomationConfig returns an opt-in automation configuration, or nil
// when inspection is not requested.
//
// The automation surface is control-capable — it can click, type, and read the
// component tree — so it must never be on by default in a shipped application.
// This is the shared gate for that decision: hosts that cannot express a
// command line (Android's NativeActivity, packaged desktop apps) all read the
// same environment contract instead of each inventing one.
//
//   - POEM_INSPECTION=1 enables it; anything else leaves it off.
//   - POEM_INSPECTION_PORT overrides the port, validated to 1..65535.
//
// It always binds loopback. On Android that still means any process on the
// device can reach it, so enable it on development builds only.
func InspectionAutomationConfig(getenv func(string) string) (*AutomationConfig, error) {
	if getenv == nil {
		return nil, nil
	}
	if strings.TrimSpace(getenv("POEM_INSPECTION")) != "1" {
		return nil, nil
	}
	port := DefaultInspectionPort
	if raw := strings.TrimSpace(getenv("POEM_INSPECTION_PORT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			return nil, fmt.Errorf("POEM_INSPECTION_PORT must be an integer from 1 through 65535")
		}
		port = parsed
	}
	return &AutomationConfig{
		Enabled: true,
		Mode:    "http",
		Host:    "127.0.0.1",
		Port:    port,
		// Announce the bound address. This is an explicitly opted-in debug
		// surface, and on a device with no console the log line is the only
		// way to tell "listening" from "silently never started".
		Verbose: true,
	}, nil
}
