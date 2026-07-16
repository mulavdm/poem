package desktop

import (
	"testing"

	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/examples/preferences"
	"github.com/mulavdm/poem/pkg/app"
)

// collectDispatched builds node with a dispatch that records every Msg it
// fires, returning the built component and a pointer to the recorded slice so
// a test can trigger a handler and assert what Update would have received.
func collectDispatched(node app.Node) (render.Component, *[]app.Msg) {
	var fired []app.Msg
	component := build(node, "root", func(m app.Msg) { fired = append(fired, m) })
	return component, &fired
}

func TestBuildTextTranslatesToLabel(t *testing.T) {
	component := build(app.Text("hello"), "root", func(app.Msg) {})
	label, ok := component.(*render.Label)
	if !ok {
		t.Fatalf("Text built %T, want *render.Label", component)
	}
	if label.ID() != "root" {
		t.Fatalf("label ID = %q, want root", label.ID())
	}
}

func TestBuildButtonDispatchesOnClick(t *testing.T) {
	component, fired := collectDispatched(app.Button("Save", app.Msg{Name: "save"}))
	button, ok := component.(*render.Button)
	if !ok {
		t.Fatalf("Button built %T, want *render.Button", component)
	}
	button.OnClick(nil)
	if len(*fired) != 1 || (*fired)[0].Name != "save" {
		t.Fatalf("dispatched %+v, want one save msg", *fired)
	}
}

func TestBuildTextAreaDispatchesValue(t *testing.T) {
	node := app.TextArea("draft", "write here", app.Msg{Name: "edit"})
	node.Disabled = true
	component, fired := collectDispatched(node)
	area, ok := component.(*render.TextArea)
	if !ok {
		t.Fatalf("TextArea built %T, want *render.TextArea", component)
	}
	if area.Value != "draft" || area.Placeholder != "write here" || !area.Disabled {
		t.Fatalf("area fields = %+v, want value/placeholder/disabled carried through", area)
	}
	area.OnChange("edited", nil)
	if len(*fired) != 1 || (*fired)[0].Name != "edit" || (*fired)[0].Payload != "edited" {
		t.Fatalf("dispatched %+v, want one edit msg carrying the new value", *fired)
	}
}

func TestBuildSwitchDispatchesBool(t *testing.T) {
	node := app.Switch("Wi-Fi", true, app.Msg{Name: "wifi"})
	node.Disabled = true
	component, fired := collectDispatched(node)
	toggle, ok := component.(*render.Switch)
	if !ok {
		t.Fatalf("Switch built %T, want *render.Switch", component)
	}
	if toggle.Label != "Wi-Fi" || !toggle.Checked || !toggle.Disabled {
		t.Fatalf("switch fields = %+v, want label/checked/disabled carried through", toggle)
	}
	toggle.OnChange(false, nil)
	if len(*fired) != 1 || (*fired)[0].Name != "wifi" || (*fired)[0].Payload != app.BoolPayload(false) {
		t.Fatalf("dispatched %+v, want one wifi msg carrying false", *fired)
	}
}

func TestBuildRadioGroupSelectsAndDispatches(t *testing.T) {
	options := []app.Option{
		{Label: "Free", Value: "free"},
		{Label: "Pro", Value: "pro"},
	}
	component, fired := collectDispatched(app.RadioGroup(options, "pro", app.Msg{Name: "plan"}))
	flex, ok := component.(*render.FlexBox)
	if !ok {
		t.Fatalf("RadioGroup built %T, want *render.FlexBox", component)
	}
	if len(flex.Children) != 2 {
		t.Fatalf("radio group has %d children, want 2", len(flex.Children))
	}
	free, ok := flex.Children[0].(*render.Radio)
	if !ok {
		t.Fatalf("first child %T, want *render.Radio", flex.Children[0])
	}
	pro := flex.Children[1].(*render.Radio)
	if free.Selected || !pro.Selected {
		t.Fatalf("selection = {free:%v pro:%v}, want only pro selected", free.Selected, pro.Selected)
	}
	if free.ID() != "root/0" || pro.ID() != "root/1" {
		t.Fatalf("radio IDs = %q,%q, want root/0,root/1", free.ID(), pro.ID())
	}
	free.OnSelect("free", nil)
	if len(*fired) != 1 || (*fired)[0].Name != "plan" || (*fired)[0].Payload != "free" {
		t.Fatalf("dispatched %+v, want one plan msg carrying free", *fired)
	}
}

func TestBuildBadgeMapsVariant(t *testing.T) {
	component := build(app.Badge("Live", app.VariantSuccess), "root", func(app.Msg) {})
	badge, ok := component.(*render.Badge)
	if !ok {
		t.Fatalf("Badge built %T, want *render.Badge", component)
	}
	if badge.Text != "Live" {
		t.Fatalf("badge text = %q, want Live", badge.Text)
	}
	if badge.Variant != render.VariantSuccess {
		t.Fatalf("badge variant = %v, want success", badge.Variant)
	}
}

func TestBuildSliderDispatchesFloat(t *testing.T) {
	node := app.Slider(0, 10, 2.5, app.Msg{Name: "volume"})
	node.Disabled = true
	component, fired := collectDispatched(node)
	slider, ok := component.(*render.Slider)
	if !ok {
		t.Fatalf("Slider built %T, want *render.Slider", component)
	}
	if slider.Min != 0 || slider.Max != 10 || slider.Value != 2.5 || !slider.Disabled {
		t.Fatalf("slider fields = %+v, want min/max/value/disabled carried through", slider)
	}
	slider.OnChange(7.5, nil)
	if len(*fired) != 1 || (*fired)[0].Name != "volume" {
		t.Fatalf("dispatched %+v, want one volume msg", *fired)
	}
	if got := (*fired)[0].Float(); got != 7.5 {
		t.Fatalf("payload decoded to %v, want 7.5", got)
	}
}

func TestBuildProgressBarCarriesValueAndMax(t *testing.T) {
	node := app.ProgressBar(40, 200)
	node.Label = "Upload"
	component := build(node, "root", func(app.Msg) {})
	bar, ok := component.(*render.ProgressBar)
	if !ok {
		t.Fatalf("ProgressBar built %T, want *render.ProgressBar", component)
	}
	if bar.Min != 0 || bar.Max != 200 || bar.Value != 40 {
		t.Fatalf("progress fields = {min:%v max:%v value:%v}, want 0/200/40", bar.Min, bar.Max, bar.Value)
	}
	if bar.AccessibleName != "Upload" {
		t.Fatalf("progress accessible name = %q, want Upload", bar.AccessibleName)
	}
}

func TestBuildProgressBarIndeterminate(t *testing.T) {
	node := app.ProgressBarNode{Indeterminate: true}
	bar := build(node, "root", func(app.Msg) {}).(*render.ProgressBar)
	if !bar.Indeterminate {
		t.Fatal("indeterminate flag not carried to POEM ProgressBar")
	}
	if bar.Max != 100 {
		t.Fatalf("progress max = %v, want default 100 when unset", bar.Max)
	}
}

func TestBuildTableCarriesColumnsRowsAndStableIDs(t *testing.T) {
	node := app.Table(
		[]app.TableColumn{{Key: "name", Label: "Name"}, {Key: "role", Label: "Role"}},
		[]app.TableRow{
			{Cells: map[string]string{"name": "Ada", "role": "Eng"}},
			{Cells: map[string]string{"name": "Grace", "role": "Eng"}},
		},
	)
	component := build(node, "root", func(app.Msg) {})
	table, ok := component.(*render.DataTable)
	if !ok {
		t.Fatalf("Table built %T, want *render.DataTable", component)
	}
	if len(table.Columns) != 2 || table.Columns[0].Key != "name" || table.Columns[1].Label != "Role" {
		t.Fatalf("columns not carried through: %+v", table.Columns)
	}
	if len(table.Rows) != 2 || table.Rows[0].Values["name"] != "Ada" {
		t.Fatalf("rows not carried through: %+v", table.Rows)
	}
	if table.Rows[0].ID == table.Rows[1].ID {
		t.Fatalf("rows share ID %q, want per-row stable IDs", table.Rows[0].ID)
	}
}

func TestBuildAccordionSeedsDefaultOpenAndDispatchesNestedMsg(t *testing.T) {
	node := app.Accordion(
		app.AccordionSection{Title: "General", Content: []app.Node{
			app.Button("save", app.Msg{Name: "save"}),
		}},
		app.AccordionSection{Title: "Advanced", DefaultOpen: true, Content: []app.Node{
			app.Text("more"),
		}},
	)
	component, fired := collectDispatched(node)
	accordion, ok := component.(*render.Accordion)
	if !ok {
		t.Fatalf("Accordion built %T, want *render.Accordion", component)
	}
	if len(accordion.Items) != 2 || accordion.Items[0].Title != "General" || accordion.Items[1].Title != "Advanced" {
		t.Fatalf("sections not carried through: %+v", accordion.Items)
	}
	// DefaultOpen seeds the initial expanded set (keyed by the per-section ID).
	if accordion.Expanded["s0"] || !accordion.Expanded["s1"] {
		t.Fatalf("default-open seeding wrong: %+v", accordion.Expanded)
	}
	// Uncontrolled: no OnToggle, so expansion is backend-local chrome.
	if accordion.OnToggle != nil {
		t.Fatal("AccordionNode should build an uncontrolled (nil OnToggle) Accordion")
	}
	// Nested content is real and still fires its Msg through Update.
	body, ok := accordion.Items[0].Content.(*render.FlexBox)
	if !ok || len(body.Children) != 1 {
		t.Fatalf("section 0 content = %+v, want a FlexBox with one child", accordion.Items[0].Content)
	}
	button, ok := body.Children[0].(*render.Button)
	if !ok {
		t.Fatalf("nested child %T, want *render.Button", body.Children[0])
	}
	button.OnClick(nil)
	if len(*fired) != 1 || (*fired)[0].Name != "save" {
		t.Fatalf("nested button dispatched %+v, want one save msg", *fired)
	}
}

func TestBuildTabsIsControlledAndRendersOnlyActivePanel(t *testing.T) {
	node := app.Tabs([]app.Tab{
		{ID: "one", Label: "One", Content: []app.Node{app.Text("first")}},
		{ID: "two", Label: "Two", Content: []app.Node{app.Button("go", app.Msg{Name: "go"})}},
	}, "two", app.Msg{Name: "tab"})
	component, fired := collectDispatched(node)
	flex, ok := component.(*render.FlexBox)
	if !ok {
		t.Fatalf("Tabs built %T, want *render.FlexBox", component)
	}
	if len(flex.Children) != 2 {
		t.Fatalf("want strip + one panel, got %d children", len(flex.Children))
	}
	strip, ok := flex.Children[0].(*render.Tabs)
	if !ok {
		t.Fatalf("first child %T, want *render.Tabs", flex.Children[0])
	}
	if strip.SelectedID != "two" {
		t.Fatalf("strip SelectedID = %q, want two", strip.SelectedID)
	}
	// Controlled: App[S] owns the selection, so OnChange must be wired.
	if strip.OnChange == nil {
		t.Fatal("TabsNode must build a controlled (non-nil OnChange) Tabs")
	}
	strip.OnChange("one", nil)
	if len(*fired) != 1 || (*fired)[0].Name != "tab" || (*fired)[0].Payload != "one" {
		t.Fatalf("dispatched %+v, want one tab msg carrying one", *fired)
	}

	// Only the active tab's content is built — the inactive tab's Button is absent.
	panel := flex.Children[1].(*render.FlexBox)
	if len(panel.Children) != 1 {
		t.Fatalf("panel children = %d, want 1", len(panel.Children))
	}
	if _, ok := panel.Children[0].(*render.Button); !ok {
		t.Fatalf("active panel built %T, want the selected tab's Button", panel.Children[0])
	}
}

func TestBuildTabsFallsBackToFirstTab(t *testing.T) {
	node := app.Tabs([]app.Tab{
		{ID: "one", Label: "One", Content: []app.Node{app.Text("first")}},
		{ID: "two", Label: "Two"},
	}, "nonexistent", app.Msg{Name: "tab"})
	if got := node.ActiveIndex(); got != 0 {
		t.Fatalf("ActiveIndex = %d, want 0 fallback", got)
	}
}

func TestBuildContainerAssignsStablePaths(t *testing.T) {
	node := app.Container(app.Vertical, 8,
		app.Text("a"),
		app.Button("b", app.Msg{Name: "b"}),
	)
	component := build(node, "root", func(app.Msg) {})
	flex, ok := component.(*render.FlexBox)
	if !ok {
		t.Fatalf("Container built %T, want *render.FlexBox", component)
	}
	if len(flex.Children) != 2 {
		t.Fatalf("container has %d children, want 2", len(flex.Children))
	}
	if got := flex.Children[0].ID(); got != "root/0" {
		t.Fatalf("first child ID = %q, want root/0", got)
	}
	if got := flex.Children[1].ID(); got != "root/1" {
		t.Fatalf("second child ID = %q, want root/1", got)
	}
}

// TestFullMouseDispatchTogglesCheckbox replicates run.go's mouse dispatch
// against the real preferences example tree — the exact sequence a tap
// produces on Android (Move context omitted; Down sets ActiveID via the
// root hit test, Up routes through the root's OnMouseUp broadcast). It
// reproduces the emulator finding that up-driven controls never toggle.
func TestFullMouseDispatchTogglesCheckbox(t *testing.T) {
	rstate := &render.ApplicationState{WindowWidth: 411, WindowHeight: 914}
	config := Configure(preferences.App, render.AppConfig{Title: "t"})
	config.BuildPagesFn(rstate)
	root := rstate.Pages["trellis-root"][0]

	// Locate the checkbox (first child) by its laid-out bounds.
	var checkbox *render.Checkbox
	root.Walk(func(c render.Component) {
		if cb, ok := c.(*render.Checkbox); ok && checkbox == nil {
			checkbox = cb
		}
	})
	if checkbox == nil {
		t.Fatal("no checkbox in preferences tree")
	}
	pt := checkbox.Bounds().Min.Add(checkbox.Bounds().Size().Div(2))

	// MouseDown path: hit test resolves the target, ActiveID records it.
	rstate.ActiveID = root.HitTest(pt)
	if rstate.ActiveID != checkbox.ID() {
		t.Fatalf("down hit %q, want %q (pt=%v bounds=%v)", rstate.ActiveID, checkbox.ID(), pt, checkbox.Bounds())
	}
	root.OnMouseDown(pt, rstate)

	// MouseUp path: the root broadcast, exactly as run.go dispatches it.
	handled := root.OnMouseUp(pt, rstate)
	rstate.ActiveID = ""

	// Rebuild from (possibly updated) app state and check the checkbox.
	config.BuildPagesFn(rstate)
	var rebuilt *render.Checkbox
	rstate.Pages["trellis-root"][0].Walk(func(c render.Component) {
		if cb, ok := c.(*render.Checkbox); ok && rebuilt == nil {
			rebuilt = cb
		}
	})
	if !rebuilt.Checked {
		t.Fatalf("checkbox did not toggle through full dispatch (up handled=%v)", handled)
	}
}

// TestOnlyActiveTargetConsumesMouseUp asserts the invariant the FlexBox
// mouse-up broadcast depends on: a release aimed at one control (ActiveID)
// must pass through every other sibling untouched — only the subtree owning
// the active target may consume it. Slider violated this (it swallowed every
// up and wiped ActiveID), which made all up-driven controls dead in any tree
// with a slider later than them; found by Android touch, reproduced here.
func TestOnlyActiveTargetConsumesMouseUp(t *testing.T) {
	rstate := &render.ApplicationState{WindowWidth: 411, WindowHeight: 914}
	config := Configure(preferences.App, render.AppConfig{Title: "t"})
	config.BuildPagesFn(rstate)
	root := rstate.Pages["trellis-root"][0].(*render.FlexBox)
	var checkbox *render.Checkbox
	root.Walk(func(c render.Component) {
		if cb, ok := c.(*render.Checkbox); ok && checkbox == nil {
			checkbox = cb
		}
	})
	pt := checkbox.Bounds().Min.Add(checkbox.Bounds().Size().Div(2))
	target := root.HitTest(pt)
	if target != checkbox.ID() {
		t.Fatalf("hit test found %q, want %q", target, checkbox.ID())
	}

	for i := len(root.Children) - 1; i >= 0; i-- {
		child := root.Children[i]
		rstate.ActiveID = target
		consumed := child.OnMouseUp(pt, rstate)
		ownsTarget := false
		child.Walk(func(c render.Component) {
			if c.ID() == target {
				ownsTarget = true
			}
		})
		if consumed && !ownsTarget {
			t.Fatalf("child %d (%T id=%q) consumed an up aimed at %q", i, child, child.ID(), target)
		}
		if rstate.ActiveID != target && !ownsTarget {
			t.Fatalf("child %d (%T id=%q) cleared ActiveID aimed at %q", i, child, child.ID(), target)
		}
	}
}
