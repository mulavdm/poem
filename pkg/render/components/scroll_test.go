package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestScrollViewPublishesAndAppliesSemanticScrollPercent(t *testing.T) {
	view := NewScrollView("scroll")
	view.Rect = image.Rect(0, 0, 300, 100)
	view.ContentH = 500
	state := &types.ApplicationState{ScrollPositions: make(map[string]int)}
	node := view.Semantics(state)
	if node.Scroll == nil || !node.Scroll.VerticallyScrollable || node.Scroll.VerticalPercent != 0 || node.Scroll.VerticalViewSize != 20 {
		t.Fatalf("scroll metadata=%+v", node.Scroll)
	}
	if !view.PerformSemanticAction("scroll", semantics.ActionSetScroll, "-1:50", state) {
		t.Fatal("semantic scroll action was ignored")
	}
	if view.ScrollY != 200 || view.CurrentScrollY != 200 || state.ScrollPositions["scroll"] != 200 {
		t.Fatalf("scroll position target=%d current=%d state=%d", view.ScrollY, view.CurrentScrollY, state.ScrollPositions["scroll"])
	}
}
