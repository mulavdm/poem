package widget

import (
	"html/template"
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/web/components"
	"github.com/mulavdm/poem/pkg/web/components/action"
)

func TestTabsAndModalExposeAccessibleRelationships(t *testing.T) {
	tabs := string(Tabs{Label: "Demo", Items: []TabItem{{ID: "one", Label: "One", Text: "Body"}}}.HTML())
	for _, wanted := range []string{`aria-controls="one"`, `aria-labelledby="one-tab"`, `tabindex="0"`} {
		if !strings.Contains(tabs, wanted) {
			t.Fatalf("missing %q in %q", wanted, tabs)
		}
	}
	attrs := components.Attrs{"data-original": "yes"}
	modal := string(Modal{ID: "dialog", Title: "Dialog", Content: "Body", Trigger: action.Button{Text: "Open", Attributes: attrs}}.HTML())
	if !strings.Contains(modal, `data-poem-modal-open="dialog"`) {
		t.Fatalf("missing modal trigger in %q", modal)
	}
	if _, exists := attrs["data-poem-modal-open"]; exists {
		t.Fatal("Modal mutated caller attributes")
	}
}

func TestFormTabsPostsSelectionAndRendersOnlySelectedPanel(t *testing.T) {
	html := string(FormTabs{
		Label:     "Views",
		Name:      "trellis_field_root",
		Items:     []TabItem{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}},
		Selected:  "two",
		PanelHTML: template.HTML(`<p>panel two</p>`),
	}.HTML())
	for _, wanted := range []string{`role="tablist"`, `type="submit"`, `name="trellis_field_root"`, `value="one"`, `value="two"`, `role="tabpanel"`, "panel two"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
	if strings.Count(html, `aria-selected="true"`) != 1 {
		t.Fatalf("expected exactly one selected tab: %q", html)
	}
	if !strings.Contains(html, `value="two" aria-selected="true"`) {
		t.Fatalf("selected tab not marked: %q", html)
	}
}
