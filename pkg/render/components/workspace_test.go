package components

import (
	"image"
	"testing"

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
