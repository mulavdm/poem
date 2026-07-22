// Package designlint validates semantic POEM trees in tests and development.
package designlint

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/render/theme"
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
			if n.Semantic.Running && !containsProgress(n.Children) {
				add("UI072", n.Semantic.ID, "running section requires a visible and accessible loading state")
			}
			if n.Semantic.Empty && !containsEmptyFeedback(n.Children) {
				add("UI073", n.Semantic.ID, "empty section requires visible guidance or recovery")
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
			visitNodes(n.Detail)
		case app.MapViewportNode:
			checkMeta(n.Semantic)
			if strings.TrimSpace(n.Semantic.Name) == "" {
				add("UI018", n.Semantic.ID, "map viewport requires an accessible name")
			}
			if !n.Valid() {
				add("UI041", n.Semantic.ID, "map viewport contains an invalid source, camera, feature, or limit")
			}
			if mapStyleUsesRawColor(n.Style) {
				add("UI042", n.Semantic.ID, "map viewport uses a raw color instead of a semantic color token")
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
		case app.OverlayNode:
			if n.Base != nil {
				visit(n.Base)
			}
			for _, layer := range n.Layers {
				if layer.Content != nil {
					visit(layer.Content)
				}
			}
		}
	}
	visit(root)
	return diagnostics
}

func mapStyleUsesRawColor(styles app.MapStyleSet) bool {
	for _, style := range [...]app.MapStyle{styles.House, styles.Windows, styles.Android, styles.Web} {
		colors := style.Colors
		for _, value := range [...]string{colors.Land, colors.Water, colors.Road, colors.Building, colors.Label, colors.Route, colors.Traffic} {
			if strings.HasPrefix(strings.TrimSpace(value), "#") {
				return true
			}
		}
	}
	return false
}

// LintTheme validates contrast between resolved semantic token pairs. It is
// deliberately separate from Lint because contrast only becomes meaningful
// after the design system resolves a platform, mode, and accessibility state.
func LintTheme(value theme.Theme) []Diagnostic {
	type pair struct {
		name                   string
		foreground, background color.RGBA
		minimum                float64
	}
	colors := value.Colors
	pairs := []pair{
		{"text/background", colors.Text, colors.Background, 4.5},
		{"text/surface", colors.Text, colors.Surface, 4.5},
		{"text/raised surface", colors.Text, colors.SurfaceRaised, 4.5},
		{"text/sunken surface", colors.Text, colors.SurfaceSunken, 4.5},
		{"muted text/background", colors.TextMuted, colors.Background, 4.5},
		{"accent foreground", colors.OnAccent, colors.Accent, 4.5},
		{"danger foreground", colors.OnDanger, colors.Danger, 4.5},
		{"warning foreground", colors.OnWarning, colors.Warning, 4.5},
		{"success foreground", colors.OnSuccess, colors.Success, 4.5},
		{"focus/background", colors.Focus, colors.Background, 3},
		{"focus/surface", colors.Focus, colors.Surface, 3},
		{"strong border/surface", colors.BorderStrong, colors.Surface, 3},
	}
	var diagnostics []Diagnostic
	for _, candidate := range pairs {
		ratio := theme.ContrastRatio(candidate.foreground, candidate.background)
		if candidate.foreground.A == 0 || candidate.background.A == 0 || ratio < candidate.minimum {
			diagnostics = append(diagnostics, Diagnostic{
				Code:   "UI043",
				NodeID: value.Name,
				Message: fmt.Sprintf("semantic token pair %s has contrast %.2f; required %.1f",
					candidate.name, ratio, candidate.minimum),
			})
		}
	}
	return diagnostics
}

func containsProgress(nodes []app.Node) bool {
	for _, node := range nodes {
		switch n := node.(type) {
		case app.ProgressNode:
			return strings.TrimSpace(n.Semantic.Name) != ""
		case app.ContainerNode:
			if containsProgress(n.Children) {
				return true
			}
		}
	}
	return false
}

func containsEmptyFeedback(nodes []app.Node) bool {
	for _, node := range nodes {
		switch n := node.(type) {
		case app.StatusNode:
			return strings.TrimSpace(n.Text) != ""
		case app.LabelNode:
			return strings.TrimSpace(n.Text) != ""
		case app.ContainerNode:
			if containsEmptyFeedback(n.Children) {
				return true
			}
		}
	}
	return false
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
