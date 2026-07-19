package designlint

import (
	"testing"

	"github.com/mulavdm/poem/pkg/app"
)

func TestLintAcceptsSemanticTree(t *testing.T) {
	commands := []app.Command{{ID: "search", Label: "Search", Invoke: app.Msg{Name: "search"}, Enabled: true}}
	root := app.FormNode{Semantic: app.Semantic{ID: "search-form", Name: "Search"}, Children: []app.Node{
		app.TextFieldNode{Semantic: app.Semantic{ID: "query", Enabled: true}, Label: "Query"},
		app.ActionNode{Semantic: app.Semantic{ID: "submit", Enabled: true}, Command: "search"},
	}}
	AssertClean(t, root, commands)
}

func TestLintFindsMissingLabelAndUnsafeDestruction(t *testing.T) {
	commands := []app.Command{{ID: "delete", Enabled: true, Destructive: true}}
	diagnostics := Lint(app.ActionNode{Semantic: app.Semantic{ID: "delete", Enabled: true}, Command: "delete", Icon: app.IconRemove}, commands)
	want := map[string]bool{"UI011": true, "UI081": true}
	for _, diagnostic := range diagnostics {
		delete(want, diagnostic.Code)
	}
	for code := range want {
		t.Errorf("missing diagnostic %s: %#v", code, diagnostics)
	}
}
