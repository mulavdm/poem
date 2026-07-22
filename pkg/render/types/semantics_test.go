package types_test

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/components"
	"github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/types"
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

func TestBuildSemanticsTreeDoesNotDuplicateWorkspaceChildren(t *testing.T) {
	button := components.NewButton("search", "Search", nil)
	tools := components.NewScrollView("tools-scroll", button)
	workspace := &components.Workspace{
		CompID:  "workspace",
		Content: components.NewPanel("map"),
		Tools:   tools,
	}
	state := &types.ApplicationState{
		ApplicationName: "MAPPS",
		CurrentPage:     "main",
		Pages:           map[string][]types.Component{"main": {workspace}},
	}
	tree := types.BuildSemanticsTree(state)
	if err := tree.Validate(); err != nil {
		t.Fatalf("workspace semantic tree did not validate: %v", err)
	}
	workspaceNode, found := tree.Find("workspace")
	if !found {
		t.Fatal("workspace semantic node was not published")
	}
	if len(workspaceNode.Children) != 2 || workspaceNode.Children[1].ID != "tools-scroll" {
		t.Fatalf("workspace semantic children=%+v", workspaceNode.Children)
	}
	if len(workspaceNode.Children[1].Children) != 1 || workspaceNode.Children[1].Children[0].ID != "search" {
		t.Fatalf("tools semantic children=%+v", workspaceNode.Children[1].Children)
	}
}

func TestBuildSemanticsTreeIncludesUltraWideWorkspaceDetail(t *testing.T) {
	detail := components.NewScrollView("detail-scroll", components.NewLabel("detail-label", "Route details"))
	workspace := &components.Workspace{
		CompID:  "workspace",
		Content: components.NewPanel("map"),
		Tools:   components.NewScrollView("tools-scroll", components.NewButton("search", "Search", nil)),
		Detail:  detail,
	}
	workspace.SetBounds(image.Rect(0, 0, 1600, 900))
	state := &types.ApplicationState{CurrentPage: "main", Pages: map[string][]types.Component{"main": {workspace}}}
	tree := types.BuildSemanticsTree(state)
	if err := tree.Validate(); err != nil {
		t.Fatalf("ultra-wide semantic tree did not validate: %v", err)
	}
	if _, found := tree.Find("detail-scroll"); !found {
		t.Fatal("ultra-wide detail surface was not published")
	}
}
