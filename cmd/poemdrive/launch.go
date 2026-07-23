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
	// Platform is currently "android". Windows apps are launched by their own
	// packaging script today; the field exists so that can move here later.
	Platform string `json:"platform"`

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

// BuildAPK mirrors android_engine/build_apk.sh's positional arguments.
type BuildAPK struct {
	Script string `json:"script"`
	AppDir string `json:"app_dir"`
	Label  string `json:"label"`
	ABI    string `json:"abi,omitempty"`
}

func (l *Launch) validate() error {
	if l.Platform != "android" {
		return fmt.Errorf("launch.platform %q is not supported; want android", l.Platform)
	}
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
	return nil
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

// Prepare brings the device and app to the state the scenario assumes.
func (x *launcher) Prepare(forceBuild bool) error {
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
	adb, err := sdkTool("platform-tools", "adb")
	if err != nil {
		return nil, err
	}
	return &launcher{launch: l, adb: adb, repoDir: repoDir, verbose: verbose}, nil
}
