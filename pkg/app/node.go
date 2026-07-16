// Package core defines Trellis's target-agnostic declarative UI tree and the
// App type an application author writes against. Backends (poem, web)
// translate a Node tree into a real running interface.
package app

import "strconv"

// Msg is a named, serializable event fired by an interactive Node. It has to
// be representable as a plain string plus an optional string payload rather
// than a Go closure: the POEM backend can dispatch it in-process, but the web
// backend has to carry it across an HTTP request, where a closure can't
// travel. Non-string payloads (so far, only bool, fired by CheckboxNode) are
// encoded as the literal strings "true"/"false" — see BoolPayload and
// Msg.Bool.
type Msg struct {
	Name    string
	Payload string
}

// Bool decodes a Payload encoded by BoolPayload.
func (m Msg) Bool() bool {
	return m.Payload == "true"
}

// BoolPayload encodes v the same way CheckboxNode's fired Msg encodes its
// Checked value, for callers constructing a Msg by hand.
func BoolPayload(v bool) string {
	return strconv.FormatBool(v)
}

// Float decodes a Payload encoded by FloatPayload, returning 0 if the payload
// is not a number.
func (m Msg) Float() float64 {
	v, err := strconv.ParseFloat(m.Payload, 64)
	if err != nil {
		return 0
	}
	return v
}

// FloatPayload encodes v the same way SliderNode's fired Msg encodes its
// value: a plain decimal with no scientific notation, so it survives the web
// transport's string round trip and an <input type="range">'s own posted
// value parses back identically. This is the second non-string payload
// convention after BoolPayload — Msg.Payload stays string, and each numeric
// kind that needs it declares its own encode/decode pair here rather than the
// IR growing a generic typed-payload mechanism (see okf architecture/overview).
func FloatPayload(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// Variant selects a Node's semantic visual treatment. Its members are the
// intersection POEM (theme.Variant) and GopherWeb (components.Variant) already
// converged on independently; a member either backend lacked (POEM's Subtle)
// is deliberately excluded so the shared IR only names treatments both
// backends can honor. Each backend's render.go maps these onto its own type.
type Variant int

const (
	VariantNeutral Variant = iota
	VariantPrimary
	VariantSecondary
	VariantDanger
	VariantSuccess
	VariantWarning
)

// Node is the shared declarative UI tree. Concrete kinds are TextNode,
// ButtonNode, TextInputNode, TextAreaNode, CheckboxNode, SwitchNode,
// SelectNode, RadioGroupNode, SliderNode, BadgeNode, ProgressBarNode,
// TableNode, AccordionNode, TabsNode, ContainerNode, and ModalNode.
type Node interface {
	isNode()
}

// TextNode renders a read-only line of text.
type TextNode struct {
	Value string
}

func (TextNode) isNode() {}

// Text creates a TextNode.
func Text(value string) TextNode {
	return TextNode{Value: value}
}

// ButtonNode renders an activatable control that fires OnClick when pressed.
type ButtonNode struct {
	Label    string
	OnClick  Msg
	Disabled bool
}

func (ButtonNode) isNode() {}

// Button creates a ButtonNode.
func Button(label string, onClick Msg) ButtonNode {
	return ButtonNode{Label: label, OnClick: onClick}
}

// TextInputNode renders an editable single-line text field that fires
// OnChange when its value changes.
type TextInputNode struct {
	Value       string
	Placeholder string
	OnChange    Msg
}

func (TextInputNode) isNode() {}

// TextInput creates a TextInputNode.
func TextInput(value, placeholder string, onChange Msg) TextInputNode {
	return TextInputNode{Value: value, Placeholder: placeholder, OnChange: onChange}
}

// TextAreaNode renders an editable multi-line text field that fires OnChange
// when its value changes. It is TextInputNode's multi-line sibling — POEM's
// TextArea and GopherWeb's form.Textarea both back it — and posts its value
// the same valueField way over the web backend.
type TextAreaNode struct {
	Value       string
	Placeholder string
	Rows        int
	Disabled    bool
	OnChange    Msg
}

func (TextAreaNode) isNode() {}

// TextArea creates a TextAreaNode.
func TextArea(value, placeholder string, onChange Msg) TextAreaNode {
	return TextAreaNode{Value: value, Placeholder: placeholder, OnChange: onChange}
}

// SliderNode renders a draggable control that selects a numeric value in
// [Min, Max], firing OnChange with a FloatPayload-encoded value. It is the
// first Node kind with a float payload — POEM's Slider and GopherWeb's
// form.Range back it — so it carries its own encode/decode convention
// (FloatPayload / Msg.Float), just as CheckboxNode introduced BoolPayload for
// bool. Step, when positive, constrains the value to multiples of it.
type SliderNode struct {
	Min      float64
	Max      float64
	Step     float64
	Value    float64
	Disabled bool
	OnChange Msg
}

func (SliderNode) isNode() {}

// Slider creates a SliderNode spanning [min, max] with the given value.
func Slider(min, max, value float64, onChange Msg) SliderNode {
	return SliderNode{Min: min, Max: max, Value: value, OnChange: onChange}
}

// BadgeNode renders a compact, non-interactive semantic status label. Like
// TextNode it fires no Msg; unlike TextNode it carries a Variant so both
// backends render it with their own semantic status styling.
type BadgeNode struct {
	Text    string
	Variant Variant
}

func (BadgeNode) isNode() {}

// Badge creates a BadgeNode.
func Badge(text string, variant Variant) BadgeNode {
	return BadgeNode{Text: text, Variant: variant}
}

// ProgressBarNode renders a non-interactive completion indicator. It is the
// display-only counterpart of SliderNode — POEM's ProgressBar and GopherWeb's
// feedback.Progress back it — and, like BadgeNode, fires no Msg. It has no Min
// field on purpose: progress runs from empty (0) to Max, matching the web
// backend's native <progress> element, which has no min. When Indeterminate is
// set, Value is ignored and both backends render an ongoing activity state.
type ProgressBarNode struct {
	Value         float64
	Max           float64
	Indeterminate bool
	Label         string
}

func (ProgressBarNode) isNode() {}

// ProgressBar creates a determinate ProgressBarNode running from 0 to max.
func ProgressBar(value, max float64) ProgressBarNode {
	return ProgressBarNode{Value: value, Max: max}
}

// CheckboxNode renders a labelled toggle that fires OnChange with a
// BoolPayload-encoded value when it's checked or unchecked.
type CheckboxNode struct {
	Label    string
	Checked  bool
	Disabled bool
	OnChange Msg
}

func (CheckboxNode) isNode() {}

// Checkbox creates a CheckboxNode.
func Checkbox(label string, checked bool, onChange Msg) CheckboxNode {
	return CheckboxNode{Label: label, Checked: checked, OnChange: onChange}
}

// SwitchNode renders a labelled on/off toggle. It is CheckboxNode's visual
// sibling — POEM's Switch and GopherWeb's form.Switch both back it — and
// fires and posts its bool value exactly the same way a CheckboxNode does
// (BoolPayload encoding; presence-is-the-value over the web transport). The
// two kinds stay distinct because the choice of a check versus a toggle is a
// real presentation decision an application author makes, and both backends
// render them as genuinely different controls.
type SwitchNode struct {
	Label    string
	Checked  bool
	Disabled bool
	OnChange Msg
}

func (SwitchNode) isNode() {}

// Switch creates a SwitchNode.
func Switch(label string, checked bool, onChange Msg) SwitchNode {
	return SwitchNode{Label: label, Checked: checked, OnChange: onChange}
}

// Option is one choice in a SelectNode.
type Option struct {
	Label string
	Value string
}

// SelectNode renders a single-choice dropdown that fires OnChange with the
// newly selected Option's Value. It has no Open/OnOpenChange field: both
// backends can manage their own open/closed popup state internally (POEM's
// Select does this itself, keyed by the node's stable ID; a native HTML
// <select> does it natively), so the shared IR doesn't need to represent it.
type SelectNode struct {
	Options     []Option
	Value       string
	Placeholder string
	Disabled    bool
	OnChange    Msg
}

func (SelectNode) isNode() {}

// Select creates a SelectNode.
func Select(options []Option, value string, onChange Msg) SelectNode {
	return SelectNode{Options: options, Value: value, OnChange: onChange}
}

// RadioGroupNode renders a set of mutually exclusive choices that fires
// OnChange with the newly selected Option's Value. It is SelectNode's sibling
// — the same single-choice-from-a-list contract, presented as radios instead
// of a dropdown — and reuses the same Option type. On POEM it builds one
// Radio per Option sharing the group's OnChange; on the web it is GopherWeb's
// form.RadioGroup, a fieldset of radios sharing one name. Single-selection is
// enforced the same way SelectNode's is: the reducer sets Value and the tree
// is rebuilt, so exactly the matching Option renders selected.
type RadioGroupNode struct {
	Options  []Option
	Value    string
	Disabled bool
	OnChange Msg
}

func (RadioGroupNode) isNode() {}

// RadioGroup creates a RadioGroupNode.
func RadioGroup(options []Option, value string, onChange Msg) RadioGroupNode {
	return RadioGroupNode{Options: options, Value: value, OnChange: onChange}
}

// TableColumn describes one column of a TableNode: Key selects which value to
// read from each row, and Label is the column heading.
type TableColumn struct {
	Key   string
	Label string
}

// TableRow is one row of a TableNode, mapping each TableColumn.Key to its
// cell value.
type TableRow struct {
	Cells map[string]string
}

// TableNode renders read-only tabular data. It fires no Msg: this is the
// display-only intersection of POEM's DataTable and GopherWeb's data.Table
// (both of which additionally support row selection / sorting that this shared
// kind deliberately does not surface yet — see okf component-parity). Columns
// give the heading order; each row supplies its cells by column Key.
type TableNode struct {
	Caption string
	Columns []TableColumn
	Rows    []TableRow
}

func (TableNode) isNode() {}

// Table creates a read-only TableNode.
func Table(columns []TableColumn, rows []TableRow) TableNode {
	return TableNode{Columns: columns, Rows: rows}
}

// Tab is one tab of a TabsNode. ID is the value its OnChange Msg carries when
// the tab is activated.
type Tab struct {
	ID      string
	Label   string
	Content []Node
}

// TabsNode renders a tab strip plus the selected tab's content. Unlike
// ModalNode and AccordionNode — whose open state is backend-local chrome —
// a TabsNode's selection lives in App[S] and travels as an OnChange Msg
// carrying the activated Tab's ID. That is a deliberate trade: the application
// can read and set the selected tab (for example, jumping to a tab after a
// save), at the cost of a full page round trip per tab click on the web
// backend's forms-only transport — acceptable on a no-JS baseline that already
// reloads on every button. Only the selected tab's content is rendered on
// either backend, so a hidden tab's controls post nothing.
type TabsNode struct {
	Tabs     []Tab
	Selected string
	OnChange Msg
}

func (TabsNode) isNode() {}

// Tabs creates a TabsNode.
func Tabs(tabs []Tab, selected string, onChange Msg) TabsNode {
	return TabsNode{Tabs: tabs, Selected: selected, OnChange: onChange}
}

// ActiveIndex returns the index of the selected Tab, falling back to the first
// tab when Selected matches none (and -1 when there are no tabs). Both
// backends and the web field collectors use it so they always agree on which
// tab's content is live.
func (n TabsNode) ActiveIndex() int {
	for i, tab := range n.Tabs {
		if tab.ID == n.Selected {
			return i
		}
	}
	if len(n.Tabs) > 0 {
		return 0
	}
	return -1
}

// AccordionSection is one collapsible section of an AccordionNode.
type AccordionSection struct {
	Title       string
	Content     []Node
	DefaultOpen bool
}

// AccordionNode renders a stack of collapsible sections. Like ModalNode, its
// open/closed state is not part of App[S]: which sections are expanded is
// backend-local UI chrome both backends manage themselves — POEM's Accordion
// self-manages it in its transient store when uncontrolled, and the web
// backend uses GopherWeb's native <details> disclosure, which toggles
// client-side with no server round trip. DefaultOpen seeds the initial state
// on each backend. Content nested inside a section still fires Msgs through
// the normal Update pipeline; only the expand/collapse chrome sits outside it.
type AccordionNode struct {
	Sections []AccordionSection
}

func (AccordionNode) isNode() {}

// Accordion creates an AccordionNode from its sections.
func Accordion(sections ...AccordionSection) AccordionNode {
	return AccordionNode{Sections: sections}
}

// Direction controls how a ContainerNode lays out its children.
type Direction int

const (
	Vertical Direction = iota
	Horizontal
)

// ContainerNode groups child nodes along Direction with a gap between them.
type ContainerNode struct {
	Direction Direction
	Gap       int
	Padding   int
	Children  []Node
}

func (ContainerNode) isNode() {}

// Container creates a ContainerNode.
func Container(direction Direction, gap int, children ...Node) ContainerNode {
	return ContainerNode{Direction: direction, Gap: gap, Children: children}
}

// ModalNode renders a trigger and, behind it, a dialog containing Content.
// Unlike every other interactive Node, its open/closed state is not part of
// App[S] — see okf/architecture/overview.md for why: both backends already
// have their own idiomatic, backend-local way to manage a dialog's open/
// closed state (POEM's OverlayManager, a native-JS hidden toggle on the
// web), so the shared IR doesn't try to unify something neither backend
// actually needs unified. Content nested inside still fires Msgs through the
// normal Update pipeline once submitted.
type ModalNode struct {
	Trigger string
	Title   string
	Content []Node
}

func (ModalNode) isNode() {}

// Modal creates a ModalNode.
func Modal(trigger, title string, content ...Node) ModalNode {
	return ModalNode{Trigger: trigger, Title: title, Content: content}
}
