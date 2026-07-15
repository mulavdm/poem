package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

func accordionTestState() *types.ApplicationState {
	return &types.ApplicationState{TransientState: renderstate.NewStore()}
}

func TestAccordionMeasuresAndLayoutsExpandedContent(t *testing.T) {
	state := accordionTestState()
	content := NewLabel("details.content", "Expanded details")
	accordion := NewAccordion("settings", []AccordionItem{{ID: "details", Title: "Details", Content: content}}, map[string]bool{"details": true}, nil)
	measurement := accordion.Measure(image.Pt(320, 400), state)
	if measurement.Preferred.Y <= activeTheme(state).Controls.Medium {
		t.Fatalf("expanded content was not measured: %+v", measurement)
	}
	accordion.Rect = image.Rect(0, 0, 320, measurement.Preferred.Y)
	layout := accordion.rebuildLayout(state)
	if layout[0].content.Empty() || content.Bounds() != layout[0].content {
		t.Fatalf("content bounds=%v layout=%v", content.Bounds(), layout[0].content)
	}
}

func TestAccordionControlledToggle(t *testing.T) {
	state := accordionTestState()
	requestedID := ""
	requestedValue := false
	accordion := NewAccordion("settings", []AccordionItem{{ID: "details", Title: "Details"}}, map[string]bool{}, func(id string, expanded bool, _ *types.ApplicationState) {
		requestedID, requestedValue = id, expanded
	})
	if !accordion.toggle(0, state) || requestedID != "details" || !requestedValue {
		t.Fatalf("toggle callback=%q %v", requestedID, requestedValue)
	}
	if accordion.Expanded["details"] {
		t.Fatal("controlled accordion mutated application value")
	}
}

func TestAccordionUncontrolledExpansionSurvivesRebuild(t *testing.T) {
	state := accordionTestState()
	items := []AccordionItem{{ID: "general", Title: "General"}, {ID: "advanced", Title: "Advanced"}}

	// An uncontrolled accordion (nil OnToggle) rebuilt from scratch every frame
	// must not lose a toggle: expansion lives in the transient store, keyed by
	// CompID, exactly like Tabs/Select persist their own uncontrolled state.
	first := NewAccordion("settings", items, map[string]bool{}, nil)
	first.Rect = image.Rect(0, 0, 320, 200)
	first.rebuildLayout(state)
	if !first.toggle(1, state) {
		t.Fatal("toggle was not handled")
	}
	if !first.Expanded["advanced"] {
		t.Fatal("first instance did not record expansion")
	}

	// A fresh instance with the same CompID and a fresh (empty) initial map —
	// what a rebuild produces — must see the persisted expansion.
	rebuilt := NewAccordion("settings", items, map[string]bool{}, nil)
	rebuilt.Rect = image.Rect(0, 0, 320, 200)
	rebuilt.rebuildLayout(state)
	if !rebuilt.Expanded["advanced"] {
		t.Fatalf("rebuilt instance lost expansion: %+v", rebuilt.Expanded)
	}

	// Toggling it back off must likewise persist.
	if !rebuilt.toggle(1, state) {
		t.Fatal("second toggle was not handled")
	}
	again := NewAccordion("settings", items, map[string]bool{}, nil)
	again.Rect = image.Rect(0, 0, 320, 200)
	again.rebuildLayout(state)
	if again.Expanded["advanced"] {
		t.Fatalf("collapse did not persist across rebuild: %+v", again.Expanded)
	}
}

func TestAccordionInitialExpandedSeedsUncontrolledStore(t *testing.T) {
	state := accordionTestState()
	items := []AccordionItem{{ID: "general", Title: "General"}, {ID: "advanced", Title: "Advanced"}}

	// The author's initial open set seeds the store on first sight, then a
	// rebuild that passes an empty map must still show the seeded section open.
	first := NewAccordion("settings", items, map[string]bool{"general": true}, nil)
	first.Rect = image.Rect(0, 0, 320, 200)
	first.rebuildLayout(state)

	rebuilt := NewAccordion("settings", items, map[string]bool{}, nil)
	rebuilt.Rect = image.Rect(0, 0, 320, 200)
	rebuilt.rebuildLayout(state)
	if !rebuilt.Expanded["general"] {
		t.Fatalf("seeded default-open lost across rebuild: %+v", rebuilt.Expanded)
	}
}

func TestAccordionKeyboardHeaderSurvivesRebuild(t *testing.T) {
	state := accordionTestState()
	state.FocusedID = "settings"
	items := []AccordionItem{{ID: "general", Title: "General"}, {ID: "advanced", Title: "Advanced"}}
	first := NewAccordion("settings", items, map[string]bool{}, nil)
	if !first.OnKey(0x28, 0, state) {
		t.Fatal("down key was not handled")
	}
	requested := ""
	rebuilt := NewAccordion("settings", items, map[string]bool{}, func(id string, _ bool, _ *types.ApplicationState) { requested = id })
	if !rebuilt.OnKey(13, 0, state) || requested != "advanced" {
		t.Fatalf("retained header toggled %q", requested)
	}
}

func TestAccordionSemanticsExposeDisclosureState(t *testing.T) {
	state := accordionTestState()
	accordion := NewAccordion("settings", []AccordionItem{{ID: "details", Title: "Details"}}, map[string]bool{"details": true}, nil)
	accordion.Rect = image.Rect(0, 0, 320, 80)
	node := accordion.Semantics(state)
	if len(node.Children) != 1 || !node.Children[0].State.Expanded || node.Children[0].ID != "settings/details" {
		t.Fatalf("unexpected semantics: %+v", node.Children)
	}
}

func TestAccordionWalkSkipsCollapsedContent(t *testing.T) {
	content := NewButton("hidden", "Hidden", nil)
	accordion := NewAccordion("settings", []AccordionItem{{ID: "details", Title: "Details", Content: content}}, map[string]bool{}, nil)
	visited := []string{}
	accordion.Walk(func(component types.Component) { visited = append(visited, component.ID()) })
	if len(visited) != 1 || visited[0] != "settings" {
		t.Fatalf("collapsed walk visited %v", visited)
	}
	accordion.Expanded["details"] = true
	visited = visited[:0]
	accordion.Walk(func(component types.Component) { visited = append(visited, component.ID()) })
	if len(visited) != 2 || visited[1] != "hidden" {
		t.Fatalf("expanded walk visited %v", visited)
	}
}

func TestAccordionHomeEndAndArrowsSkipDisabledHeaders(t *testing.T) {
	state := accordionTestState()
	state.FocusedID = "settings"
	items := []AccordionItem{
		{ID: "disabled-first", Title: "Disabled first", Disabled: true},
		{ID: "general", Title: "General"},
		{ID: "advanced", Title: "Advanced"},
		{ID: "disabled-last", Title: "Disabled last", Disabled: true},
	}
	accordion := NewAccordion("settings", items, nil, nil)
	if !accordion.OnKey(0x23, 0, state) || accordion.activeHeader(state) != 2 {
		t.Fatalf("End focused header %d", accordion.activeHeader(state))
	}
	if !accordion.OnKey(0x28, 0, state) || accordion.activeHeader(state) != 1 {
		t.Fatalf("Down did not wrap to first enabled header: %d", accordion.activeHeader(state))
	}
	if !accordion.OnKey(0x24, 0, state) || accordion.activeHeader(state) != 1 {
		t.Fatalf("Home focused header %d", accordion.activeHeader(state))
	}
	if !accordion.OnKey(0x26, 0, state) || accordion.activeHeader(state) != 2 {
		t.Fatalf("Up did not wrap to last enabled header: %d", accordion.activeHeader(state))
	}
}

func TestAccordionSemanticFocusIsSingleAndNonDestructive(t *testing.T) {
	state := accordionTestState()
	accordion := NewAccordion("settings", []AccordionItem{{ID: "general", Title: "General"}, {ID: "advanced", Title: "Advanced"}}, map[string]bool{}, nil)
	accordion.Rect = image.Rect(0, 0, 320, 80)
	if !accordion.PerformSemanticAction("settings/advanced", semantics.ActionFocus, "", state) {
		t.Fatal("header focus action failed")
	}
	node := accordion.Semantics(state)
	if node.State.Focused || node.Children[0].State.Focused || !node.Children[1].State.Focused || accordion.Expanded["advanced"] {
		t.Fatalf("focus mutated disclosure or produced duplicate focus: root=%+v children=%+v expanded=%v", node.State, node.Children, accordion.Expanded)
	}
	if accordion.PerformSemanticAction("settings/advanced", semantics.ActionInvoke, "", state) {
		t.Fatal("header accepted unadvertised invoke action")
	}
	if !accordion.PerformSemanticAction("settings/advanced", semantics.ActionExpand, "", state) || !accordion.Expanded["advanced"] {
		t.Fatal("header did not expand")
	}
	if !accordion.PerformSemanticAction("settings/advanced", semantics.ActionExpand, "", state) || !accordion.Expanded["advanced"] {
		t.Fatal("idempotent expansion collapsed the header")
	}
	if !accordion.PerformSemanticAction("settings/advanced", semantics.ActionCollapse, "", state) || accordion.Expanded["advanced"] {
		t.Fatal("header did not collapse")
	}
}

func TestAccordionDisabledOnlyIsNotFocusable(t *testing.T) {
	accordion := NewAccordion("settings", []AccordionItem{{ID: "disabled", Title: "Disabled", Disabled: true}}, nil, nil)
	if accordion.Focusable() {
		t.Fatal("disabled-only accordion entered sequential focus")
	}
}
