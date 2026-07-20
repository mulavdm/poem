package windows_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Windows host's engine-reader thread must never publish to the UI thread
// with a *synchronous* SendMessageW. Doing so blocks the reader until the UI
// thread replies, and the UI thread can be inside SendEvent -> WriteMessage
// waiting on an engine that cannot drain — because this reader is the only
// thing draining it. That three-way deadlock left the window blank and
// unresponsive right after startup.
//
// This guard is deliberately static. The runtime smoke test
// (host_smoke_windows_test.go) asserts the window pumps, but it was verified to
// PASS against the buggy host: reproducing the race needs a large engine→host
// burst (MAPPS floods the pipe with map-scene geometry) that a test fixture
// cannot cheaply recreate. So the runtime test is a broad guard against startup
// hangs, and this one deterministically pins the specific invariant that was
// violated.
//
// Every engine→UI hand-off in the reader must use PostMessageW; WM_POEM_*
// handlers take ownership of any heap payload precisely so posting is safe.
func TestWindowsHostReaderNeverSendsSynchronously(t *testing.T) {
	path := filepath.Join("..", "..", "cpp_sidecar", "src", "main.cpp")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("host source unavailable (%v)", err)
	}
	source := string(body)

	const marker = "std::thread reader"
	start := strings.Index(source, marker)
	if start < 0 {
		t.Fatalf("could not find the engine-reader thread in %s; update this guard if the host was restructured", path)
	}
	// Scope the check to the reader thread body: the UI thread itself may
	// legitimately use SendMessageW, but the reader never may.
	rest := source[start:]
	if end := strings.Index(rest, "\n    });"); end > 0 {
		rest = rest[:end]
	}
	// Match the call, not the word: the fix's own comment names SendMessageW
	// while explaining why it must not be used here.
	if strings.Contains(rest, "SendMessageW(") {
		t.Fatal("engine-reader thread uses a synchronous SendMessageW: this deadlocks the UI thread on startup (see WM_POEM_SEMANTICS). Use PostMessageW and let the handler own the payload.")
	}
}
