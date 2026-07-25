// Package core defines Trellis's target-agnostic declarative UI tree and the
// App type an application author writes against. Backends (poem, web)
// translate a Node tree into a real running interface.
package app

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"strings"
)

// Msg is a named event fired by an interactive Node or completed Cmd. Its
// browser-transported portion has to be representable as a plain string plus
// an optional string payload rather than a Go closure: the POEM backend can
// dispatch it in-process, but the web backend has to carry it across an HTTP
// request, where a closure can't travel. Runtime-local command completions
// may also carry Value and Err.
// Non-string browser payloads (so far, only bool, fired by CheckboxNode) are
// encoded as the literal strings "true"/"false" — see BoolPayload and
// Msg.Bool.
type Msg struct {
	Name    string
	Payload string

	// Value and Err are runtime-local command completion data. Browser events
	// can populate only Name and Payload; drivers set these fields after a Cmd
	// completes inside the application process.
	Value any
	Err   error
}

// Cmd describes asynchronous application work. Name is both the concurrency
// key and the name of the completion Msg. Commands with different names may
// run concurrently; a newer command with the same name supersedes the older
// one. The zero value performs no work.
type Cmd struct {
	Name string
	Run  func(context.Context) (any, error)
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

// ImageTransform is the controlled transform applied by an ImageViewportNode.
// OffsetX and OffsetY are fractions of the viewport width and height; positive
// values move the image right and down. Scale is normalized to 1 when zero.
type ImageTransform struct {
	OffsetX float64 `json:"x"`
	OffsetY float64 `json:"y"`
	Scale   float64 `json:"scale"`
}

// ImageViewportCommit is the controlled transform plus the logical viewport
// dimensions that produced it. Zero dimensions represent the legacy M4
// transform-only payload.
type ImageViewportCommit struct {
	OffsetX float64 `json:"x"`
	OffsetY float64 `json:"y"`
	Scale   float64 `json:"scale"`
	Width   int     `json:"width,omitempty"`
	Height  int     `json:"height,omitempty"`
}

func (c ImageViewportCommit) Transform() ImageTransform {
	return ImageTransform{OffsetX: c.OffsetX, OffsetY: c.OffsetY, Scale: c.Scale}.Normalized()
}
func (c ImageViewportCommit) Valid() bool {
	t := c.Transform()
	return t.Valid() && ((c.Width == 0 && c.Height == 0) || (c.Width >= 32 && c.Width <= 4096 && c.Height >= 32 && c.Height <= 4096))
}

func ImageViewportCommitPayload(c ImageViewportCommit) string {
	c.Scale = c.Transform().Scale
	if !c.Valid() {
		return ""
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (m Msg) ViewportCommit() (ImageViewportCommit, bool) {
	var commit ImageViewportCommit
	decoder := json.NewDecoder(strings.NewReader(m.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&commit); err != nil {
		return ImageViewportCommit{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ImageViewportCommit{}, false
	}
	commit.Scale = commit.Transform().Scale
	return commit, commit.Valid()
}

// Normalized returns t with the zero-value scale interpreted as identity.
func (t ImageTransform) Normalized() ImageTransform {
	if t.Scale == 0 {
		t.Scale = 1
	}
	return t
}

// Valid reports whether t is safe to accept from an untrusted browser event.
func (t ImageTransform) Valid() bool {
	t = t.Normalized()
	return !math.IsNaN(t.OffsetX) && !math.IsInf(t.OffsetX, 0) &&
		!math.IsNaN(t.OffsetY) && !math.IsInf(t.OffsetY, 0) &&
		!math.IsNaN(t.Scale) && !math.IsInf(t.Scale, 0) &&
		math.Abs(t.OffsetX) <= 4 && math.Abs(t.OffsetY) <= 4 &&
		t.Scale >= 0.125 && t.Scale <= 64
}

// ImageTransformPayload encodes a transform for Msg.Payload.
func ImageTransformPayload(t ImageTransform) string {
	t = t.Normalized()
	if !t.Valid() {
		return ""
	}
	encoded, err := json.Marshal(t)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// ImageTransform decodes a strictly validated ImageViewportNode payload.
func (m Msg) ImageTransform() (ImageTransform, bool) {
	commit, ok := m.ViewportCommit()
	if !ok {
		return ImageTransform{}, false
	}
	return commit.Transform(), true
}

// ViewportPoint is a normalized point inside an ImageViewportNode. X and Y
// range from zero at the top-left edge to one at the bottom-right edge.
type ViewportPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Valid reports whether p can safely originate from an untrusted client.
func (p ViewportPoint) Valid() bool {
	return !math.IsNaN(p.X) && !math.IsInf(p.X, 0) && !math.IsNaN(p.Y) && !math.IsInf(p.Y, 0) &&
		p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1
}

// ViewportPointPayload strictly encodes p for Msg.Payload.
func ViewportPointPayload(p ViewportPoint) string {
	if !p.Valid() {
		return ""
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// ViewportPoint decodes a strictly validated ImageViewportNode activation.
func (m Msg) ViewportPoint() (ViewportPoint, bool) {
	var point ViewportPoint
	decoder := json.NewDecoder(strings.NewReader(m.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&point); err != nil {
		return ViewportPoint{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ViewportPoint{}, false
	}
	return point, point.Valid()
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
// TableNode, AccordionNode, TabsNode, ImageNode, ImageViewportNode,
// ResponsiveNode, ContainerNode, and
// ModalNode.
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
	Label     string
	OnClick   Msg
	Disabled  bool
	Variant   Variant
	Icon      IconID
	Placement ActionPlacement
}

func (ButtonNode) isNode() {}

// Button creates a ButtonNode.
func Button(label string, onClick Msg) ButtonNode {
	return ButtonNode{Label: label, OnClick: onClick}
}

// TextInputNode renders an editable single-line text field that fires
// OnChange when its value changes.
type TextInputNode struct {
	Label       string
	Value       string
	Placeholder string
	Description string
	Error       string
	Disabled    bool
	ReadOnly    bool
	Password    bool
	OnChange    Msg
}

func (TextInputNode) isNode() {}

// TextInput creates a TextInputNode.
func TextInput(value, placeholder string, onChange Msg) TextInputNode {
	return TextInputNode{Value: value, Placeholder: placeholder, OnChange: onChange}
}

// PasswordInput creates a TextInputNode masked for sensitive password input.
func PasswordInput(value, placeholder string, onChange Msg) TextInputNode {
	return TextInputNode{Value: value, Placeholder: placeholder, Password: true, OnChange: onChange}
}

// TextAreaNode renders an editable multi-line text field that fires OnChange
// when its value changes. It is TextInputNode's multi-line sibling — POEM's
// TextArea and GopherWeb's form.Textarea both back it — and posts its value
// the same valueField way over the web backend.
type TextAreaNode struct {
	Label       string
	Value       string
	Placeholder string
	Description string
	Error       string
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
	Label    string
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
	Label       string
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
	Label    string
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

// OverlayAnchor names where a floating layer sits within an OverlayNode.
type OverlayAnchor int

const (
	OverlayTopLeft OverlayAnchor = iota
	OverlayTop
	OverlayTopRight
	OverlayLeft
	OverlayCenter
	OverlayRight
	OverlayBottomLeft
	OverlayBottom
	OverlayBottomRight
)

func (a OverlayAnchor) valid() bool { return a >= OverlayTopLeft && a <= OverlayBottomRight }

// OverlayLayer is one floating layer above an OverlayNode's Base. Content is
// sized to itself and held Inset logical pixels clear of the edges Anchor hugs.
type OverlayLayer struct {
	Anchor  OverlayAnchor
	Inset   int
	Content Node
}

// OverlayNode stacks floating Layers on top of Base within one box. Base fills
// the box and alone determines its size; layers float above without affecting
// layout, so a canvas (a map, an image) keeps its full area while its controls
// sit in a corner. Layers paint in order (last on top) and, being drawn over
// the canvas, take the click before it. Reach for it when content belongs
// visually *on* another region rather than beside it — map zoom controls, a
// floating action button. For a centered modal dialog with a backdrop, use
// ModalNode instead; an overlay is non-modal and leaves the canvas interactive.
type OverlayNode struct {
	Base   Node
	Layers []OverlayLayer
}

func (OverlayNode) isNode() {}

// OverlayFloat wraps base with floating layers. Layers are given in paint
// order (last on top).
func OverlayFloat(base Node, layers ...OverlayLayer) OverlayNode {
	return OverlayNode{Base: base, Layers: layers}
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

// ImageNode renders a raster image from encoded bytes (PNG, JPEG, or GIF —
// whatever image.Decode understands). It is display-only and fires no Msg.
// The bytes travel as-is to the web target (a data URI) and are decoded once
// per content change on the native targets, so a View may return the same
// slice every frame without re-decode cost. Alt describes the image for
// accessibility. MaxWidth/MaxHeight (logical px, optional) bound the layout;
// the image keeps its aspect ratio inside them.
type ImageNode struct {
	Encoded   []byte
	Alt       string
	MaxWidth  int
	MaxHeight int
}

func (ImageNode) isNode() {}

// Image creates an ImageNode from encoded image bytes.
func Image(encoded []byte, alt string) ImageNode {
	return ImageNode{Encoded: encoded, Alt: alt}
}

// ImageViewportNode renders an ImageNode inside an interactive, clipped
// viewport. Transform is controlled application state. Pointer drag, wheel,
// and pinch gestures update it through OnChange; MinScale and MaxScale default
// to 0.5 and 8. Disabled preserves the current view while suppressing input.
type ImageViewportNode struct {
	Image     ImageNode
	Transform ImageTransform
	MinScale  float64
	MaxScale  float64
	Disabled  bool
	OnChange  Msg
	// OnActivate fires for a short click or tap on the viewport background.
	OnActivate Msg
	// Markers are accessible annotations anchored to normalized image points.
	Markers []ImageMarker
}

func (ImageViewportNode) isNode() {}

// ImageViewport creates an interactive viewport for image.
func ImageViewport(image ImageNode, onChange Msg) ImageViewportNode {
	return ImageViewportNode{Image: image, MinScale: 0.5, MaxScale: 8, OnChange: onChange}
}

// ResponsiveNode selects one of two node groups from the available viewport
// width. Compact is the no-JavaScript web baseline.
type ResponsiveNode struct {
	// Breakpoint is the maximum logical-pixel width that uses Compact.
	Breakpoint int
	// Compact contains the narrow layout.
	Compact []Node
	// Wide contains the layout used above Breakpoint.
	Wide []Node
}

func (ResponsiveNode) isNode() {}

// Responsive creates an adaptive node group.
func Responsive(breakpoint int, compact, wide []Node) ResponsiveNode {
	if breakpoint <= 0 {
		breakpoint = 600
	}
	return ResponsiveNode{Breakpoint: breakpoint, Compact: compact, Wide: wide}
}

// ImageMarker is an accessible annotation anchored to normalized image
// coordinates. Marker IDs must be stable within the current View tree.
type ImageMarker struct {
	// ID is stable and unique within this viewport's current marker set.
	ID string
	// X is the normalized horizontal image coordinate.
	X float64
	// Y is the normalized vertical image coordinate.
	Y float64
	// Label is the marker's accessible name and optional selected caption.
	Label string
	// Variant selects the marker color.
	Variant Variant
	// Selected emphasizes the marker and exposes its selected semantic state.
	Selected bool
	// Disabled suppresses pointer and semantic activation.
	Disabled bool
	// OnActivate fires with the marker ID in Msg.Payload.
	OnActivate Msg
}
