// Command poemdrive replays a scenario against a running POEM application
// through its automation surface, then gates the resulting frame timings
// against explicit budgets and a recorded baseline.
//
// It works against any host that exposes the surface — the Windows gallery,
// MAPPS, or an Android device reached through `adb forward` — because the
// scenario file supplies the base URL and the interactions, and nothing about
// the app is compiled in.
//
// Why a file rather than a script: the scene becomes a fixture. A gate that
// cannot replay the exact interaction it measured is comparing two different
// things and reporting the difference as a regression. See
// docs/M0_performance_baselines.md, which recorded MAPPS runs varying by ~50%
// precisely because the interaction was not pinned.
//
// Usage:
//
//	poemdrive [flags] scenes/gallery-tabs.json
//
// Exit codes: 0 all gates passed, 1 a gate failed, 2 the run could not complete.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mulavdm/poem/pkg/render"
)

type driver struct {
	scenario            *Scenario
	client              *http.Client
	verbose             bool
	captureDir          string
	captures            int
	captureAfterFrame   uint64
	captureFramePending bool
}

func (d *driver) url(path string) string {
	return strings.TrimRight(d.scenario.BaseURL, "/") + path
}

func (d *driver) post(path string, body any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(http.MethodPost, d.url(path), reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode >= 400 {
		var failure struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &failure) == nil && strings.TrimSpace(failure.Error) != "" {
			return fmt.Errorf("POST %s: %s: %s", path, resp.Status, failure.Error)
		}
		return fmt.Errorf("POST %s: %s: %s", path, resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (d *driver) getJSON(path string, dst any) error {
	resp, err := d.client.Get(d.url(path))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.Unmarshal(raw, dst)
}

// waitReady blocks until the automation surface answers. An app under a
// debugger or a cold Android start can take several seconds, and failing
// instantly would make the tool useless in exactly those cases.
func (d *driver) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		// Answering /state is not enough: a freshly launched app serves it
		// before the first frame exists, and the component tree is empty until
		// then. Readiness means "there is something to drive".
		var tree struct {
			Flat []struct {
				ID string `json:"id"`
			} `json:"flat"`
		}
		if err := d.getJSON("/components", &tree); err != nil {
			lastErr = err
		} else if len(tree.Flat) > 0 {
			return nil
		} else {
			lastErr = fmt.Errorf("component tree is still empty")
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("automation surface at %s never became ready: %w", d.scenario.BaseURL, lastErr)
}

func (d *driver) step(s Step) error {
	switch s.Action {
	case actionClick:
		d.rememberFrameBeforeAction()
		return d.post("/click", map[string]string{"command": "click", "id": s.ID})
	case actionFocus:
		d.rememberFrameBeforeAction()
		return d.post("/focus", map[string]string{"command": "focus", "id": s.ID})
	case actionSetText:
		d.rememberFrameBeforeAction()
		return d.post("/set-text", map[string]string{"command": "set-text", "id": s.ID, "value": s.Value})
	case actionPressKey:
		d.rememberFrameBeforeAction()
		return d.post("/press-key", map[string]string{"command": "press-key", "key": s.Key})
	case actionWheel:
		d.rememberFrameBeforeAction()
		return d.post("/wheel", map[string]any{"id": s.ID, "delta": s.Delta})
	case actionPan:
		d.rememberFrameBeforeAction()
		return d.post("/pan", map[string]any{"id": s.ID, "delta_x": s.DeltaX, "delta_y": s.DeltaY, "phase": s.Phase})
	case actionPinch:
		d.rememberFrameBeforeAction()
		return d.post("/pinch", map[string]any{"id": s.ID, "delta_x": s.DeltaX, "delta_y": s.DeltaY, "scale": s.Scale, "phase": s.Phase})
	case actionDrag:
		d.rememberFrameBeforeAction()
		return d.post("/pointer-drag", map[string]any{"id": s.ID, "delta_x": s.DeltaX, "delta_y": s.DeltaY, "button": s.Button, "phase": s.Phase})
	case actionAssertValue:
		return d.assertComponentValue(s.ID, s.Value)
	case actionResize:
		d.rememberFrameBeforeAction()
		return d.post("/resize", map[string]any{"width": s.Width, "height": s.Height})
	case actionAssertExists:
		return d.assertComponentExists(s.ID)
	case actionAssertAbsent:
		return d.assertComponentAbsent(s.ID)
	case actionAssertCount:
		return d.assertComponentCount(s.IDPrefix, s.Count)
	case actionAssertNoOverlap:
		return d.assertNoOverlap(s.IDs)
	case actionWait:
		time.Sleep(time.Duration(s.MS) * time.Millisecond)
		return nil
	case actionCapture:
		return d.capture(s)
	}
	return fmt.Errorf("unknown action %q", s.Action)
}

func (d *driver) assertComponentValue(id, contains string) error {
	var tree struct {
		Flat []struct {
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"flat"`
	}
	if err := d.getJSON("/components", &tree); err != nil {
		return err
	}
	for _, node := range tree.Flat {
		if node.ID != id {
			continue
		}
		if strings.Contains(node.Value, contains) {
			return nil
		}
		return fmt.Errorf("component %q value %q does not contain %q", id, node.Value, contains)
	}
	return fmt.Errorf("component %q was not present while asserting value", id)
}

func (d *driver) componentFlatList() ([]struct {
	ID     string `json:"id"`
	Bounds struct {
		X int `json:"x"`
		Y int `json:"y"`
		W int `json:"w"`
		H int `json:"h"`
	} `json:"bounds"`
}, error) {
	var tree struct {
		Flat []struct {
			ID     string `json:"id"`
			Bounds struct {
				X int `json:"x"`
				Y int `json:"y"`
				W int `json:"w"`
				H int `json:"h"`
			} `json:"bounds"`
		} `json:"flat"`
	}
	if err := d.getJSON("/components", &tree); err != nil {
		return nil, err
	}
	return tree.Flat, nil
}

func (d *driver) assertComponentExists(id string) error {
	flat, err := d.componentFlatList()
	if err != nil {
		return err
	}
	for _, node := range flat {
		if node.ID == id {
			return nil
		}
	}
	return fmt.Errorf("component %q was not present, expected it to exist", id)
}

func (d *driver) assertComponentAbsent(id string) error {
	flat, err := d.componentFlatList()
	if err != nil {
		return err
	}
	for _, node := range flat {
		if node.ID == id {
			return fmt.Errorf("component %q was present, expected it to be absent", id)
		}
	}
	return nil
}

func (d *driver) assertComponentCount(prefix string, want int) error {
	flat, err := d.componentFlatList()
	if err != nil {
		return err
	}
	got := 0
	for _, node := range flat {
		if strings.HasPrefix(node.ID, prefix) {
			got++
		}
	}
	if got != want {
		return fmt.Errorf("expected %d component(s) with id prefix %q, found %d", want, prefix, got)
	}
	return nil
}

// assertNoOverlap checks that no two of the named components' bounding
// rectangles intersect -- useful for the class of bug where a fixed-size
// layout slot overflows into its neighbor (button rows, wrapped text) only
// under specific content, which a screenshot catches but a plain
// assert-value on either component's own text cannot.
func (d *driver) assertNoOverlap(ids []string) error {
	flat, err := d.componentFlatList()
	if err != nil {
		return err
	}
	type rect struct {
		id             string
		x0, y0, x1, y1 int
	}
	rects := make([]rect, 0, len(ids))
	for _, want := range ids {
		found := false
		for _, node := range flat {
			if node.ID == want {
				rects = append(rects, rect{
					id: want,
					x0: node.Bounds.X, y0: node.Bounds.Y,
					x1: node.Bounds.X + node.Bounds.W, y1: node.Bounds.Y + node.Bounds.H,
				})
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("component %q was not present while checking overlap", want)
		}
	}
	for i := 0; i < len(rects); i++ {
		for j := i + 1; j < len(rects); j++ {
			a, b := rects[i], rects[j]
			if a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1 {
				return fmt.Errorf("components %q and %q overlap", a.id, b.id)
			}
		}
	}
	return nil
}

// capture asks the host for an image and writes it beside the others. The
// endpoint returns PNG bytes already encoded, and every host that can capture
// answers the same request — that is the point of routing it through the
// automation surface rather than making each caller reach for a
// platform-specific tool.
func (d *driver) capture(s Step) error {
	source := s.Source
	if source == "" {
		source = "native"
	}
	// A click returns as soon as the engine accepts it, not when the resulting
	// frame has been presented, so an immediate readback grabs the previous
	// frame. Waiting for the frame counter to advance is exact where a fixed
	// sleep is a guess.
	if d.captureFramePending {
		if err := d.awaitFrameAfter(d.captureAfterFrame, 600*time.Millisecond); err != nil && d.verbose {
			fmt.Fprintln(os.Stderr, "  capture:", err)
		}
		d.captureFramePending = false
	}
	resp, err := d.client.Get(d.url(captureSources[source]))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		// The host explains why it could not capture; surfacing that verbatim
		// beats writing a blank image that reads as a rendering failure.
		return fmt.Errorf("capture %q from %s: %s: %s", s.Name, source, resp.Status, strings.TrimSpace(string(body)))
	}
	path := filepath.Join(d.captureDir, fmt.Sprintf("%s-%s.png", d.scenario.Name, s.Name))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	d.captures++
	if d.verbose {
		fmt.Fprintf(os.Stderr, "  captured %s (%d bytes)\n", path, len(body))
	}
	return nil
}

// verifyTargets checks that every component a step names actually exists
// before anything is driven. Without it a wrong id — or, worse, the right id
// against the wrong app, which happens when a stale `adb forward` still owns
// the port — surfaces only as `400 Bad Request` from the middle of a warmup.
func (d *driver) verifyTargets() error {
	var tree struct {
		Flat []struct {
			ID string `json:"id"`
		} `json:"flat"`
	}
	if err := d.getJSON("/components", &tree); err != nil {
		return fmt.Errorf("reading the component tree: %w", err)
	}
	present := make(map[string]bool, len(tree.Flat))
	for _, node := range tree.Flat {
		present[node.ID] = true
	}

	var missing []string
	seen := map[string]bool{}
	for _, step := range d.scenario.Steps {
		if step.ID == "" || step.Dynamic || seen[step.ID] || present[step.ID] {
			continue
		}
		seen[step.ID] = true
		missing = append(missing, step.ID)
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("the app at %s does not have %v.\n"+
		"  Either the ids are wrong, or this is a different app than the scenario expects —\n"+
		"  a stale `adb forward` or another instance can leave something else on the port.\n"+
		"  GET %s/components lists what is actually there",
		d.scenario.BaseURL, missing, d.scenario.BaseURL)
}

// awaitNewFrame blocks until the engine reports having rendered another frame,
// so a capture shows the state the preceding steps produced.
func (d *driver) currentFrameCount() (uint64, error) {
	var perf render.PerfState
	if err := d.getJSON("/perf/state", &perf); err != nil {
		return 0, err
	}
	return perf.Frames.FrameCount, nil
}

func (d *driver) rememberFrameBeforeAction() {
	if frame, err := d.currentFrameCount(); err == nil {
		d.captureAfterFrame = frame
		d.captureFramePending = true
	}
}

func (d *driver) awaitFrameAfter(start uint64, timeout time.Duration) error {
	now, err := d.currentFrameCount()
	if err != nil {
		return err
	}
	if now > start {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		if now, err := d.currentFrameCount(); err == nil && now > start {
			return nil
		}
	}
	return fmt.Errorf("no new frame within %s; the capture may show the previous one", timeout)
}

// run executes warmup, resets the counters, then replays the step list.
// Resetting after warmup is what keeps first-paint and lazily built caches out
// of the sample.
func (d *driver) run() (Measurement, error) {
	for i := 0; i < d.scenario.Warmup; i++ {
		step := d.scenario.Steps[i%len(d.scenario.Steps)]
		if step.Action == actionCapture {
			continue // warmup images are noise; capture only the measured pass
		}
		if err := d.step(step); err != nil {
			return Measurement{}, fmt.Errorf("warmup: %w", err)
		}
	}
	if err := d.post("/perf/reset", nil); err != nil {
		return Measurement{}, fmt.Errorf("reset: %w", err)
	}

	// Captures fire only on the final pass through the step list. Repeating
	// them every cycle would rewrite the same filenames and, worse, charge
	// every cycle for a full readback and PNG encode — perturbing the very
	// timings the run exists to measure.
	lastCycle := d.scenario.Iterations - len(d.scenario.Steps)
	pace := time.Duration(*d.scenario.PaceMS) * time.Millisecond
	for i := 0; i < d.scenario.Iterations; i++ {
		stepStart := time.Now()
		step := d.scenario.Steps[i%len(d.scenario.Steps)]
		if step.Action == actionCapture && i < lastCycle {
			continue
		}
		if err := d.step(step); err != nil {
			return Measurement{}, fmt.Errorf("step %d: %w", i, err)
		}
		if d.verbose && (i+1)%100 == 0 {
			fmt.Fprintf(os.Stderr, "  %d/%d\n", i+1, d.scenario.Iterations)
		}
		if remaining := pace - time.Since(stepStart); remaining > 0 {
			time.Sleep(remaining)
		}
	}
	if d.scenario.SettleMS > 0 {
		time.Sleep(time.Duration(d.scenario.SettleMS) * time.Millisecond)
	}

	var perf render.PerfState
	if err := d.getJSON("/perf/state", &perf); err != nil {
		return Measurement{}, fmt.Errorf("perf/state: %w", err)
	}

	// A host without a native debug channel (web, test drivers) is a normal
	// case, not an error — but it is recorded, so a budget naming a native
	// metric fails loudly instead of passing on absent data.
	var native render.NativePerfState
	nativeOK := d.getJSON("/perf/native", &native) == nil

	measurement := collect(perf, native, nativeOK)
	measurement.Scenario = d.scenario.Name
	measurement.Captured = time.Now().UTC().Format(time.RFC3339)
	if measurement.Frames == 0 && !d.scenario.Functional {
		return measurement, fmt.Errorf("no frames were recorded; the steps may not have changed anything")
	}
	return measurement, nil
}

func printTable(m Measurement, scenario *Scenario) {
	fmt.Printf("%s — %d frames sampled\n", m.Scenario, m.Frames)
	if !m.NativeAvailable {
		fmt.Println("  (host exposes no native debug channel; native.* metrics unavailable)")
	}
	names := make([]string, 0, len(m.Metrics))
	for name := range m.Metrics {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("  %-22s %7s %8s %8s %8s %8s\n", "metric", "mean", "p50", "p95", "p99", "max")
	for _, name := range names {
		s := m.Metrics[name]
		marker := ""
		if budget, ok := scenario.Budgets[name+".p95"]; ok && s.P95 > budget {
			marker = "  <- over p95 budget"
		}
		fmt.Printf("  %-22s %7.3f %8.3f %8.3f %8.3f %8.3f%s\n",
			name, s.Mean, s.P50, s.P95, s.P99, s.Max, marker)
	}
}

func main() {
	baselinePath := flag.String("baseline", "", "compare against this recorded baseline and fail on regression")
	savePath := flag.String("save-baseline", "", "write this run's measurement as a baseline")
	asJSON := flag.Bool("json", false, "emit the measurement as JSON instead of a table")
	verbose := flag.Bool("v", false, "report progress while driving")
	captureDir := flag.String("capture-dir", "", "directory for images written by capture steps")
	noLaunch := flag.Bool("no-launch", false, "skip the scenario's launch block; drive whatever is already running")
	forceBuild := flag.Bool("build", false, "rebuild the app before installing it")
	teardown := flag.Bool("teardown", false, "stop what this run started when it finishes; omit to leave the app or device up for the next scenario")
	repoDir := flag.String("repo", ".", "repository root that the scenario's relative paths resolve against")
	timeout := flag.Duration("ready-timeout", 30*time.Second, "how long to wait for the automation surface")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: poemdrive [flags] <scenario.json>\n\n"+
			"Replays a scenario against a running POEM app and gates its frame timings.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	scenario, err := LoadScenario(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "poemdrive:", err)
		os.Exit(2)
	}

	if scenario.HasCaptures() && *captureDir == "" {
		fmt.Fprintf(os.Stderr, "poemdrive: %s has capture steps; pass -capture-dir\n", scenario.Name)
		os.Exit(2)
	}
	if *captureDir != "" {
		if err := os.MkdirAll(*captureDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "poemdrive:", err)
			os.Exit(2)
		}
	}

	probe := &driver{scenario: scenario, client: &http.Client{Timeout: 3 * time.Second}}
	surfaceUp := probe.waitReady(time.Second) == nil

	var launched *launcher
	if scenario.Launch != nil && !*noLaunch {
		l, err := newLauncher(scenario.Launch, *repoDir, *verbose)
		if err != nil {
			fmt.Fprintln(os.Stderr, "poemdrive:", err)
			os.Exit(2)
		}
		l.surfaceUp = surfaceUp
		fmt.Fprintf(os.Stderr, "preparing %s target\n", scenario.Launch.Platform)
		if err := l.Prepare(*forceBuild); err != nil {
			fmt.Fprintln(os.Stderr, "poemdrive: launch:", err)
			os.Exit(2)
		}
		launched = l
		// Teardown runs on every exit path after this point, including the
		// failure paths: a scenario that dies half way must not strand an
		// emulator it booted.
		if *teardown {
			defer launched.Teardown()
		}
	}

	d := &driver{
		scenario:   scenario,
		client:     &http.Client{Timeout: 30 * time.Second},
		verbose:    *verbose,
		captureDir: *captureDir,
	}
	if err := d.waitReady(*timeout); err != nil {
		fmt.Fprintln(os.Stderr, "poemdrive:", err)
		os.Exit(2)
	}

	// Windows capture returns a blank image while the window has never been
	// presented — a window-state problem that reads as a rendering failure.
	// Only bring the window to the foreground when the scenario actually
	// captures the desktop; restoring and clamping it on screen is enough
	// for every other capture source, and doing more than that is an
	// intrusive alt-tab for no benefit -- especially across a batch of
	// scenarios run back to back.
	if scenario.Launch != nil && scenario.Launch.PrepareWindow && !*noLaunch {
		prepareBody := map[string]bool{
			"restore_window":      true,
			"clamp_to_work_area":  true,
			"bring_to_foreground": scenario.NeedsForegroundWindow(),
		}
		if err := d.post("/prepare-window", prepareBody); err != nil && *verbose {
			fmt.Fprintln(os.Stderr, "  prepare-window:", err)
		}
		// Foregrounding round-trips through the native debug channel and the
		// surface can stop answering while that is in flight, so readiness is
		// re-established rather than assumed.
		if err := d.waitReady(*timeout); err != nil {
			fmt.Fprintln(os.Stderr, "poemdrive: after prepare-window:", err)
			os.Exit(2)
		}
	}

	if err := d.verifyTargets(); err != nil {
		fmt.Fprintln(os.Stderr, "poemdrive:", err)
		fail(launched, *teardown, 2)
	}

	measurement, err := d.run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "poemdrive:", err)
		fail(launched, *teardown, 2)
	}

	if *savePath != "" {
		encoded, _ := json.MarshalIndent(measurement, "", "  ")
		if err := os.WriteFile(*savePath, append(encoded, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "poemdrive:", err)
			os.Exit(2)
		}
	}

	failures := CheckBudgets(measurement, scenario.Budgets)
	if *baselinePath != "" {
		raw, err := os.ReadFile(*baselinePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "poemdrive:", err)
			os.Exit(2)
		}
		var baseline Measurement
		if err := json.Unmarshal(raw, &baseline); err != nil {
			fmt.Fprintf(os.Stderr, "poemdrive: %s: %v\n", *baselinePath, err)
			os.Exit(2)
		}
		failures = append(failures, CheckRegression(measurement, baseline, scenario.RegressionTolerancePct, scenario.RegressionFloorMS)...)
	}

	if *asJSON {
		payload := struct {
			Measurement
			Failures []Failure `json:"failures,omitempty"`
		}{Measurement: measurement, Failures: failures}
		encoded, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(encoded))
	} else {
		printTable(measurement, scenario)
		if d.captures > 0 {
			fmt.Printf("\n%d image(s) written to %s\n", d.captures, d.captureDir)
		}
		if len(failures) == 0 {
			fmt.Println("\nall gates passed")
		} else {
			fmt.Printf("\n%d gate failure(s):\n", len(failures))
			for _, f := range failures {
				fmt.Printf("  FAIL %s — %s\n", f.Key, f.Reason)
			}
		}
	}

	if len(failures) > 0 {
		fail(launched, *teardown, 1)
	}
	// The success path returns normally, so the deferred teardown runs. Calling
	// it here as well would tear down twice.
}

// fail exits with code after tearing down, because os.Exit skips deferred
// calls and a scenario that dies must not strand an emulator it booted.
func fail(l *launcher, teardown bool, code int) {
	if teardown && l != nil {
		l.Teardown()
	}
	os.Exit(code)
}
