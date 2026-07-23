package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Launch describes how to get the app under test running before the scenario
// drives it. Without this a scenario is only half a fixture: the interactions
// are pinned but "have the right build installed on a booted device with the
// port forwarded" is left to whoever runs it, which is exactly the manual
// preamble a scenario exists to remove.
type Launch struct {
	// Platform is "android" or "windows".
	Platform string `json:"platform"`

	// Exe is the Windows executable to start, relative to the repository root.
	Exe  string            `json:"exe,omitempty"`
	Args []string          `json:"args,omitempty"`
	Env  map[string]string `json:"env,omitempty"`

	// ReuseRunning drives an instance that already answers on base_url rather
	// than starting a second one. Defaults true on Windows: starting a rival
	// process that loses the race for the inspection port is worse than
	// driving the one already there, and killing the user's window uninvited
	// is worse still.
	ReuseRunning *bool `json:"reuse_running,omitempty"`

	// PrepareWindow foregrounds and restores the window once the surface
	// answers. Capture on Windows returns a blank image while the window has
	// never been presented, which reads as a rendering failure rather than as
	// the window-state problem it is.
	PrepareWindow bool `json:"prepare_window,omitempty"`

	// AVD is started when no device is attached. Ignored if one already is,
	// so a physical device or a running emulator wins.
	AVD string `json:"avd,omitempty"`

	Package  string `json:"package"`
	Activity string `json:"activity,omitempty"`

	// APK is installed before launch, relative to the repository root.
	APK string `json:"apk,omitempty"`

	// Build regenerates the APK first. Skipped unless -build is passed, since
	// a rebuild is slow and usually unnecessary between runs.
	Build *BuildAPK `json:"build,omitempty"`

	// ForwardPort maps the device's inspection port to the same port on the
	// host, which is what makes base_url reachable.
	ForwardPort int `json:"forward_port,omitempty"`

	// InspectionProperty is the system property that opts the app into its
	// control surface. NativeActivity has no command line, so a property is
	// the only channel available before the process starts.
	InspectionProperty string `json:"inspection_property,omitempty"`

	BootTimeoutSeconds int `json:"boot_timeout_seconds,omitempty"`
}

// BuildAPK describes how to produce the artifact before installing it. The
// Android fields mirror build_apk.sh's positional arguments; Windows uses Args
// verbatim against a PowerShell script.
type BuildAPK struct {
	Script string   `json:"script"`
	Args   []string `json:"args,omitempty"`
	AppDir string   `json:"app_dir,omitempty"`
	Label  string   `json:"label,omitempty"`
	ABI    string   `json:"abi,omitempty"`
}

func (l *Launch) validate() error {
	switch l.Platform {
	case "android":
		if l.Package == "" {
			return fmt.Errorf("launch needs a package")
		}
		if l.Build != nil {
			if l.Build.Script == "" || l.Build.AppDir == "" || l.Build.Label == "" {
				return fmt.Errorf("launch.build needs script, app_dir and label")
			}
			if l.APK == "" {
				return fmt.Errorf("launch.build needs launch.apk as its output path")
			}
		}
	case "windows":
		if l.Exe == "" {
			return fmt.Errorf("launch needs an exe")
		}
		if l.Build != nil && l.Build.Script == "" {
			return fmt.Errorf("launch.build needs a script")
		}
	default:
		return fmt.Errorf("launch.platform %q is not supported; want android or windows", l.Platform)
	}
	return nil
}

func (l *Launch) reuseRunning() bool {
	if l.ReuseRunning != nil {
		return *l.ReuseRunning
	}
	return l.Platform == "windows"
}

// sdkTool resolves an Android SDK executable without requiring it on PATH,
// because the SDK usually is not.
func sdkTool(subdir, name string) (string, error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var roots []string
	for _, env := range []string{"ANDROID_SDK", "ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := os.Getenv(env); v != "" {
			roots = append(roots, v)
		}
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		roots = append(roots, filepath.Join(local, "Android", "Sdk"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, "Android", "Sdk"), filepath.Join(home, "Library", "Android", "sdk"))
	}
	for _, root := range roots {
		candidate := filepath.Join(root, subdir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath(strings.TrimSuffix(name, ".exe")); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("could not find %s; set ANDROID_SDK or put it on PATH", name)
}

type launcher struct {
	launch  *Launch
	adb     string
	repoDir string
	verbose bool
	// surfaceUp records whether the inspection port already answered before
	// this run started anything.
	surfaceUp bool
}

func (x *launcher) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = x.repoDir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if x.verbose && text != "" {
		fmt.Fprintf(os.Stderr, "    %s\n", firstLine(text))
	}
	if err != nil {
		return text, fmt.Errorf("%s %s: %w: %s", filepath.Base(name), strings.Join(args, " "), err, firstLine(text))
	}
	return text, nil
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func (x *launcher) deviceAttached() bool {
	out, err := x.run(x.adb, "devices")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n")[1:] {
		if strings.HasSuffix(strings.TrimSpace(line), "\tdevice") {
			return true
		}
	}
	return false
}

// ensureDevice boots the configured AVD when nothing is attached, then waits
// for the boot to complete. A device that answers adb but has not finished
// booting will reject `am start`, so waiting for sys.boot_completed is not
// optional.
func (x *launcher) ensureDevice() error {
	if x.deviceAttached() {
		if x.verbose {
			fmt.Fprintln(os.Stderr, "  device already attached")
		}
		return nil
	}
	if x.launch.AVD == "" {
		return fmt.Errorf("no device attached and no launch.avd configured")
	}

	emulator, err := sdkTool("emulator", "emulator")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "  booting AVD %s\n", x.launch.AVD)
	cmd := exec.Command(emulator, "-avd", x.launch.AVD, "-no-boot-anim")
	cmd.Dir = x.repoDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting emulator: %w", err)
	}
	// Deliberately not waited on: the emulator outlives this run, so the next
	// scenario starts against a warm device.
	go func() { _ = cmd.Wait() }()

	timeout := time.Duration(x.launch.BootTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if x.deviceAttached() {
			if out, err := x.run(x.adb, "shell", "getprop", "sys.boot_completed"); err == nil && strings.TrimSpace(out) == "1" {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("AVD %s did not finish booting within %s", x.launch.AVD, timeout)
}

func (x *launcher) buildAPK() error {
	b := x.launch.Build
	abi := b.ABI
	if abi == "" {
		abi = "x86_64"
	}
	// The build script is bash; on Windows that is Git Bash, which is present
	// wherever the Go toolchain and NDK workflow already are.
	bash, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("launch.build needs bash on PATH: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  building %s (%s)\n", filepath.Base(x.launch.APK), abi)
	// build_apk.sh resolves its app-directory argument against the working
	// directory, and its documented usage runs it from its own folder. Running
	// it from the repository root instead resolves app_dir somewhere else
	// entirely, so the working directory is pinned to the script.
	cmd := exec.Command(bash, "./"+filepath.Base(b.Script),
		b.AppDir, x.launch.Package, b.Label, filepath.Base(x.launch.APK), abi)
	cmd.Dir = filepath.Join(x.repoDir, filepath.FromSlash(filepath.Dir(b.Script)))
	out, err := cmd.CombinedOutput()
	if err != nil {
		// The build log is the only useful diagnostic here, so it is passed
		// through whole rather than reduced to its first line.
		return fmt.Errorf("%s failed: %w\n%s", b.Script, err, strings.TrimSpace(string(out)))
	}
	if x.verbose {
		fmt.Fprintf(os.Stderr, "    %s\n", firstLine(strings.TrimSpace(string(out))))
	}
	return nil
}

// buildWindows runs a PowerShell packaging script.
func (x *launcher) buildWindows() error {
	b := x.launch.Build
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell")
		if err != nil {
			return fmt.Errorf("launch.build needs pwsh or powershell on PATH: %w", err)
		}
	}
	fmt.Fprintf(os.Stderr, "  building %s\n", filepath.Base(b.Script))
	// -ExecutionPolicy Bypass applies to this child process only; it does not
	// change any machine or user policy. Without it a default Windows install
	// refuses to run the repository's own packaging script.
	args := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.FromSlash(b.Script)}, b.Args...)
	cmd := exec.Command(shell, args...)
	cmd.Dir = x.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\n%s", b.Script, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// prepareWindows starts the desktop app unless one is already answering.
func (x *launcher) prepareWindows(forceBuild bool, surfaceUp bool) error {
	if surfaceUp && x.launch.reuseRunning() {
		fmt.Fprintln(os.Stderr, "  reusing the instance already on the inspection port")
		return nil
	}
	if surfaceUp {
		return fmt.Errorf("something is already serving the inspection port; stop it or set reuse_running")
	}

	exePath := filepath.Join(x.repoDir, filepath.FromSlash(x.launch.Exe))
	// Absolute, because Windows resolves a relative executable path against the
	// calling process's working directory rather than cmd.Dir.
	if abs, err := filepath.Abs(exePath); err == nil {
		exePath = abs
	}
	if _, err := os.Stat(exePath); err != nil || forceBuild {
		if x.launch.Build == nil {
			return fmt.Errorf("%s is missing and launch.build is not configured", x.launch.Exe)
		}
		if err := x.buildWindows(); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "  launching %s\n", filepath.Base(exePath))
	cmd := exec.Command(exePath, x.launch.Args...)
	cmd.Dir = filepath.Dir(exePath)
	if len(x.launch.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range x.launch.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", x.launch.Exe, err)
	}
	// The app outlives this run, as an emulator does, so the next scenario
	// starts against a warm process.
	go func() { _ = cmd.Wait() }()
	return nil
}

// Prepare brings the device and app to the state the scenario assumes.
func (x *launcher) Prepare(forceBuild bool) error {
	if x.launch.Platform == "windows" {
		return x.prepareWindows(forceBuild, x.surfaceUp)
	}
	if err := x.ensureDevice(); err != nil {
		return err
	}

	if x.launch.Build != nil {
		apkPath := filepath.Join(x.repoDir, filepath.FromSlash(x.launch.APK))
		_, statErr := os.Stat(apkPath)
		if forceBuild || statErr != nil {
			if err := x.buildAPK(); err != nil {
				return err
			}
		} else if x.verbose {
			fmt.Fprintln(os.Stderr, "  reusing existing APK (pass -build to rebuild)")
		}
	}

	if x.launch.APK != "" {
		fmt.Fprintf(os.Stderr, "  installing %s\n", filepath.Base(x.launch.APK))
		if _, err := x.run(x.adb, "install", "-r", filepath.FromSlash(x.launch.APK)); err != nil {
			return err
		}
	}

	// The property must be set before the process starts: pkg/mobile reads it
	// at package init and translates it into the POEM_INSPECTION environment
	// contract, so setting it on a running app has no effect.
	if prop := x.launch.InspectionProperty; prop != "" {
		if _, err := x.run(x.adb, "shell", "setprop", prop, "1"); err != nil {
			return err
		}
	}

	if _, err := x.run(x.adb, "shell", "am", "force-stop", x.launch.Package); err != nil {
		return err
	}
	activity := x.launch.Activity
	if activity == "" {
		activity = "android.app.NativeActivity"
	}
	fmt.Fprintf(os.Stderr, "  launching %s\n", x.launch.Package)
	if _, err := x.run(x.adb, "shell", "am", "start", "-n", x.launch.Package+"/"+activity); err != nil {
		return err
	}

	if port := x.launch.ForwardPort; port > 0 {
		spec := fmt.Sprintf("tcp:%d", port)
		if _, err := x.run(x.adb, "forward", spec, spec); err != nil {
			return err
		}
		if x.verbose {
			fmt.Fprintf(os.Stderr, "  forwarded %s\n", spec)
		}
	}
	return nil
}

// newLauncher resolves the tools a launch needs. repoDir anchors the relative
// paths in the scenario so the command works from any working directory.
func newLauncher(l *Launch, repoDir string, verbose bool) (*launcher, error) {
	if err := l.validate(); err != nil {
		return nil, err
	}
	x := &launcher{launch: l, repoDir: repoDir, verbose: verbose}
	if l.Platform == "android" {
		adb, err := sdkTool("platform-tools", "adb")
		if err != nil {
			return nil, err
		}
		x.adb = adb
	}
	return x, nil
}
