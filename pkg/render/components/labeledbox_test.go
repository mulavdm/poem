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

func TestLabeledBoxWithoutHelpOrErrorKeepsOriginalSizing(t *testing.T) {
	child := NewTextInput("plain", "")
	box := &LabeledBox{CompID: "plain-field", Title: "Plain", Child: child}
	measured := box.Measure(image.Pt(200, 0), nil)
	box.SetBounds(image.Rect(0, 0, 200, measured.Preferred.Y))
	if box.hasDescription() {
		t.Fatal("expected no description row")
	}
	if child.Rect.Max.Y != box.Rect.Max.Y {
		t.Fatalf("child bottom=%d, want flush with box bottom=%d", child.Rect.Max.Y, box.Rect.Max.Y)
	}
}

func TestLabeledBoxHelpTextReservesRowAndDescribesChild(t *testing.T) {
	child := NewTextInput("with-help", "")
	box := &LabeledBox{CompID: "help-field", Title: "With help", Child: child, Help: "Used for display and routing."}
	measured := box.Measure(image.Pt(200, 0), nil)
	plainBox := &LabeledBox{CompID: "plain-field", Title: "With help", Child: NewTextInput("plain", "")}
	plainMeasured := plainBox.Measure(image.Pt(200, 0), nil)
	if measured.Preferred.Y <= plainMeasured.Preferred.Y {
		t.Fatalf("help row not reserved: with-help=%d plain=%d", measured.Preferred.Y, plainMeasured.Preferred.Y)
	}
	box.SetBounds(image.Rect(0, 0, 200, measured.Preferred.Y))
	if child.Rect.Max.Y >= box.Rect.Max.Y {
		t.Fatalf("expected child bottom short of box bottom, got child=%d box=%d", child.Rect.Max.Y, box.Rect.Max.Y)
	}

	state := &types.ApplicationState{CurrentPage: "page", Pages: map[string][]types.Component{"page": {box}}}
	tree := types.BuildSemanticsTree(state)
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	node, ok := tree.Find("with-help")
	if !ok || len(node.Relations.DescribedBy) != 1 || node.Relations.DescribedBy[0] != "help-field/description" {
		t.Fatalf("child relationships=%+v", node.Relations)
	}
	desc, ok := tree.Find("help-field/description")
	if !ok || desc.Role != semantics.RoleText || desc.Name != "Used for display and routing." {
		t.Fatalf("description=%+v found=%v", desc, ok)
	}
}

func TestLabeledBoxErrorTakesPrecedenceOverHelp(t *testing.T) {
	child := NewTextInput("with-error", "")
	box := &LabeledBox{CompID: "error-field", Title: "With error", Child: child, Help: "Helpful text.", Error: "Required."}
	box.SetBounds(image.Rect(0, 0, 200, 90))

	if got := box.descriptionText(); got != "Required." {
		t.Fatalf("descriptionText()=%q, want the error text", got)
	}

	state := &types.ApplicationState{CurrentPage: "page", Pages: map[string][]types.Component{"page": {box}}}
	tree := types.BuildSemanticsTree(state)
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	desc, ok := tree.Find("error-field/description")
	if !ok || desc.Role != semantics.RoleAlert || desc.Name != "Required." {
		t.Fatalf("description=%+v found=%v", desc, ok)
	}
}
