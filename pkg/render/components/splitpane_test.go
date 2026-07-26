package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
)

func TestSplitPaneEnforcesMinimumsAndReportsChanges(t *testing.T) {
	state := &types.ApplicationState{}
	changed := 0
	pane := &SplitPane{CompID: "workspace", Rect: image.Rect(0, 0, 500, 300), SplitOffset: 200, SplitBarWidth: 6, MinLeft: 120, MinRight: 160,
		OnSplitChanged: func(offset int, _ *types.ApplicationState) { changed = offset }}
	state.ActiveID = "workspace_splitter"
	if !pane.OnMouseMove(image.Pt(10, 20), state) || pane.SplitOffset != 120 || changed != 120 {
		t.Fatalf("minimum/callback offset=%d changed=%d", pane.SplitOffset, changed)
	}
	state.FocusedID = "workspace_splitter"
	if !pane.OnKey(0x27, 0, state) || pane.SplitOffset != 128 {
		t.Fatalf("keyboard adjustment offset=%d", pane.SplitOffset)
	}
}
