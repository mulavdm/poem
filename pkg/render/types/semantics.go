package types

import (
	"reflect"

	"github.com/mulavdm/poem/pkg/render/semantics"
)

// SemanticComponent is the incremental accessibility contract. It remains
// optional for legacy controls while POEM 2.0 migrates the component catalog.
type SemanticComponent interface {
	Semantics(state *ApplicationState) semantics.Node
}

// SemanticActionComponent handles actions addressed to semantic descendants
// that are not separate render components, such as menu items or table rows.
type SemanticActionComponent interface {
	PerformSemanticAction(targetID string, action semantics.Action, value string, state *ApplicationState) bool
}

// RealtimeViewportEventComponent consumes an opaque event emitted by a native
// ABI-v3 viewport. POEM routes by component ID and never interprets payload.
type RealtimeViewportEventComponent interface {
	OnRealtimeViewportEvent(payload []byte, state *ApplicationState) bool
}

// SemanticChildTransformer maps descendant semantic geometry through a
// container's coordinate space, such as scrolling and clipping.
type SemanticChildTransformer interface {
	TransformSemanticChild(node *semantics.Node, state *ApplicationState)
}

func BuildSemanticsTree(state *ApplicationState) semantics.Tree {
	root := semantics.Node{ID: "poem.application", Role: semantics.RoleApplication, Name: "POEM application"}
	if state == nil {
		return semantics.Tree{Root: root}
	}
	if state.ApplicationName != "" {
		root.Name = state.ApplicationName
	}
	if page, ok := state.Pages[state.CurrentPage]; ok {
		for _, component := range page {
			root.Children = append(root.Children, semanticNodesForComponent(component, state)...)
		}
	}
	if state.Overlays != nil {
		for _, overlay := range state.Overlays.Snapshot() {
			root.Children = append(root.Children, semanticNodesForComponent(overlay.Component, state)...)
		}
	}
	if reflect.DeepEqual(root, state.SemanticTree.Root) {
		return state.SemanticTree
	}
	state.SemanticsRevision++
	state.SemanticTree = semantics.Tree{Revision: state.SemanticsRevision, Root: root}
	return state.SemanticTree
}

func semanticNodesForComponent(component Component, state *ApplicationState) []semantics.Node {
	if component == nil {
		return nil
	}
	children := make([]semantics.Node, 0)
	if container, ok := component.(ChildComponent); ok {
		for _, child := range container.ChildComponents() {
			children = append(children, semanticNodesForComponent(child, state)...)
		}
	}
	if transformer, ok := component.(SemanticChildTransformer); ok {
		for index := range children {
			transformer.TransformSemanticChild(&children[index], state)
		}
	}
	if semantic, ok := component.(SemanticComponent); ok {
		node := semantic.Semantics(state)
		if node.ID == "" {
			return children
		}
		node.Children = append(node.Children, children...)
		return []semantics.Node{node}
	}
	return children
}
