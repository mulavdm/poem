package types_test

import (
	"image"
	"testing"

	"go_native_gpu_gui/pkg/render/components"
	"go_native_gpu_gui/pkg/render/layout"
	"go_native_gpu_gui/pkg/render/types"
)

func TestBuildSemanticsTreePreservesContainerHierarchy(t *testing.T) {
	button := components.NewButton("save", "Save", nil)
	button.Rect = image.Rect(0, 250, 100, 286)
	hidden := components.NewButton("hidden", "Hidden", nil)
	hidden.Rect = image.Rect(0, 400, 100, 436)
	stack := &layout.FlexBox{CompID: "stack", Children: []types.Component{button, hidden}}
	scroll := components.NewScrollView("viewport", stack)
	scroll.Rect = image.Rect(0, 0, 320, 200)
	scroll.CurrentScrollY = 100
	state := &types.ApplicationState{
		ApplicationName: "Demo",
		CurrentPage:     "main",
		Pages:           map[string][]types.Component{"main": {scroll}},
	}
	tree := types.BuildSemanticsTree(state)
	if len(tree.Root.Children) != 1 || tree.Root.Children[0].ID != "viewport" {
		t.Fatalf("root semantic children=%+v", tree.Root.Children)
	}
	if len(tree.Root.Children[0].Children) != 2 || tree.Root.Children[0].Children[0].ID != "save" {
		t.Fatalf("scroll semantic descendants=%+v", tree.Root.Children[0].Children)
	}
	if got := tree.Root.Children[0].Children[0].Bounds; got != image.Rect(0, 150, 100, 186) {
		t.Fatalf("visible child bounds=%v", got)
	}
	if !tree.Root.Children[0].Children[1].State.Offscreen {
		t.Fatalf("clipped child was not marked offscreen: %+v", tree.Root.Children[0].Children[1])
	}
}
