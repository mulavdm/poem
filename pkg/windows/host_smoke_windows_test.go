//go:build windows

package windows_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// This is a startup smoke test for the Windows native host: it launches the
// real host against a real app DLL and asserts the window actually pumps
// messages. It exists because a host-side deadlock (a synchronous SendMessageW
// publishing semantic trees from the engine-reader thread) once left the window
// blank and unresponsive immediately after startup, and nothing caught it — the
// process was alive, the engine produced a frame, and only the UI thread was
// wedged. "The process is running" is therefore not a useful health signal;
// "the window answers a message" is.
//
// It drives internal/testapp/semanticsheavy rather than a bundled example: the
// examples are too small to lose that race, and this test was verified to pass
// against the buggy host when driven by the settings example — i.e. it would not
// have caught the very regression it exists for. The fixture publishes a large
// semantic tree during the startup handshake, which does reproduce it.

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procEnumWindows      = user32.NewProc("EnumWindows")
	procGetWindowThread  = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible  = user32.NewProc("IsWindowVisible")
	procSendMessageTimeo = user32.NewProc("SendMessageTimeoutW")
	procIsHungAppWindow  = user32.NewProc("IsHungAppWindow")
)

const (
	wmNull           = 0x0000
	smtoAbortIfHung  = 0x0002
	pumpProbeTimeout = 5000 // ms
)

// hostExecutable finds a built poem_windows_host.exe, preferring an explicit
// override so CI can point at its build output.
func hostExecutable(t *testing.T) string {
	t.Helper()
	if override := os.Getenv("POEM_HOST_EXE"); override != "" {
		if _, err := os.Stat(override); err == nil {
			return override
		}
		t.Fatalf("POEM_HOST_EXE=%s does not exist", override)
	}
	for _, candidate := range []string{
		filepath.Join("..", "..", "cpp_sidecar", "build", "Release", "poem_windows_host.exe"),
		filepath.Join("..", "..", "cpp_sidecar", "build-m9", "Release", "poem_windows_host.exe"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Skip("no poem_windows_host.exe found; build cpp_sidecar or set POEM_HOST_EXE")
	return ""
}

func TestWindowsHostWindowPumpsMessagesAfterStartup(t *testing.T) {
	host := hostExecutable(t)

	// Stage the host beside a freshly built app DLL; the host loads an adjacent
	// poem_app.dll by design.
	dir := t.TempDir()
	build := exec.Command("go", "build", "-buildmode=c-shared",
		"-o", filepath.Join(dir, "poem_app.dll"),
		"github.com/mulavdm/poem/internal/testapp/semanticsheavy")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build app dll: %v\n%s", err, output)
	}
	staged := filepath.Join(dir, "poem_windows_host.exe")
	if err := copyFile(host, staged); err != nil {
		t.Fatalf("stage host: %v", err)
	}

	process := exec.Command(staged)
	process.Dir = dir
	if err := process.Start(); err != nil {
		t.Fatalf("start host: %v", err)
	}
	defer func() {
		_ = process.Process.Kill()
		_, _ = process.Process.Wait()
	}()

	hwnd := waitForVisibleWindow(uint32(process.Process.Pid), 40*time.Second)
	if hwnd == 0 {
		t.Fatal("host never created a visible window within 40s")
	}
	if !windowPumps(hwnd) {
		t.Fatal("window does not pump messages after startup: the UI thread is wedged (see the SendMessageW/PostMessageW deadlock this test guards)")
	}
	if windowHung(hwnd) {
		t.Fatal("window is reported hung by the shell after startup")
	}
}

func copyFile(from, to string) error {
	body, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, body, 0o755)
}

// waitForVisibleWindow polls for a visible top-level window owned by pid.
func waitForVisibleWindow(pid uint32, timeout time.Duration) syscall.Handle {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hwnd := findVisibleWindow(pid); hwnd != 0 {
			return hwnd
		}
		time.Sleep(250 * time.Millisecond)
	}
	return 0
}

func findVisibleWindow(pid uint32) syscall.Handle {
	var found syscall.Handle
	callback := syscall.NewCallback(func(hwnd syscall.Handle, _ uintptr) uintptr {
		var owner uint32
		procGetWindowThread.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&owner)))
		if owner != pid {
			return 1 // keep enumerating
		}
		if visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd)); visible == 0 {
			return 1
		}
		found = hwnd
		return 0 // stop
	})
	procEnumWindows.Call(callback, 0)
	return found
}

// windowPumps reports whether the window's thread services messages: a hung UI
// thread never answers, which is exactly the startup failure being guarded.
func windowPumps(hwnd syscall.Handle) bool {
	var result uintptr
	ret, _, _ := procSendMessageTimeo.Call(uintptr(hwnd), wmNull, 0, 0,
		smtoAbortIfHung, pumpProbeTimeout, uintptr(unsafe.Pointer(&result)))
	return ret != 0
}

func windowHung(hwnd syscall.Handle) bool {
	ret, _, _ := procIsHungAppWindow.Call(uintptr(hwnd))
	return ret != 0
}
