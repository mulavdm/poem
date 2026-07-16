package web

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components"
	"github.com/mulavdm/poem/pkg/web/components/action"
	"github.com/mulavdm/poem/pkg/web/components/data"
	"github.com/mulavdm/poem/pkg/web/components/feedback"
	"github.com/mulavdm/poem/pkg/web/components/form"
	"github.com/mulavdm/poem/pkg/web/components/widget"

	"github.com/mulavdm/poem/pkg/app"
)

// msgFieldName is the hidden/submit-button field carrying which Msg fired
// this form submission. fieldPrefix namespaces TextInputNode field names so
// their posted values can be matched back to the Node that produced them.
const (
	msgFieldName = "trellis_msg"
	fieldPrefix  = "trellis_field_"
)

// renderNode converts a app.Node tree into GopherWeb component HTML, using
// GopherWeb's real public renderers (action.Button, form.Input, ...) so
// styling, escaping, and markup shape match the rest of a GopherWeb-based
// page exactly — Trellis never hand-rolls unescaped HTML for a node kind
// GopherWeb already renders. path is the same stable per-node identifier
// scheme the poem backend uses, so the two backends address equivalent nodes
// with equivalent IDs.
func renderNode(node app.Node, path string) template.HTML {
	switch n := node.(type) {
	case app.TextNode:
		return template.HTML("<p class=\"trellis-text\">" + html.EscapeString(n.Value) + "</p>")

	case app.ButtonNode:
		return action.Button{
			Text:     n.Label,
			Type:     "submit",
			Disabled: n.Disabled,
			Attributes: components.Attrs{
				"name":  msgFieldName,
				"value": n.OnClick.Name,
			},
		}.HTML()

	case app.TextInputNode:
		return form.Input{
			Name:        fieldPrefix + path,
			Value:       n.Value,
			Placeholder: n.Placeholder,
		}.HTML()

	case app.TextAreaNode:
		return form.Textarea{
			Name:        fieldPrefix + path,
			Value:       n.Value,
			Placeholder: n.Placeholder,
			Rows:        n.Rows,
			Disabled:    n.Disabled,
		}.HTML()

	case app.SliderNode:
		return form.Range{
			Name:     fieldPrefix + path,
			Min:      n.Min,
			Max:      n.Max,
			Step:     n.Step,
			Value:    n.Value,
			Disabled: n.Disabled,
		}.HTML()

	case app.BadgeNode:
		return feedback.Badge{
			Text:    n.Text,
			Variant: webVariant(n.Variant),
		}.HTML()

	case app.ProgressBarNode:
		return feedback.Progress{
			Label:         n.Label,
			Value:         n.Value,
			Max:           n.Max,
			Indeterminate: n.Indeterminate,
		}.HTML()

	case app.CheckboxNode:
		return form.Checkbox{
			Name:     fieldPrefix + path,
			Label:    n.Label,
			Checked:  n.Checked,
			Disabled: n.Disabled,
		}.HTML()

	case app.SwitchNode:
		return form.Switch{
			Name:     fieldPrefix + path,
			Label:    n.Label,
			Checked:  n.Checked,
			Disabled: n.Disabled,
		}.HTML()

	case app.SelectNode:
		options := make([]form.Option, len(n.Options))
		for i, option := range n.Options {
			options[i] = form.Option{Label: option.Label, Value: option.Value}
		}
		return form.Select{
			Name:     fieldPrefix + path,
			Options:  options,
			Value:    n.Value,
			Disabled: n.Disabled,
		}.HTML()

	case app.RadioGroupNode:
		options := make([]form.Option, len(n.Options))
		for i, option := range n.Options {
			options[i] = form.Option{Label: option.Label, Value: option.Value}
		}
		return form.RadioGroup{
			Name:     fieldPrefix + path,
			Options:  options,
			Value:    n.Value,
			Disabled: n.Disabled,
		}.HTML()

	case app.TableNode:
		columns := make([]data.TableColumn, len(n.Columns))
		for i, column := range n.Columns {
			columns[i] = data.TableColumn{Key: column.Key, Label: column.Label}
		}
		rows := make([]data.TableRow, len(n.Rows))
		for i, row := range n.Rows {
			rows[i] = data.TableRow(row.Cells)
		}
		return data.Table{Caption: n.Caption, Columns: columns, Rows: rows}.HTML()

	case app.TabsNode:
		items := make([]widget.TabItem, len(n.Tabs))
		for i, tab := range n.Tabs {
			items[i] = widget.TabItem{ID: tab.ID, Label: tab.Label}
		}
		// Only the selected tab's content is rendered: selection is App[S]-owned
		// (see app.TabsNode), so each tab button submits the form and the
		// server decides what the next panel is — no hidden panels, and no
		// controls posting from a tab the visitor can't see. The tab buttons
		// post the strip's own field name, so the existing valueField path
		// dispatches OnChange with the activated tab's ID.
		var panel strings.Builder
		selected := ""
		if active := n.ActiveIndex(); active >= 0 {
			selected = n.Tabs[active].ID
			tabPath := childPath(path, active)
			for j, child := range n.Tabs[active].Content {
				panel.WriteString(string(renderNode(child, childPath(tabPath, j))))
			}
		}
		return widget.FormTabs{
			Name:      fieldPrefix + path,
			Items:     items,
			Selected:  selected,
			PanelHTML: template.HTML(panel.String()),
		}.HTML()

	case app.AccordionNode:
		// One GopherWeb <details> disclosure per section. Its open/closed state
		// is native client-side chrome with no server round trip (see
		// app.AccordionNode); every section's content is rendered into the form
		// up front, so nested controls submit regardless of open state.
		var b strings.Builder
		b.WriteString(`<div class="trellis-accordion">`)
		for i, section := range n.Sections {
			sectionPath := childPath(path, i)
			var content strings.Builder
			for j, child := range section.Content {
				content.WriteString(string(renderNode(child, childPath(sectionPath, j))))
			}
			b.WriteString(string(widget.Disclosure{
				Summary: section.Title,
				Content: template.HTML(content.String()),
				Open:    section.DefaultOpen,
			}.HTML()))
		}
		b.WriteString(`</div>`)
		return template.HTML(b.String())

	case app.ContainerNode:
		class := "trellis-container trellis-container--vertical"
		if n.Direction == app.Horizontal {
			class = "trellis-container trellis-container--horizontal"
		}
		var b strings.Builder
		fmt.Fprintf(&b, `<div class="%s" style="gap:%dpx;padding:%dpx">`, class, n.Gap, n.Padding)
		for i, child := range n.Children {
			b.WriteString(string(renderNode(child, childPath(path, i))))
		}
		b.WriteString(`</div>`)
		return template.HTML(b.String())

	case app.ModalNode:
		var content strings.Builder
		for i, child := range n.Content {
			content.WriteString(string(renderNode(child, childPath(path, i))))
		}
		// GopherWeb's widget.Modal handles the backdrop, the hidden toggle,
		// the focus trap, Escape, and its own close button entirely
		// client-side — no new JS to write here. Its open/closed state never
		// round-trips to the server (see app.ModalNode's doc comment), so
		// unlike every other case in this switch, nothing here reads from or
		// writes to App[S].
		return widget.Modal{
			ID:          path,
			Title:       n.Title,
			ContentHTML: template.HTML(content.String()),
			Trigger:     action.Button{Text: n.Trigger},
		}.HTML()

	default:
		panic(fmt.Sprintf("web: unsupported node type %T", node))
	}
}

// fieldKind selects how the /__event handler reads a posted field's value
// back out of the submitted form. Most fields are always present and carry
// their value directly (valueField); an HTML checkbox is only present in the
// POST body when checked, so its presence itself is the value
// (checkboxField) rather than something to read out of it.
type fieldKind int

const (
	valueField fieldKind = iota
	checkboxField
)

// postedField pairs the Msg an interactive Node fires with how to read its
// current value back out of a form submission.
type postedField struct {
	msg  app.Msg
	kind fieldKind
}

// collectFields walks node collecting every interactive Node's posted field
// name mapped to how to read and dispatch it, so the /__event handler can
// translate a posted form into Update calls before applying the message that
// actually submitted the form.
func collectFields(node app.Node, path string, out map[string]postedField) {
	switch n := node.(type) {
	case app.TextInputNode:
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: valueField}
	case app.TextAreaNode:
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: valueField}
	case app.CheckboxNode:
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: checkboxField}
	case app.SwitchNode:
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: checkboxField}
	case app.SelectNode:
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: valueField}
	case app.RadioGroupNode:
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: valueField}
	case app.SliderNode:
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: valueField}
	case app.ContainerNode:
		for i, child := range n.Children {
			collectFields(child, childPath(path, i), out)
		}
	case app.TabsNode:
		// The strip posts the activated tab's ID under its own field name.
		out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: valueField}
		if active := n.ActiveIndex(); active >= 0 {
			tabPath := childPath(path, active)
			for j, child := range n.Tabs[active].Content {
				collectFields(child, childPath(tabPath, j), out)
			}
		}
	case app.AccordionNode:
		for i, section := range n.Sections {
			sectionPath := childPath(path, i)
			for j, child := range section.Content {
				collectFields(child, childPath(sectionPath, j), out)
			}
		}
	case app.ModalNode:
		for i, child := range n.Content {
			collectFields(child, childPath(path, i), out)
		}
	}
}

func collectMessages(node app.Node, out map[string]bool) {
	switch n := node.(type) {
	case app.ButtonNode:
		if !n.Disabled && n.OnClick.Name != "" {
			out[n.OnClick.Name] = true
		}
	case app.TextInputNode:
		if n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.TextAreaNode:
		if !n.Disabled && n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.CheckboxNode:
		if !n.Disabled && n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.SwitchNode:
		if !n.Disabled && n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.SelectNode:
		if !n.Disabled && n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.RadioGroupNode:
		if !n.Disabled && n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.SliderNode:
		if !n.Disabled && n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.ContainerNode:
		for _, child := range n.Children {
			collectMessages(child, out)
		}
	case app.TabsNode:
		if n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
		if active := n.ActiveIndex(); active >= 0 {
			for _, child := range n.Tabs[active].Content {
				collectMessages(child, out)
			}
		}
	case app.AccordionNode:
		for _, section := range n.Sections {
			for _, child := range section.Content {
				collectMessages(child, out)
			}
		}
	case app.ModalNode:
		for _, child := range n.Content {
			collectMessages(child, out)
		}
	}
}

// webVariant maps a app.Variant onto GopherWeb's component variant. Both
// enumerations share the same member names, so this stays a total switch.
func webVariant(v app.Variant) components.Variant {
	switch v {
	case app.VariantPrimary:
		return components.VariantPrimary
	case app.VariantSecondary:
		return components.VariantSecondary
	case app.VariantDanger:
		return components.VariantDanger
	case app.VariantSuccess:
		return components.VariantSuccess
	case app.VariantWarning:
		return components.VariantWarning
	default:
		return components.VariantNeutral
	}
}

func childPath(parent string, index int) string {
	return fmt.Sprintf("%s-%d", parent, index)
}
