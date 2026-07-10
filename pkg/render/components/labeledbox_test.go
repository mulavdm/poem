package components

import (
	"image"
	"testing"

	"go_native_gpu_gui/pkg/render/semantics"
	"go_native_gpu_gui/pkg/render/types"
)

func TestLabeledBoxRelatesChildToVisibleLabel(t *testing.T) {
	child := NewTextInput("name", "")
	box := &LabeledBox{CompID: "name-field", Title: "Project name", Child: child}
	box.SetBounds(image.Rect(10, 20, 250, 90))
	state := &types.ApplicationState{CurrentPage: "page", Pages: map[string][]types.Component{"page": {box}}}
	tree := types.BuildSemanticsTree(state)
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	node, ok := tree.Find("name")
	if !ok || len(node.Relations.LabeledBy) != 1 || node.Relations.LabeledBy[0] != "name-field/label" {
		t.Fatalf("child relationships=%+v", node.Relations)
	}
	label, ok := tree.Find("name-field/label")
	if !ok || label.Role != semantics.RoleText || label.Name != "Project name" {
		t.Fatalf("label=%+v found=%v", label, ok)
	}
}
