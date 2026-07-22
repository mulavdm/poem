package designlint

import (
	"fmt"
	"image"

	"github.com/mulavdm/poem/pkg/render/components"
	renderlayout "github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/types"
)

type layoutScrollPath struct {
	viewport image.Rectangle
	contentH int
}

// LintLayout validates geometry that only exists after the native component
// tree has been laid out. It complements Lint; it does not change the existing
// semantic-tree contract.
func LintLayout(root types.Component, viewport image.Rectangle, state *types.ApplicationState) []Diagnostic {
	if root == nil || viewport.Empty() {
		return nil
	}

	var diagnostics []Diagnostic
	var visit func(types.Component, []layoutScrollPath)
	visit = func(component types.Component, scrolls []layoutScrollPath) {
		if component == nil {
			return
		}
		bounds := component.Bounds()
		if textBearing(component) {
			measurement := types.MeasureComponent(component, image.Pt(bounds.Dx(), 0), state)
			required := image.Pt(measurement.Min.X, measurement.Preferred.Y)
			if bounds.Dx() < required.X || bounds.Dy() < required.Y {
				diagnostics = append(diagnostics, Diagnostic{
					Code:    "UI023",
					NodeID:  component.ID(),
					Message: fmt.Sprintf("intrinsic text minimum %dx%d exceeds rendered bounds %dx%d", required.X, required.Y, bounds.Dx(), bounds.Dy()),
				})
			}
		}

		if component.Focusable() && !rectangleContained(viewport, bounds) && !reachableThroughScroll(bounds, scrolls) {
			diagnostics = append(diagnostics, Diagnostic{
				Code:    "UI102",
				NodeID:  component.ID(),
				Message: "actionable component is outside the reachable viewport and has no valid scroll path",
			})
		}
		if flex, ok := component.(*renderlayout.FlexBox); ok {
			diagnostics = append(diagnostics, lintFlexFocusOrder(flex)...)
		}

		nextScrolls := scrolls
		if scroll, ok := component.(*components.ScrollView); ok {
			nextScrolls = append(append([]layoutScrollPath(nil), scrolls...), layoutScrollPath{viewport: scroll.Bounds(), contentH: scroll.ContentH})
		}
		if parent, ok := component.(types.ChildComponent); ok {
			for _, child := range parent.ChildComponents() {
				visit(child, nextScrolls)
			}
		}
	}
	visit(root, nil)
	return diagnostics
}

func lintFlexFocusOrder(flex *renderlayout.FlexBox) []Diagnostic {
	type focusPosition struct {
		id     string
		bounds image.Rectangle
	}
	positions := make([]focusPosition, 0, len(flex.Children))
	for _, child := range flex.Children {
		if focusable := firstFocusable(child); focusable != nil {
			positions = append(positions, focusPosition{id: focusable.ID(), bounds: focusable.Bounds()})
		}
	}
	for index := 1; index < len(positions); index++ {
		previous, current := positions[index-1], positions[index]
		outOfOrder := false
		if flex.Direction == renderlayout.Vertical {
			outOfOrder = current.bounds.Min.Y < previous.bounds.Min.Y
		} else if flex.Wrap && current.bounds.Min.Y != previous.bounds.Min.Y {
			outOfOrder = current.bounds.Min.Y < previous.bounds.Min.Y
		} else {
			outOfOrder = current.bounds.Min.X < previous.bounds.Min.X
		}
		if outOfOrder {
			return []Diagnostic{{
				Code:    "UI103",
				NodeID:  current.id,
				Message: "keyboard focus order does not match the visual order of sibling controls",
			}}
		}
	}
	return nil
}

func firstFocusable(component types.Component) types.Component {
	if component == nil {
		return nil
	}
	if component.Focusable() {
		return component
	}
	if parent, ok := component.(types.ChildComponent); ok {
		for _, child := range parent.ChildComponents() {
			if focusable := firstFocusable(child); focusable != nil {
				return focusable
			}
		}
	}
	return nil
}

func textBearing(component types.Component) bool {
	switch component.(type) {
	case *components.Button, *components.Badge, *components.Label, *components.DynamicLabel:
		return true
	default:
		return false
	}
}

func rectangleContained(outer, inner image.Rectangle) bool {
	return !inner.Empty() && inner.Min.X >= outer.Min.X && inner.Min.Y >= outer.Min.Y && inner.Max.X <= outer.Max.X && inner.Max.Y <= outer.Max.Y
}

func reachableThroughScroll(bounds image.Rectangle, scrolls []layoutScrollPath) bool {
	for index := len(scrolls) - 1; index >= 0; index-- {
		scroll := scrolls[index]
		if scroll.contentH <= scroll.viewport.Dy() || scroll.viewport.Empty() {
			continue
		}
		contentBottom := scroll.viewport.Min.Y + scroll.contentH
		if bounds.Min.X >= scroll.viewport.Min.X && bounds.Max.X <= scroll.viewport.Max.X &&
			bounds.Min.Y >= scroll.viewport.Min.Y && bounds.Max.Y <= contentBottom {
			return true
		}
	}
	return false
}
