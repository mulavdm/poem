// Package designlint validates semantic POEM trees in tests and development.
package designlint

import (
	"fmt"
	"strings"

	"github.com/mulavdm/poem/pkg/app"
)

// Diagnostic is one guide-coded design contract violation.
type Diagnostic struct {
	Code    string
	NodeID  string
	Message string
}

func (d Diagnostic) Error() string {
	if d.NodeID == "" {
		return fmt.Sprintf("%s: %s", d.Code, d.Message)
	}
	return fmt.Sprintf("%s [%s]: %s", d.Code, d.NodeID, d.Message)
}

// Lint checks a semantic tree without rendering it.
func Lint(root app.Node, commands []app.Command) []Diagnostic {
	commandByID := make(map[string]app.Command, len(commands))
	for _, command := range commands {
		commandByID[command.ID] = command
	}
	ids := map[string]bool{}
	var diagnostics []Diagnostic
	add := func(code, id, message string) {
		diagnostics = append(diagnostics, Diagnostic{Code: code, NodeID: id, Message: message})
	}
	var visit func(app.Node)
	checkMeta := func(meta app.Semantic) {
		if strings.TrimSpace(meta.ID) == "" {
			add("UI001", "", "semantic node requires a stable ID")
		} else if ids[meta.ID] {
			add("UI002", meta.ID, "stable ID is duplicated")
		} else {
			ids[meta.ID] = true
		}
	}
	visitNodes := func(nodes []app.Node) {
		for _, node := range nodes {
			visit(node)
		}
	}
	visit = func(node app.Node) {
		switch n := node.(type) {
		case app.ActionNode:
			checkMeta(n.Semantic)
			command := commandByID[n.Command]
			label := n.Label
			if label == "" {
				label = command.Label
			}
			if strings.TrimSpace(label) == "" {
				add("UI011", n.Semantic.ID, "action, including icon-only action, requires an accessible label")
			}
			if n.Command != "" && command.ID == "" {
				add("UI021", n.Semantic.ID, "action references an unknown command")
			}
			if (n.Semantic.Running || command.Running) && n.Semantic.Name == "" && n.Semantic.Description == "" {
				add("UI071", n.Semantic.ID, "running command requires visible or accessible feedback")
			}
			if command.Destructive && !command.Recoverable {
				add("UI081", n.Semantic.ID, "destructive command requires recovery or confirmation")
			}
		case app.LabelNode:
			checkMeta(n.Semantic)
		case app.TextFieldNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Label) == "" {
				add("UI012", n.Semantic.ID, "text field requires a persistent label")
			}
		case app.ToggleFieldNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Label) == "" {
				add("UI012", n.Semantic.ID, "toggle requires a label")
			}
		case app.ChoiceFieldNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Label) == "" {
				add("UI012", n.Semantic.ID, "choice field requires a label")
			}
		case app.RangeFieldNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Label) == "" {
				add("UI012", n.Semantic.ID, "range field requires a label")
			}
		case app.CollectionNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Caption) == "" && strings.TrimSpace(n.Semantic.Name) == "" {
				add("UI013", n.Semantic.ID, "collection requires an accessible caption")
			}
			for _, item := range n.Items {
				if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Title) == "" {
					add("UI013", n.Semantic.ID, "interactive collection items require stable IDs and titles")
				}
				for _, action := range item.Actions {
					visit(action)
				}
			}
		case app.NavigationNode:
			checkMeta(n.Semantic)
			for _, destination := range n.Destinations {
				if destination.ID == "" || destination.Label == "" {
					add("UI014", n.Semantic.ID, "every destination requires an ID and label")
				}
				visitNodes(destination.Content)
			}
		case app.StatusNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Text) == "" {
				add("UI015", n.Semantic.ID, "status requires text")
			}
		case app.ProgressNode:
			checkMeta(n.Semantic)
			if n.Semantic.Name == "" {
				add("UI015", n.Semantic.ID, "progress requires an accessible name")
			}
		case app.DisclosureNode:
			checkMeta(n.Semantic)
			for _, section := range n.Sections {
				visitNodes(section.Content)
			}
		case app.DialogNode:
			checkMeta(n.Semantic)
			if n.Title == "" {
				add("UI016", n.Semantic.ID, "dialog requires a title")
			}
			visitNodes(n.Content)
		case app.FormNode:
			checkMeta(n.Semantic)
			visitNodes(n.Children)
		case app.SectionNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Title) == "" {
				add("UI017", n.Semantic.ID, "section requires a visible title")
			}
			visitNodes(n.Children)
		case app.WorkspaceNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Title) == "" {
				add("UI095", n.Semantic.ID, "workspace requires a title")
			}
			if n.Navigation != nil {
				visit(n.Navigation)
			}
			if n.Status != nil {
				visit(n.Status)
			}
			visitNodes(n.Header)
			visitNodes(n.Content)
			visitNodes(n.Tools)
		case app.MapViewportNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Semantic.Name) == "" {
				add("UI018", n.Semantic.ID, "map viewport requires an accessible name")
			}
			if !n.Valid() {
				add("UI041", n.Semantic.ID, "map viewport contains an invalid source, camera, feature, or limit")
			}
		case app.AdaptiveNode:
			checkMeta(n.Semantic)
			base := ids
			for _, branch := range [][]app.Node{n.Compact, n.Medium, n.Expanded, n.UltraWide} {
				ids = make(map[string]bool, len(base))
				for id := range base {
					ids[id] = true
				}
				visitNodes(branch)
			}
			ids = base
		case app.IdentityNode:
			checkMeta(n.Semantic)
			visit(n.Child)
		case app.ContainerNode:
			if n.Gap > 4 || n.Padding > 4 {
				add("UI031", "", "application node uses raw spacing instead of semantic spacing")
			}
			visitNodes(n.Children)
		case app.TabsNode:
			for _, tab := range n.Tabs {
				visitNodes(tab.Content)
			}
		case app.AccordionNode:
			for _, section := range n.Sections {
				visitNodes(section.Content)
			}
		case app.ModalNode:
			visitNodes(n.Content)
		}
	}
	visit(root)
	return diagnostics
}

// AssertClean fails a test when the tree contains a non-allowlisted code.
func AssertClean(tb interface {
	Helper()
	Errorf(string, ...any)
}, root app.Node, commands []app.Command, allowCodes ...string) {
	tb.Helper()
	allow := map[string]bool{}
	for _, code := range allowCodes {
		allow[code] = true
	}
	for _, diagnostic := range Lint(root, commands) {
		if !allow[diagnostic.Code] {
			tb.Errorf("design lint: %s", diagnostic.Error())
		}
	}
}
