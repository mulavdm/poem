package semantics

import "testing"

func TestTreeValidationRejectsDuplicateIDs(t *testing.T) {
	tree := Tree{Root: Node{ID: "root", Role: RoleApplication, Children: []Node{
		{ID: "save", Role: RoleButton}, {ID: "save", Role: RoleButton},
	}}}
	if err := tree.Validate(); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestTreeFind(t *testing.T) {
	tree := Tree{Root: Node{ID: "root", Role: RoleApplication, Children: []Node{{ID: "name", Role: RoleTextField}}}}
	if node, ok := tree.Find("name"); !ok || node.Role != RoleTextField {
		t.Fatal("semantic node not found")
	}
}

func TestTreeValidationRejectsInvalidTextSelection(t *testing.T) {
	tree := Tree{Root: Node{ID: "root", Role: RoleApplication, Children: []Node{{
		ID: "field", Role: RoleTextField, Value: "abc", Text: &TextValue{SelectionStart: 1, SelectionEnd: 4},
	}}}}
	if err := tree.Validate(); err == nil {
		t.Fatal("invalid text selection was accepted")
	}
}

func TestTreeValidationRejectsInvalidGridMetadata(t *testing.T) {
	for _, child := range []Node{
		{ID: "grid", Role: RoleGrid, Grid: &GridValue{Rows: -1, Columns: 2}},
		{ID: "cell", Role: RoleGridCell, GridItem: &GridItemValue{Row: 0, Column: 0}},
	} {
		tree := Tree{Root: Node{ID: "root", Role: RoleApplication, Children: []Node{child}}}
		if err := tree.Validate(); err == nil {
			t.Fatalf("invalid grid metadata was accepted: %+v", child)
		}
	}
}

func TestTreeValidationAcceptsRelationships(t *testing.T) {
	tree := Tree{Root: Node{ID: "root", Role: RoleApplication, Children: []Node{
		{ID: "label", Role: RoleText},
		{ID: "field", Role: RoleTextField, Relations: Relationships{LabeledBy: []string{"label"}}},
	}}}
	if err := tree.Validate(); err != nil {
		t.Fatalf("valid relationship rejected: %v", err)
	}
}

func TestTreeValidationRejectsInvalidRelationships(t *testing.T) {
	for _, relations := range []Relationships{
		{LabeledBy: []string{"missing"}},
		{Controls: []string{"field"}},
		{FlowsTo: []string{"label", "label"}},
	} {
		tree := Tree{Root: Node{ID: "root", Role: RoleApplication, Children: []Node{
			{ID: "label", Role: RoleText},
			{ID: "field", Role: RoleTextField, Relations: relations},
		}}}
		if err := tree.Validate(); err == nil {
			t.Fatalf("invalid relationships accepted: %+v", relations)
		}
	}
}
