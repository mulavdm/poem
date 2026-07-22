package designlint

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/components"
	"github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestLintLayoutReportsNarrowTextBearingControl(t *testing.T) {
	button := components.NewButton("narrow", "Deliberately narrowed action", nil)
	button.SetBounds(image.Rect(0, 0, 80, 38))

	if !hasLayoutCode(LintLayout(button, image.Rect(0, 0, 200, 100), &types.ApplicationState{FontCharWidth: 8}), "UI023") {
		t.Fatal("expected UI023 for a text-bearing control narrower than its intrinsic minimum")
	}
}

func TestLintLayoutReportsFocusOrderThatContradictsVisualOrder(t *testing.T) {
	first := components.NewButton("first", "First", nil)
	second := components.NewButton("second", "Second", nil)
	first.SetBounds(image.Rect(0, 50, 100, 88))
	second.SetBounds(image.Rect(0, 0, 100, 38))
	root := &layout.FlexBox{CompID: "root", Direction: layout.Vertical, Children: []types.Component{first, second}}
	root.Rect = image.Rect(0, 0, 200, 100)
	// Set child bounds after assigning the root rectangle so this deliberately
	// malformed fixture is not repaired by FlexBox.SetBounds.
	first.SetBounds(image.Rect(0, 50, 100, 88))
	second.SetBounds(image.Rect(0, 0, 100, 38))

	if !hasLayoutCode(LintLayout(root, image.Rect(0, 0, 200, 100), &types.ApplicationState{FontCharWidth: 8}), "UI103") {
		t.Fatal("expected UI103 when focus traversal contradicts visual order")
	}
}

func TestLintLayoutReportsOffCanvasActionWithoutScrollPath(t *testing.T) {
	button := components.NewButton("off-canvas", "Action", nil)
	button.SetBounds(image.Rect(0, 140, 140, 178))

	if !hasLayoutCode(LintLayout(button, image.Rect(0, 0, 200, 100), &types.ApplicationState{FontCharWidth: 8}), "UI102") {
		t.Fatal("expected UI102 for an off-canvas action without a scroll path")
	}
}

func TestLintLayoutAcceptsActionReachableThroughScrollViewport(t *testing.T) {
	button := components.NewButton("reachable", "Reachable action", nil)
	button.SetBounds(image.Rect(0, 180, 160, 218))
	scroll := components.NewScrollView("scroll", button)
	scroll.SetBounds(image.Rect(0, 0, 200, 100))
	button.SetBounds(image.Rect(0, 180, 160, 218))
	scroll.ContentH = 240

	if hasLayoutCode(LintLayout(scroll, image.Rect(0, 0, 200, 100), &types.ApplicationState{FontCharWidth: 8}), "UI102") {
		t.Fatal("did not expect UI102 when scrolling can reach the action")
	}
}

func hasLayoutCode(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
