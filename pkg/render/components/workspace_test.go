package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestWorkspaceEscapeCollapsesCompactSheetOnce(t *testing.T) {
	workspace := &Workspace{CompID: "workspace"}
	workspace.SetBounds(image.Rect(0, 0, 390, 800))
	state := &types.ApplicationState{}
	if !workspace.OnKey(0x1B, 0, state) {
		t.Fatal("first Escape did not collapse compact sheet")
	}
	if got := workspace.interaction(state).SheetFraction; got != .24 {
		t.Fatalf("sheet fraction = %v, want .24", got)
	}
	if workspace.OnKey(0x1B, 0, state) {
		t.Fatal("already-collapsed sheet consumed a second Escape")
	}
}

func TestWorkspaceEscapeDoesNotCollapseExpandedTools(t *testing.T) {
	workspace := &Workspace{CompID: "workspace"}
	workspace.SetBounds(image.Rect(0, 0, 1200, 800))
	if workspace.OnKey(0x1B, 0, &types.ApplicationState{}) {
		t.Fatal("expanded workspace consumed Escape")
	}
}

func TestWorkspaceMeasuresExpandedToolsWithoutClippingActionText(t *testing.T) {
	from := NewButton("from", "⌖  From · Choose a starting point", nil)
	toolsContent := &layout.FlexBox{CompID: "tools", Direction: layout.Vertical, Padding: 16, Children: []types.Component{from}}
	tools := NewScrollView("tools-scroll", toolsContent)
	content := NewPanel("map")
	workspace := &Workspace{CompID: "workspace", Content: content, Tools: tools}
	workspace.SetBounds(image.Rect(0, 0, 900, 700))
	state := &types.ApplicationState{FontCharWidth: 8, TextScale: 1}
	workspace.layout(state)
	tools.layout(state)
	toolsContent.Draw(&recordingPainter{}, state)

	if workspace.compact {
		t.Fatal("900x700 at the default text scale should use expanded tools")
	}
	minimum := from.Measure(image.Point{}, state).Min
	if got := from.Bounds(); got.Dx() < minimum.X {
		t.Fatalf("From action width = %d, intrinsic minimum = %d", got.Dx(), minimum.X)
	}
	if content.Bounds().Dx() < workspaceLayoutMetrics(state).minMap {
		t.Fatalf("map width = %d, minimum = %d", content.Bounds().Dx(), workspaceLayoutMetrics(state).minMap)
	}
}

func TestWorkspaceUsesCompactToolsWhenTextScaleCannotPreserveMapMinimum(t *testing.T) {
	from := NewButton("from", "⌖  From · Choose a starting point", nil)
	toolsContent := &layout.FlexBox{CompID: "tools", Direction: layout.Vertical, Padding: 16, Children: []types.Component{from}}
	tools := NewScrollView("tools-scroll", toolsContent)
	workspace := &Workspace{CompID: "workspace", Content: NewPanel("map"), Tools: tools}
	workspace.SetBounds(image.Rect(0, 0, 900, 700))
	state := &types.ApplicationState{FontCharWidth: 16, TextScale: 2}
	workspace.layout(state)
	tools.layout(state)
	toolsContent.Draw(&recordingPainter{}, state)

	if !workspace.compact {
		t.Fatal("scaled tools should reflow into the compact sheet when expanded tools would consume the map minimum")
	}
	minimum := from.Measure(image.Point{}, state).Min
	if got := from.Bounds(); got.Dx() < minimum.X {
		t.Fatalf("scaled From action width = %d, intrinsic minimum = %d", got.Dx(), minimum.X)
	}
}

func TestWorkspaceUsesCompactToolsWhenScaledMediumRailWouldHideMapContext(t *testing.T) {
	tools := NewScrollView("tools-scroll", NewButton("from", "From - Choose a starting point", nil))
	content := NewPanel("map")
	workspace := &Workspace{CompID: "workspace", Content: content, Tools: tools}
	workspace.SetBounds(image.Rect(0, 0, 720, 800))
	state := &types.ApplicationState{FontCharWidth: 16, TextScale: 2}
	workspace.layout(state)

	if workspace.WindowClass() != WorkspaceCompact {
		t.Fatalf("scaled medium class=%v, want compact fallback", workspace.WindowClass())
	}
	if content.Bounds().Dx() != 720 {
		t.Fatalf("compact fallback map width=%d, want 720", content.Bounds().Dx())
	}
}

func TestWorkspaceToolsHaveIndependentVerticalScrollExtent(t *testing.T) {
	children := make([]types.Component, 20)
	for index := range children {
		children[index] = NewButton("action", "Planner or settings action", nil)
	}
	toolsContent := &layout.FlexBox{CompID: "tools", Direction: layout.Vertical, Gap: 10, Padding: 16, Children: children}
	tools := NewScrollView("tools-scroll", toolsContent)
	workspace := &Workspace{CompID: "workspace", Content: NewPanel("map"), Tools: tools}
	workspace.SetBounds(image.Rect(0, 0, 900, 700))
	state := &types.ApplicationState{FontCharWidth: 8, TextScale: 1}
	workspace.layout(state)
	tools.layout(state)

	if tools.ContentH <= tools.Bounds().Dy() {
		t.Fatalf("tools content height = %d, viewport height = %d; want an independent scroll path", tools.ContentH, tools.Bounds().Dy())
	}
}

func TestWorkspaceResolvesAllFourWindowClasses(t *testing.T) {
	state := &types.ApplicationState{FontCharWidth: 8, TextScale: 1}
	for _, test := range []struct {
		name  string
		width int
		want  WorkspaceWindowClass
	}{
		{name: "compact", width: 599, want: WorkspaceCompact},
		{name: "medium", width: 600, want: WorkspaceMedium},
		{name: "expanded", width: 840, want: WorkspaceExpanded},
		{name: "ultra-wide", width: 1200, want: WorkspaceUltraWide},
	} {
		t.Run(test.name, func(t *testing.T) {
			content := NewPanel("map")
			tools := NewScrollView("tools-scroll", NewButton("search", "Search", nil))
			detail := NewScrollView("detail-scroll", NewLabel("detail-label", "Route details"))
			workspace := &Workspace{CompID: "workspace", Content: content, Tools: tools, Detail: detail}
			workspace.SetBounds(image.Rect(0, 0, test.width, 700))
			workspace.layout(state)

			if got := workspace.WindowClass(); got != test.want {
				t.Fatalf("window class=%v, want %v", got, test.want)
			}
			switch test.want {
			case WorkspaceCompact:
				if content.Bounds().Dx() != test.width || tools.Bounds().Min.Y <= content.Bounds().Min.Y {
					t.Fatalf("compact bounds content=%v tools=%v", content.Bounds(), tools.Bounds())
				}
			case WorkspaceMedium:
				if content.Bounds().Dx() != test.width || tools.Bounds().Dx() >= content.Bounds().Dx() {
					t.Fatalf("medium rail did not overlay a persistent canvas: content=%v tools=%v", content.Bounds(), tools.Bounds())
				}
			case WorkspaceExpanded:
				if detail.Bounds() != (image.Rectangle{}) || tools.Bounds().Max.X >= content.Bounds().Min.X {
					t.Fatalf("expanded bounds content=%v tools=%v detail=%v", content.Bounds(), tools.Bounds(), detail.Bounds())
				}
			case WorkspaceUltraWide:
				if detail.Bounds().Empty() || tools.Bounds().Max.X >= detail.Bounds().Min.X || detail.Bounds().Max.X >= content.Bounds().Min.X {
					t.Fatalf("ultra-wide columns content=%v tools=%v detail=%v", content.Bounds(), tools.Bounds(), detail.Bounds())
				}
			}
		})
	}
}

func TestWorkspacePreservesInputFocusAndScrollAcrossClasses(t *testing.T) {
	query := NewTextInput("query", "Address or place")
	query.Value = "half-entered route"
	toolsContent := &layout.FlexBox{CompID: "tools", Direction: layout.Vertical, Children: []types.Component{query}}
	tools := NewScrollView("tools-scroll", toolsContent)
	tools.ScrollY, tools.CurrentScrollY = 37, 37
	workspace := &Workspace{CompID: "workspace", Content: NewPanel("map"), Tools: tools, Detail: NewScrollView("detail-scroll", NewLabel("detail", "Route details"))}
	state := &types.ApplicationState{ActiveID: query.ID(), FontCharWidth: 8, TextScale: 1}

	for _, width := range []int{411, 720, 900, 1600, 900} {
		workspace.SetBounds(image.Rect(0, 0, width, 700))
		workspace.layout(state)
		if query.Value != "half-entered route" || state.ActiveID != query.ID() {
			t.Fatalf("state changed at width %d: value=%q active=%q", width, query.Value, state.ActiveID)
		}
		if tools.ScrollY != 37 || tools.CurrentScrollY != 37 {
			t.Fatalf("scroll anchor changed at width %d: target=%d current=%d", width, tools.ScrollY, tools.CurrentScrollY)
		}
	}
}
