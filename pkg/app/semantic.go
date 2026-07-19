package app

import "github.com/mulavdm/poem/pkg/design"

// Semantic identifies a node independently of its current adaptive branch.
type Semantic struct {
	ID          string
	Name        string
	Description string
	Enabled     bool
	ReadOnly    bool
	Running     bool
	Error       string
}

// IconID is a dependency-free semantic icon name mapped by each renderer.
type IconID string

const (
	IconNone       IconID = ""
	IconSearch     IconID = "search"
	IconRoute      IconID = "route"
	IconRefresh    IconID = "refresh"
	IconAdd        IconID = "add"
	IconRemove     IconID = "remove"
	IconSettings   IconID = "settings"
	IconMore       IconID = "more"
	IconClose      IconID = "close"
	IconMap        IconID = "map"
	IconLocation   IconID = "location"
	IconSwap       IconID = "swap"
	IconClear      IconID = "clear"
	IconDirections IconID = "directions"
	IconConnection IconID = "connection"
)

type ActionImportance uint8

const (
	ImportanceSecondary ActionImportance = iota
	ImportancePrimary
	ImportanceSubtle
)

type ActionPlacement uint8

const (
	PlacementContent ActionPlacement = iota
	PlacementToolbar
	PlacementContextual
	PlacementOverflow
)

// Command is application-owned semantic action metadata. Invoke is the
// transport-safe message dispatched by every target.
type Command struct {
	ID          string
	Label       string
	Description string
	Icon        IconID
	Invoke      Msg
	Enabled     bool
	Visible     bool
	Running     bool
	Destructive bool
	Recoverable bool
	Importance  ActionImportance
	Placement   ActionPlacement
}

// Spacing is semantic token spacing; applications never supply raw pixels.
type Spacing uint8

const (
	SpaceNone Spacing = iota
	SpaceTight
	SpaceRelated
	SpaceSection
	SpaceRegion
)

// IdentityNode preserves stable identity while delegating presentation.
type IdentityNode struct {
	Semantic Semantic
	Child    Node
}

func (IdentityNode) isNode() {}

// ActionNode invokes a state-derived command.
type ActionNode struct {
	Semantic   Semantic
	Command    string
	Label      string
	Icon       IconID
	Invoke     Msg
	Importance ActionImportance
	Placement  ActionPlacement
}

func (ActionNode) isNode() {}

// LabelNode is semantic read-only text.
type LabelNode struct {
	Semantic Semantic
	Text     string
}

func (LabelNode) isNode() {}

// TextFieldNode is a labelled text field.
type TextFieldNode struct {
	Semantic  Semantic
	Label     string
	Value     string
	Hint      string
	Multiline bool
	OnChange  Msg
}

func (TextFieldNode) isNode() {}

// ToggleFieldNode is a labelled boolean field.
type ToggleFieldNode struct {
	Semantic Semantic
	Label    string
	Value    bool
	OnChange Msg
}

func (ToggleFieldNode) isNode() {}

// ChoiceFieldNode is a labelled single-choice field.
type ChoiceFieldNode struct {
	Semantic Semantic
	Label    string
	Options  []Option
	Value    string
	OnChange Msg
}

func (ChoiceFieldNode) isNode() {}

// RangeFieldNode is a labelled numeric field.
type RangeFieldNode struct {
	Semantic Semantic
	Label    string
	Min      float64
	Max      float64
	Step     float64
	Value    float64
	OnChange Msg
}

func (RangeFieldNode) isNode() {}

// CollectionNode presents records as a list/cards or a table/grid according
// to the active window class.
type CollectionNode struct {
	Semantic Semantic
	Caption  string
	Columns  []TableColumn
	Rows     []TableRow
	Items    []CollectionItem
}

func (CollectionNode) isNode() {}

type CollectionField struct {
	Label string
	Value string
}

type CollectionItem struct {
	ID          string
	Title       string
	Description string
	Metadata    []CollectionField
	Selected    bool
	Disabled    bool
	Actions     []ActionNode
}

// Destination is one application navigation destination.
type Destination struct {
	ID      string
	Label   string
	Icon    IconID
	Content []Node
}

// NavigationNode adapts destinations to bottom, rail, or sidebar placement.
type NavigationNode struct {
	Semantic     Semantic
	Destinations []Destination
	Selected     string
	OnChange     Msg
}

func (NavigationNode) isNode() {}

// StatusNode communicates semantic application status.
type StatusNode struct {
	Semantic Semantic
	Text     string
	Variant  Variant
}

func (StatusNode) isNode() {}

// ProgressNode communicates determinate or indeterminate activity.
type ProgressNode struct {
	Semantic      Semantic
	Value         float64
	Max           float64
	Indeterminate bool
}

func (ProgressNode) isNode() {}

// DisclosureNode groups controlled disclosure sections.
type DisclosureNode struct {
	Semantic Semantic
	Sections []AccordionSection
}

func (DisclosureNode) isNode() {}

// DialogNode describes a controlled semantic dialog.
type DialogNode struct {
	Semantic Semantic
	Trigger  string
	Title    string
	Content  []Node
}

func (DialogNode) isNode() {}

// FormNode groups related fields using token spacing.
type FormNode struct {
	Semantic Semantic
	Children []Node
}

func (FormNode) isNode() {}

// SectionNode groups related content under a semantic heading. Renderers own
// its surface, spacing, and typography.
type SectionNode struct {
	Semantic    Semantic
	Title       string
	Description string
	Children    []Node
}

func (SectionNode) isNode() {}

// WorkspaceNode composes navigation, primary content, and optional tools.
type WorkspaceNode struct {
	Semantic      Semantic
	Title         string
	Subtitle      string
	HeaderActions []string
	Header        []Node
	Status        Node
	Navigation    Node
	Content       []Node
	Tools         []Node
}

func (WorkspaceNode) isNode() {}

// AdaptiveNode provides all four standard window-class branches.
type AdaptiveNode struct {
	Semantic  Semantic
	Compact   []Node
	Medium    []Node
	Expanded  []Node
	UltraWide []Node
}

func (AdaptiveNode) isNode() {}

// LowerSemantic resolves semantic nodes to the common renderer catalog. It
// preserves IDs with IdentityNode and retains all adaptive branches.
func LowerSemantic(node Node, commands []Command) Node {
	registry := make(map[string]Command, len(commands))
	for _, command := range commands {
		if command.ID != "" {
			registry[command.ID] = command
		}
	}
	return lowerSemantic(node, registry)
}

func lowerSemantic(node Node, commands map[string]Command) Node {
	wrap := func(meta Semantic, child Node) Node {
		if meta.ID == "" {
			return child
		}
		return IdentityNode{Semantic: meta, Child: child}
	}
	lowerChildren := func(nodes []Node) []Node {
		out := make([]Node, len(nodes))
		for i, child := range nodes {
			out[i] = lowerSemantic(child, commands)
		}
		return out
	}
	switch n := node.(type) {
	case ActionNode:
		label, invoke, enabled := n.Label, n.Invoke, n.Semantic.Enabled
		if command, ok := commands[n.Command]; ok {
			if label == "" {
				label = command.Label
			}
			invoke = command.Invoke
			enabled = enabled && command.Enabled && !command.Running
			if n.Icon == IconNone {
				n.Icon = command.Icon
			}
			if n.Importance == ImportanceSecondary {
				n.Importance = command.Importance
			}
			if n.Placement == PlacementContent {
				n.Placement = command.Placement
			}
		}
		n.Label, n.Invoke = label, invoke
		n.Semantic.Enabled = enabled && !n.Semantic.Running
		return n
	case LabelNode:
		return wrap(n.Semantic, TextNode{Value: n.Text})
	case TextFieldNode:
		if n.Multiline {
			return wrap(n.Semantic, TextAreaNode{Label: n.Label, Value: n.Value, Placeholder: n.Hint, Description: n.Semantic.Description, Error: n.Semantic.Error, Disabled: !n.Semantic.Enabled || n.Semantic.ReadOnly, OnChange: n.OnChange})
		}
		return wrap(n.Semantic, TextInputNode{Label: n.Label, Value: n.Value, Placeholder: n.Hint, Description: n.Semantic.Description, Error: n.Semantic.Error, Disabled: !n.Semantic.Enabled, ReadOnly: n.Semantic.ReadOnly, OnChange: n.OnChange})
	case ToggleFieldNode:
		return wrap(n.Semantic, SwitchNode{Label: n.Label, Checked: n.Value, Disabled: !n.Semantic.Enabled || n.Semantic.ReadOnly, OnChange: n.OnChange})
	case ChoiceFieldNode:
		return wrap(n.Semantic, SelectNode{Label: n.Label, Options: n.Options, Value: n.Value, Disabled: !n.Semantic.Enabled || n.Semantic.ReadOnly, OnChange: n.OnChange})
	case RangeFieldNode:
		return wrap(n.Semantic, SliderNode{Label: n.Label, Min: n.Min, Max: n.Max, Step: n.Step, Value: n.Value, Disabled: !n.Semantic.Enabled || n.Semantic.ReadOnly, OnChange: n.OnChange})
	case CollectionNode:
		if len(n.Items) > 0 {
			for itemIndex := range n.Items {
				for actionIndex := range n.Items[itemIndex].Actions {
					resolved := lowerSemantic(n.Items[itemIndex].Actions[actionIndex], commands)
					n.Items[itemIndex].Actions[actionIndex] = resolved.(ActionNode)
				}
			}
			return n
		}
		cards := make([]Node, 0, len(n.Rows)+1)
		cards = append(cards, TextNode{Value: n.Caption})
		for _, row := range n.Rows {
			fields := make([]Node, 0, len(n.Columns))
			for _, column := range n.Columns {
				fields = append(fields, TextNode{Value: column.Label + ": " + row.Cells[column.Key]})
			}
			cards = append(cards, ContainerNode{Direction: Vertical, Gap: int(SpaceTight), Children: fields})
		}
		table := TableNode{Caption: n.Caption, Columns: n.Columns, Rows: n.Rows}
		adaptive := ResponsiveNode{Breakpoint: 840, Compact: cards, Wide: []Node{table}}
		return wrap(n.Semantic, adaptive)
	case NavigationNode:
		tabs := make([]Tab, len(n.Destinations))
		for i, destination := range n.Destinations {
			tabs[i] = Tab{ID: destination.ID, Label: destination.Label, Content: lowerChildren(destination.Content)}
		}
		return wrap(n.Semantic, TabsNode{Tabs: tabs, Selected: n.Selected, OnChange: n.OnChange})
	case StatusNode:
		return wrap(n.Semantic, BadgeNode{Text: n.Text, Variant: n.Variant})
	case ProgressNode:
		return wrap(n.Semantic, ProgressBarNode{Value: n.Value, Max: n.Max, Indeterminate: n.Indeterminate, Label: n.Semantic.Name})
	case DisclosureNode:
		sections := append([]AccordionSection(nil), n.Sections...)
		for i := range sections {
			sections[i].Content = lowerChildren(sections[i].Content)
		}
		return wrap(n.Semantic, AccordionNode{Sections: sections})
	case DialogNode:
		return wrap(n.Semantic, ModalNode{Trigger: n.Trigger, Title: n.Title, Content: lowerChildren(n.Content)})
	case FormNode:
		return wrap(n.Semantic, ContainerNode{Direction: Vertical, Gap: int(SpaceRelated), Children: lowerChildren(n.Children)})
	case SectionNode:
		n.Children = lowerChildren(n.Children)
		return n
	case WorkspaceNode:
		if n.Navigation != nil {
			n.Navigation = lowerSemantic(n.Navigation, commands)
		}
		if n.Status != nil {
			n.Status = lowerSemantic(n.Status, commands)
		}
		n.Content = lowerChildren(n.Content)
		n.Tools = lowerChildren(n.Tools)
		for _, commandID := range n.HeaderActions {
			if command, ok := commands[commandID]; ok && command.Visible {
				n.Header = append(n.Header, lowerSemantic(ActionNode{Semantic: Semantic{ID: "header-" + commandID, Enabled: true}, Command: commandID, Placement: PlacementToolbar}, commands))
			}
		}
		return n
	case AdaptiveNode:
		ultra := ResponsiveNode{Breakpoint: 1200, Compact: lowerChildren(n.Expanded), Wide: lowerChildren(n.UltraWide)}
		expanded := ResponsiveNode{Breakpoint: 840, Compact: lowerChildren(n.Medium), Wide: []Node{ultra}}
		all := ResponsiveNode{Breakpoint: 600, Compact: lowerChildren(n.Compact), Wide: []Node{expanded}}
		return wrap(n.Semantic, all)
	case IdentityNode:
		return IdentityNode{Semantic: n.Semantic, Child: lowerSemantic(n.Child, commands)}
	case ContainerNode:
		n.Children = lowerChildren(n.Children)
		return n
	case TabsNode:
		for i := range n.Tabs {
			n.Tabs[i].Content = lowerChildren(n.Tabs[i].Content)
		}
		return n
	case AccordionNode:
		for i := range n.Sections {
			n.Sections[i].Content = lowerChildren(n.Sections[i].Content)
		}
		return n
	case ModalNode:
		n.Content = lowerChildren(n.Content)
		return n
	default:
		return node
	}
}

// WindowClass is re-exported for application adaptation helpers.
type WindowClass = design.WindowClass
