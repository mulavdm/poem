package designlint

import (
	"image/color"
	"testing"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/render/theme"
)

func TestLintAcceptsSemanticTree(t *testing.T) {
	commands := []app.Command{{ID: "search", Label: "Search", Invoke: app.Msg{Name: "search"}, Enabled: true}}
	root := app.FormNode{Semantic: app.Semantic{ID: "search-form", Name: "Search"}, Children: []app.Node{
		app.TextFieldNode{Semantic: app.Semantic{ID: "query", Enabled: true}, Label: "Query"},
		app.ActionNode{Semantic: app.Semantic{ID: "submit", Enabled: true}, Command: "search"},
	}}
	AssertClean(t, root, commands)
}

func TestLintRejectsRawMapColor(t *testing.T) {
	style := app.MapStyle{ID: "raw", Colors: app.MapSemanticColors{Land: "#112233"}, LabelDensity: 1, TerrainScale: 1}
	root := app.MapViewportNode{
		Semantic: app.Semantic{ID: "map", Name: "Map"},
		Source:   app.MapSource{ID: "source", ProviderID: "provider", MinZoom: 0, MaxZoom: 20},
		Camera:   app.MapCamera{Latitude: 52, Longitude: 4, Zoom: 10},
		Style:    app.MapStyleSet{House: style},
		MaxZoom:  20,
		MaxPitch: 60,
	}
	if !hasCode(Lint(root, nil), "UI042") {
		t.Fatal("expected UI042 for a raw map color")
	}
}

func TestLintThemeRejectsLowContrastTokens(t *testing.T) {
	value := theme.ModernLight()
	value.Colors.Text = color.RGBA{240, 240, 240, 255}
	if !hasCode(LintTheme(value), "UI043") {
		t.Fatal("expected UI043 for a low-contrast semantic token pair")
	}
}

func hasCode(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
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

func TestLintRequiresDeclaredLoadingAndEmptyFeedback(t *testing.T) {
	root := app.Container(app.Vertical, 0,
		app.SectionNode{Semantic: app.Semantic{ID: "loading", Running: true}, Title: "Loading"},
		app.SectionNode{Semantic: app.Semantic{ID: "empty", Empty: true}, Title: "Empty"},
	)
	diagnostics := Lint(root, nil)
	want := map[string]bool{"UI072": true, "UI073": true}
	for _, diagnostic := range diagnostics {
		delete(want, diagnostic.Code)
	}
	for code := range want {
		t.Errorf("missing diagnostic %s: %#v", code, diagnostics)
	}
}

func TestLintAcceptsVisibleLoadingAndEmptyFeedback(t *testing.T) {
	root := app.Container(app.Vertical, 0,
		app.SectionNode{Semantic: app.Semantic{ID: "loading", Running: true}, Title: "Loading", Children: []app.Node{
			app.ProgressNode{Semantic: app.Semantic{ID: "progress", Name: "Loading local data"}, Indeterminate: true},
		}},
		app.SectionNode{Semantic: app.Semantic{ID: "empty", Empty: true}, Title: "Empty", Children: []app.Node{
			app.StatusNode{Semantic: app.Semantic{ID: "empty-status"}, Text: "Try a broader search"},
		}},
	)
	for _, diagnostic := range Lint(root, nil) {
		if diagnostic.Code == "UI072" || diagnostic.Code == "UI073" {
			t.Fatalf("unexpected state diagnostic: %s", diagnostic.Error())
		}
	}
}
