// Package semantics describes an accessibility tree independently of any OS API.
package semantics

import (
	"fmt"
	"image"
)

type Role string

const (
	RoleApplication  Role = "application"
	RoleWindow       Role = "window"
	RoleGroup        Role = "group"
	RoleText         Role = "text"
	RoleButton       Role = "button"
	RoleTextField    Role = "text-field"
	RoleCheckBox     Role = "checkbox"
	RoleRadioButton  Role = "radio-button"
	RoleSwitch       Role = "switch"
	RoleSlider       Role = "slider"
	RoleProgressBar  Role = "progress-bar"
	RoleList         Role = "list"
	RoleListBox      Role = "list-box"
	RoleListItem     Role = "list-item"
	RoleTable        Role = "table"
	RoleTree         Role = "tree"
	RoleTreeItem     Role = "tree-item"
	RoleTab          Role = "tab"
	RoleDialog       Role = "dialog"
	RoleMenu         Role = "menu"
	RoleMenuItem     Role = "menu-item"
	RoleComboBox     Role = "combo-box"
	RoleOption       Role = "option"
	RoleSeparator    Role = "separator"
	RoleStatus       Role = "status"
	RoleTabList      Role = "tab-list"
	RoleRow          Role = "row"
	RoleCell         Role = "cell"
	RoleColumnHeader Role = "column-header"
	RoleToolBar      Role = "toolbar"
	RoleToolTip      Role = "tooltip"
	RoleAlert        Role = "alert"
	RoleLink         Role = "link"
	RoleNavigation   Role = "navigation"
	RoleDatePicker   Role = "date-picker"
	RoleGrid         Role = "grid"
	RoleGridCell     Role = "grid-cell"
)

type Action string

const (
	ActionFocus        Action = "focus"
	ActionInvoke       Action = "invoke"
	ActionSetValue     Action = "set-value"
	ActionSetSelection Action = "set-selection"
	ActionSetScroll    Action = "set-scroll-percent"
	ActionIncrement    Action = "increment"
	ActionDecrement    Action = "decrement"
	ActionExpand       Action = "expand"
	ActionCollapse     Action = "collapse"
	ActionSelect       Action = "select"
)

type State struct {
	Disabled, Focused, Selected, Checked, Expanded, ReadOnly, Required, Invalid, Password, Offscreen bool
}

type RangeValue struct {
	Minimum, Maximum float64
	SmallChange      float64
	LargeChange      float64
}

type TextValue struct {
	SelectionStart int
	SelectionEnd   int
	Multiline      bool
}

type CollectionValue struct {
	Selectable        bool
	CanSelectMultiple bool
	SelectionRequired bool
}

type GridValue struct {
	Rows    int
	Columns int
}

type GridItemValue struct {
	Row, Column         int
	RowSpan, ColumnSpan int
}

type ScrollValue struct {
	HorizontallyScrollable bool
	VerticallyScrollable   bool
	HorizontalPercent      float64
	VerticalPercent        float64
	HorizontalViewSize     float64
	VerticalViewSize       float64
}

// Relationships connects semantic nodes without imposing platform-specific
// accessibility concepts on components. IDs refer to nodes in the same tree.
type Relationships struct {
	LabeledBy   []string
	DescribedBy []string
	Controls    []string
	FlowsTo     []string
}

type Node struct {
	ID          string
	Role        Role
	Name        string
	Description string
	AccessKey   string
	Value       string
	Bounds      image.Rectangle
	State       State
	Range       *RangeValue
	Text        *TextValue
	Collection  *CollectionValue
	Grid        *GridValue
	GridItem    *GridItemValue
	Scroll      *ScrollValue
	Relations   Relationships
	Actions     []Action
	Children    []Node
}

type Tree struct {
	Revision uint64
	Root     Node
}

func (t Tree) Validate() error {
	seen := make(map[string]struct{})
	var nodes []Node
	var visit func(Node) error
	visit = func(node Node) error {
		if node.ID == "" {
			return fmt.Errorf("semantic node has empty id")
		}
		if _, exists := seen[node.ID]; exists {
			return fmt.Errorf("duplicate semantic id %q", node.ID)
		}
		seen[node.ID] = struct{}{}
		nodes = append(nodes, node)
		if node.Role == "" {
			return fmt.Errorf("semantic node %q has no role", node.ID)
		}
		if node.Text != nil {
			length := len([]rune(node.Value))
			if node.Text.SelectionStart < 0 || node.Text.SelectionEnd < node.Text.SelectionStart || node.Text.SelectionEnd > length {
				return fmt.Errorf("semantic text node %q has invalid selection %d:%d for length %d", node.ID, node.Text.SelectionStart, node.Text.SelectionEnd, length)
			}
		}
		if node.Grid != nil {
			if node.Grid.Rows < 0 || node.Grid.Columns < 0 || node.Grid.Rows > 1_000_000 || node.Grid.Columns > 100_000 {
				return fmt.Errorf("semantic grid %q has invalid dimensions %dx%d", node.ID, node.Grid.Rows, node.Grid.Columns)
			}
		}
		if node.GridItem != nil {
			if node.GridItem.Row < 0 || node.GridItem.Column < 0 || node.GridItem.RowSpan <= 0 || node.GridItem.ColumnSpan <= 0 {
				return fmt.Errorf("semantic grid item %q has invalid coordinates or span", node.ID)
			}
		}
		if node.Scroll != nil {
			validPercent := func(value float64) bool { return value == -1 || value >= 0 && value <= 100 }
			validView := func(value float64) bool { return value >= 0 && value <= 100 }
			if !validPercent(node.Scroll.HorizontalPercent) || !validPercent(node.Scroll.VerticalPercent) ||
				!validView(node.Scroll.HorizontalViewSize) || !validView(node.Scroll.VerticalViewSize) {
				return fmt.Errorf("semantic scroll node %q has invalid percentages", node.ID)
			}
			if !node.Scroll.HorizontallyScrollable && node.Scroll.HorizontalPercent != -1 ||
				!node.Scroll.VerticallyScrollable && node.Scroll.VerticalPercent != -1 {
				return fmt.Errorf("semantic scroll node %q reports a percent for a non-scrollable axis", node.ID)
			}
		}
		for _, child := range node.Children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(t.Root); err != nil {
		return err
	}
	for _, node := range nodes {
		groups := [][]string{node.Relations.LabeledBy, node.Relations.DescribedBy, node.Relations.Controls, node.Relations.FlowsTo}
		for _, ids := range groups {
			if len(ids) > 256 {
				return fmt.Errorf("semantic node %q has too many relationships", node.ID)
			}
			related := make(map[string]struct{}, len(ids))
			for _, id := range ids {
				if id == node.ID {
					return fmt.Errorf("semantic node %q relates to itself", node.ID)
				}
				if _, ok := seen[id]; !ok {
					return fmt.Errorf("semantic node %q references missing node %q", node.ID, id)
				}
				if _, duplicate := related[id]; duplicate {
					return fmt.Errorf("semantic node %q repeats relationship %q", node.ID, id)
				}
				related[id] = struct{}{}
			}
		}
	}
	return nil
}

func (t Tree) Find(id string) (Node, bool) {
	var found Node
	var ok bool
	var visit func(Node)
	visit = func(node Node) {
		if ok {
			return
		}
		if node.ID == id {
			found, ok = node, true
			return
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(t.Root)
	return found, ok
}
