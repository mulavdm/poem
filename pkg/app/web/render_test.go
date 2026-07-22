package web

import (
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/app"
)

func TestRenderTextAreaEmitsTextareaControl(t *testing.T) {
	node := app.TextArea("draft body", "write here", app.Msg{Name: "edit"})
	html := string(renderNode(node, rootPath))
	if !strings.Contains(html, "<textarea") {
		t.Fatalf("TextArea did not render a <textarea>: %s", html)
	}
	if !strings.Contains(html, `name="`+fieldPrefix+rootPath+`"`) {
		t.Fatalf("TextArea field not namespaced by path: %s", html)
	}
	if !strings.Contains(html, "draft body") {
		t.Fatalf("TextArea value missing: %s", html)
	}
}

func TestRenderSliderEmitsRangeControl(t *testing.T) {
	node := app.Slider(0, 10, 2.5, app.Msg{Name: "volume"})
	node.Step = 0.5
	html := string(renderNode(node, rootPath))
	for _, wanted := range []string{`type="range"`, `min="0"`, `max="10"`, `step="0.5"`, `value="2.5"`, `name="` + fieldPrefix + rootPath + `"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("Slider missing %q: %s", wanted, html)
		}
	}
}

func TestActionSubmitValueRoundTripsPayloadAndProtectsPrefixNames(t *testing.T) {
	for _, message := range []app.Msg{{Name: "panel", Payload: "settings"}, {Name: actionPrefix + "literal"}} {
		encoded := actionSubmitValue(message)
		decoded, ok := submittedAction(encoded)
		if !ok || decoded.Name != message.Name || decoded.Payload != message.Payload {
			t.Fatalf("round trip %+v => %q => %+v, %v", message, encoded, decoded, ok)
		}
	}
	if _, ok := submittedAction(actionPrefix + "malformed"); ok {
		t.Fatal("accepted malformed encoded action")
	}
}

func TestSelectedActionUsesPressedSelectionSemantics(t *testing.T) {
	node := app.ActionNode{Semantic: app.Semantic{Enabled: true}, Label: "From", Selected: true, Importance: app.ImportanceSecondary, Invoke: app.Msg{Name: "endpoint"}}
	html := string(renderNode(node, rootPath))
	if !strings.Contains(html, `aria-pressed="true"`) || strings.Contains(html, "poem-button--primary") {
		t.Fatalf("selected action did not remain a non-primary selection: %s", html)
	}
}

func TestResponsiveRendersCompactBaselineAndCollectsBothBranches(t *testing.T) {
	node := app.Responsive(700,
		[]app.Node{app.Button("Compact", app.Msg{Name: "compact"})},
		[]app.Node{app.Button("Wide", app.Msg{Name: "wide"})},
	)
	html := string(renderNode(node, rootPath))
	for _, wanted := range []string{`data-poem-responsive`, `data-breakpoint="700"`, `data-poem-compact`, `data-poem-wide hidden disabled`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("Responsive missing %q: %s", wanted, html)
		}
	}
	allowed := map[string]bool{}
	collectMessages(node, allowed)
	if !allowed["compact"] || !allowed["wide"] {
		t.Fatalf("responsive messages = %#v", allowed)
	}
}

// TestFloatPayloadRoundTrips proves the FloatPayload/Msg.Float convention is a
// clean round trip across the string transport, including a fractional value.
func TestFloatPayloadRoundTrips(t *testing.T) {
	for _, v := range []float64{0, 2.5, 7.25, 100} {
		if got := (app.Msg{Payload: app.FloatPayload(v)}).Float(); got != v {
			t.Fatalf("FloatPayload(%v).Float() = %v", v, got)
		}
	}
	if got := (app.Msg{Payload: "not-a-number"}).Float(); got != 0 {
		t.Fatalf("non-numeric payload decoded to %v, want 0", got)
	}
}

func TestRenderBadgeCarriesVariantClass(t *testing.T) {
	html := string(renderNode(app.Badge("Live", app.VariantSuccess), rootPath))
	if !strings.Contains(html, "poem-badge") {
		t.Fatalf("Badge did not render GopherWeb badge markup: %s", html)
	}
	if !strings.Contains(html, "success") {
		t.Fatalf("Badge variant class missing: %s", html)
	}
	if !strings.Contains(html, "Live") {
		t.Fatalf("Badge text missing: %s", html)
	}
}

func TestWorkspaceRendersSemanticLandmarks(t *testing.T) {
	node := app.WorkspaceNode{Semantic: app.Semantic{ID: "work", Name: "Workspace", Enabled: true}, Title: "MAPPS", Subtitle: "Private", Content: []app.Node{app.Text("Map")}, Tools: []app.Node{app.SectionNode{Semantic: app.Semantic{ID: "plan", Name: "Planner", Enabled: true}, Title: "Plan", Children: []app.Node{app.Text("Search")}}}, Detail: []app.Node{app.SectionNode{Semantic: app.Semantic{ID: "detail", Name: "Route details", Enabled: true}, Title: "Route details", Children: []app.Node{app.Text("12 minutes")}}}}
	html := string(renderNode(node, rootPath))
	for _, wanted := range []string{`class="poem-workspace"`, `<main class="poem-workspace__content">`, `<aside class="poem-workspace__tools"`, `<aside class="poem-workspace__detail"`, `<h1>MAPPS</h1>`, `Route details`, `class="poem-section"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("workspace markup missing %q: %s", wanted, html)
		}
	}
}

func TestContainerUsesTokenClassesWithoutInlineStyle(t *testing.T) {
	html := string(renderNode(app.Container(app.Vertical, 12, app.Text("Content")), rootPath))
	if strings.Contains(html, `style=`) {
		t.Fatalf("container emitted CSP-blocked inline style: %s", html)
	}
	for _, wanted := range []string{"trellis-gap-3", "trellis-padding-0"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("container missing %q: %s", wanted, html)
		}
	}
}

func TestViewportScriptHasResizeFallbackAndBucketAlignedThreshold(t *testing.T) {
	script := string(imageViewportScript)
	for _, wanted := range []string{`addEventListener("resize", scheduleResizeCommit`, `Math.abs(rect.width - renderedWidth) > 127`, `setTimeout(scheduleResizeCommit, 0)`} {
		if !strings.Contains(script, wanted) {
			t.Fatalf("viewport script missing %q", wanted)
		}
	}
}

func TestRenderSwitchEmitsToggleControl(t *testing.T) {
	html := string(renderNode(app.Switch("Wi-Fi", true, app.Msg{Name: "wifi"}), rootPath))
	for _, wanted := range []string{"poem-switch", `role="switch"`, "checked", "Wi-Fi"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("Switch missing %q: %s", wanted, html)
		}
	}
	if !strings.Contains(html, `name="`+fieldPrefix+rootPath+`"`) {
		t.Fatalf("Switch field not namespaced by path: %s", html)
	}
}

func TestRenderRadioGroupSharesNameAndMarksSelection(t *testing.T) {
	options := []app.Option{
		{Label: "Free", Value: "free"},
		{Label: "Pro", Value: "pro"},
	}
	html := string(renderNode(app.RadioGroup(options, "pro", app.Msg{Name: "plan"}), rootPath))
	for _, wanted := range []string{"poem-radiogroup", `type="radio"`, `name="` + fieldPrefix + rootPath + `"`, `value="free"`, `value="pro"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("RadioGroup missing %q: %s", wanted, html)
		}
	}
	if strings.Count(html, "checked") != 1 {
		t.Fatalf("expected exactly one checked radio: %s", html)
	}
}

// TestRadioGroupFieldRoundTrips proves a RadioGroupNode collects as a single
// valueField under one shared name — the same posting mechanics as SelectNode.
func TestRadioGroupFieldRoundTrips(t *testing.T) {
	view := app.Container(app.Vertical, 0,
		app.RadioGroup([]app.Option{{Label: "Free", Value: "free"}}, "free", app.Msg{Name: "plan"}),
	)
	fields := map[string]postedField{}
	collectFields(view, rootPath, fields)
	field, ok := fields[fieldPrefix+childPath(rootPath, 0)]
	if !ok {
		t.Fatalf("radio group field not collected: %+v", fields)
	}
	if field.kind != valueField || field.msg.Name != "plan" {
		t.Fatalf("radio group field = %+v, want valueField dispatching plan", field)
	}
}

// TestSwitchFieldReadsAsCheckbox proves a SwitchNode collects as a
// checkboxField — presence-is-the-value, the same posting mechanics as a
// CheckboxNode — so an unchecked toggle absent from the POST still updates.
func TestSwitchFieldReadsAsCheckbox(t *testing.T) {
	view := app.Container(app.Vertical, 0,
		app.Switch("Wi-Fi", false, app.Msg{Name: "wifi"}),
	)
	fields := map[string]postedField{}
	collectFields(view, rootPath, fields)
	field, ok := fields[fieldPrefix+childPath(rootPath, 0)]
	if !ok {
		t.Fatalf("switch field not collected: %+v", fields)
	}
	if field.kind != checkboxField || field.msg.Name != "wifi" {
		t.Fatalf("switch field = %+v, want checkboxField dispatching wifi", field)
	}
}

func TestRenderProgressBarEmitsProgressElement(t *testing.T) {
	node := app.ProgressBar(40, 100)
	node.Label = "Upload"
	html := string(renderNode(node, rootPath))
	for _, wanted := range []string{"poem-progress", `max="100"`, `value="40"`, `aria-label="Upload"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("ProgressBar missing %q: %s", wanted, html)
		}
	}
}

// TestProgressBarContributesNoFieldOrMessage proves a ProgressBarNode is
// display-only: like BadgeNode it posts no field and allows no message.
func TestProgressBarContributesNoFieldOrMessage(t *testing.T) {
	view := app.Container(app.Vertical, 0, app.ProgressBar(40, 100))
	fields := map[string]postedField{}
	collectFields(view, rootPath, fields)
	if len(fields) != 0 {
		t.Fatalf("progress bar contributed fields: %+v", fields)
	}
	allowed := map[string]bool{}
	collectMessages(view, allowed)
	if len(allowed) != 0 {
		t.Fatalf("progress bar contributed messages: %+v", allowed)
	}
}

func TestRenderTableEmitsRowsAndHeadings(t *testing.T) {
	node := app.Table(
		[]app.TableColumn{{Key: "name", Label: "Name"}, {Key: "role", Label: "Role"}},
		[]app.TableRow{{Cells: map[string]string{"name": "Ada", "role": "Eng"}}},
	)
	node.Caption = "Team"
	html := string(renderNode(node, rootPath))
	for _, wanted := range []string{"poem-table", "<caption>Team</caption>", "<th", "Name", "Role", "<td>Ada</td>", "<td>Eng</td>"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("Table missing %q: %s", wanted, html)
		}
	}
}

// TestTableContributesNoFieldOrMessage proves a read-only TableNode posts no
// field and allows no message, like the other display-only kinds.
func TestTableContributesNoFieldOrMessage(t *testing.T) {
	view := app.Container(app.Vertical, 0, app.Table(
		[]app.TableColumn{{Key: "name", Label: "Name"}},
		[]app.TableRow{{Cells: map[string]string{"name": "Ada"}}},
	))
	fields := map[string]postedField{}
	collectFields(view, rootPath, fields)
	allowed := map[string]bool{}
	collectMessages(view, allowed)
	if len(fields) != 0 || len(allowed) != 0 {
		t.Fatalf("read-only table contributed fields=%+v messages=%+v", fields, allowed)
	}
}

func TestRenderTabsPostsSelectionAndRendersOnlyActivePanel(t *testing.T) {
	node := app.Tabs([]app.Tab{
		{ID: "one", Label: "One", Content: []app.Node{app.Text("first panel")}},
		{ID: "two", Label: "Two", Content: []app.Node{app.Text("second panel")}},
	}, "two", app.Msg{Name: "tab"})
	html := string(renderNode(node, rootPath))
	for _, wanted := range []string{`role="tablist"`, `type="submit"`, `name="` + fieldPrefix + rootPath + `"`, `value="one"`, `value="two"`, "second panel"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("Tabs missing %q: %s", wanted, html)
		}
	}
	// The inactive tab's panel must not be in the form at all.
	if strings.Contains(html, "first panel") {
		t.Fatalf("inactive tab content was rendered: %s", html)
	}
	if strings.Count(html, `aria-selected="true"`) != 1 {
		t.Fatalf("expected exactly one selected tab: %s", html)
	}
}

// TestTabsFieldDispatchesActivatedTab proves the tab strip collects as a
// valueField, so the clicked tab button's posted value reaches OnChange, and
// that only the active tab's nested controls are collected.
func TestTabsFieldDispatchesActivatedTab(t *testing.T) {
	view := app.Tabs([]app.Tab{
		{ID: "one", Label: "One", Content: []app.Node{app.Button("hidden", app.Msg{Name: "hiddenMsg"})}},
		{ID: "two", Label: "Two", Content: []app.Node{app.Button("shown", app.Msg{Name: "shownMsg"})}},
	}, "two", app.Msg{Name: "tab"})

	fields := map[string]postedField{}
	collectFields(view, rootPath, fields)
	strip, ok := fields[fieldPrefix+rootPath]
	if !ok {
		t.Fatalf("tab strip field not collected: %+v", fields)
	}
	if strip.kind != valueField || strip.msg.Name != "tab" {
		t.Fatalf("strip field = %+v, want valueField dispatching tab", strip)
	}

	allowed := map[string]bool{}
	collectMessages(view, allowed)
	if !allowed["tab"] || !allowed["shownMsg"] {
		t.Fatalf("expected tab and active-panel messages allowed: %+v", allowed)
	}
	if allowed["hiddenMsg"] {
		t.Fatalf("inactive tab's message must not be allowed: %+v", allowed)
	}
}

func TestRenderAccordionEmitsDisclosuresPerSection(t *testing.T) {
	node := app.Accordion(
		app.AccordionSection{Title: "General", Content: []app.Node{app.Text("g")}},
		app.AccordionSection{Title: "Advanced", DefaultOpen: true, Content: []app.Node{app.Text("a")}},
	)
	html := string(renderNode(node, rootPath))
	if strings.Count(html, "<details") != 2 {
		t.Fatalf("expected two <details> sections: %s", html)
	}
	for _, wanted := range []string{"poem-disclosure", "General", "Advanced", "<summary>"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("Accordion missing %q: %s", wanted, html)
		}
	}
	// Only the DefaultOpen section carries the open attribute.
	if strings.Count(html, " open") != 1 {
		t.Fatalf("expected exactly one open section: %s", html)
	}
}

// TestAccordionRecursesIntoSectionContent proves nested controls inside a
// section are collected as real fields/messages (all section content renders
// into the form up front, regardless of a section's open state).
func TestAccordionRecursesIntoSectionContent(t *testing.T) {
	view := app.Accordion(
		app.AccordionSection{Title: "S1", Content: []app.Node{
			app.TextInput("", "name", app.Msg{Name: "setName"}),
		}},
		app.AccordionSection{Title: "S2", Content: []app.Node{
			app.Button("go", app.Msg{Name: "go"}),
		}},
	)
	fields := map[string]postedField{}
	collectFields(view, rootPath, fields)
	if len(fields) != 1 {
		t.Fatalf("expected the nested text input collected, got %+v", fields)
	}
	allowed := map[string]bool{}
	collectMessages(view, allowed)
	if !allowed["setName"] || !allowed["go"] {
		t.Fatalf("nested section messages not allowed: %+v", allowed)
	}
}

// TestTextAreaFieldRoundTrips proves a value posted for a TextAreaNode reaches
// Update via its OnChange Msg, the same valueField path a TextInput uses, and
// that a non-interactive BadgeNode contributes no field or allowed message.
func TestTextAreaFieldRoundTrips(t *testing.T) {
	view := app.Container(app.Vertical, 0,
		app.Badge("status", app.VariantNeutral),
		app.TextArea("", "notes", app.Msg{Name: "edit"}),
	)

	fields := map[string]postedField{}
	collectFields(view, rootPath, fields)
	if len(fields) != 1 {
		t.Fatalf("collected %d fields, want only the textarea: %+v", len(fields), fields)
	}
	areaField, ok := fields[fieldPrefix+childPath(rootPath, 1)]
	if !ok {
		t.Fatalf("textarea field not collected under its path: %+v", fields)
	}
	if areaField.kind != valueField || areaField.msg.Name != "edit" {
		t.Fatalf("textarea field = %+v, want valueField dispatching edit", areaField)
	}

	allowed := map[string]bool{}
	collectMessages(view, allowed)
	if !allowed["edit"] {
		t.Fatalf("textarea OnChange not allowed: %+v", allowed)
	}
	if len(allowed) != 1 {
		t.Fatalf("badge should contribute no message; allowed = %+v", allowed)
	}
}

func TestRenderImageEmitsDataURI(t *testing.T) {
	// A 1x1 transparent PNG.
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R',
		0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89,
		0, 0, 0, 0x0d, 'I', 'D', 'A', 'T', 0x78, 0x9c, 0x62, 0, 1, 0, 0, 5, 0, 1, 0x0d, 0x0a, 0x2d, 0xb4,
		0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}
	node := app.Image(png, "route overview")
	html := string(renderNode(node, rootPath))
	for _, wanted := range []string{`src="data:image/png;base64,`, `alt="route overview"`, "trellis-image"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("Image missing %q: %s", wanted, html)
		}
	}
	// Non-image bytes must render nothing rather than an unsniffable URI.
	if got := string(renderNode(app.Image([]byte("plain text here"), "x"), rootPath)); got != "" {
		t.Fatalf("non-image bytes rendered %q", got)
	}
}
