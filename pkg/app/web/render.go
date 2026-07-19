package web

import (
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"math"
	"net/http"
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
	case app.IdentityNode:
		return renderNode(n.Child, semanticPath(path, n.Semantic.ID))

	case app.ActionNode:
		label := actionLabel(n.Icon, n.Label)
		attrs := components.Attrs{"name": msgFieldName, "value": n.Invoke.Name, "data-poem-placement": actionPlacementName(n.Placement)}
		if n.Semantic.Running {
			attrs["aria-busy"] = "true"
		}
		return action.Button{Text: label, Type: "submit", Variant: actionWebVariant(n), Disabled: !n.Semantic.Enabled || n.Semantic.Running, Attributes: attrs}.HTML()

	case app.TextNode:
		return template.HTML("<p class=\"trellis-text\">" + html.EscapeString(n.Value) + "</p>")

	case app.ButtonNode:
		return action.Button{
			Text:     n.Label,
			Type:     "submit",
			Variant:  webVariant(n.Variant),
			Disabled: n.Disabled,
			Attributes: components.Attrs{
				"name":  msgFieldName,
				"value": n.OnClick.Name,
			},
		}.HTML()

	case app.TextInputNode:
		return form.Input{
			Name:        fieldPrefix + path,
			Label:       n.Label,
			Value:       n.Value,
			Placeholder: n.Placeholder,
			Help:        n.Description,
			Error:       n.Error,
			Disabled:    n.Disabled,
			ReadOnly:    n.ReadOnly,
		}.HTML()

	case app.TextAreaNode:
		return form.Textarea{
			Name:        fieldPrefix + path,
			Label:       n.Label,
			Value:       n.Value,
			Placeholder: n.Placeholder,
			Rows:        n.Rows,
			Disabled:    n.Disabled,
			Help:        n.Description,
			Error:       n.Error,
		}.HTML()

	case app.SliderNode:
		return form.Range{
			Name:     fieldPrefix + path,
			Label:    n.Label,
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
			Label:    n.Label,
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
			Label:    n.Label,
			Options:  options,
			Value:    n.Value,
			Disabled: n.Disabled,
		}.HTML()

	case app.ImageNode:
		// The encoded bytes ship inline as a data URI: self-contained (no
		// asset endpoint), and the sniffed MIME keeps the browser honest.
		if len(n.Encoded) == 0 {
			return ""
		}
		mime := http.DetectContentType(n.Encoded)
		if !strings.HasPrefix(mime, "image/") {
			return ""
		}
		return template.HTML(fmt.Sprintf(`<img class="trellis-image" alt="%s" data-max-width="%d" data-max-height="%d" src="data:%s;base64,%s">`,
			html.EscapeString(n.Alt), n.MaxWidth, n.MaxHeight, mime, base64.StdEncoding.EncodeToString(n.Encoded)))

	case app.ImageViewportNode:
		if len(n.Image.Encoded) == 0 {
			return ""
		}
		mime := http.DetectContentType(n.Image.Encoded)
		if !strings.HasPrefix(mime, "image/") {
			return ""
		}
		transform := n.Transform.Normalized()
		minScale, maxScale := n.MinScale, n.MaxScale
		if minScale <= 0 {
			minScale = 0.5
		}
		if maxScale < minScale {
			maxScale = math.Max(8, minScale)
		}
		width, height := n.Image.MaxWidth, n.Image.MaxHeight
		if width <= 0 {
			width = 640
		}
		if height <= 0 {
			height = 400
		}
		disabled, tabIndex := "false", "0"
		if n.Disabled {
			disabled, tabIndex = "true", "-1"
		}
		activateField := ""
		if n.OnActivate.Name != "" && !n.Disabled {
			activateField = fieldPrefix + path + "/activate"
		}
		var viewport strings.Builder
		viewport.WriteString(fmt.Sprintf(
			`<div class="trellis-image-viewport" role="group" aria-label="%s" tabindex="%s" data-poem-image-viewport data-field="%s" data-activate-field="%s" data-disabled="%s" data-x="%s" data-y="%s" data-scale="%s" data-min-scale="%s" data-max-scale="%s" data-render-width="%d" data-render-height="%d"><img alt="" draggable="false" src="data:%s;base64,%s">`,
			html.EscapeString(n.Image.Alt), tabIndex, html.EscapeString(fieldPrefix+path), html.EscapeString(activateField), disabled,
			app.FloatPayload(transform.OffsetX), app.FloatPayload(transform.OffsetY), app.FloatPayload(transform.Scale),
			app.FloatPayload(minScale), app.FloatPayload(maxScale), width, height, mime,
			base64.StdEncoding.EncodeToString(n.Image.Encoded)))
		for index, marker := range n.Markers {
			if marker.ID == "" || !validImageMarker(marker) {
				continue
			}
			fieldName := fieldPrefix + path + fmt.Sprintf("/marker-%d", index)
			classes := "trellis-image-marker trellis-image-marker--" + webVariantName(marker.Variant)
			if marker.Selected {
				classes += " is-selected"
			}
			disabledAttr := ""
			if marker.Disabled || n.Disabled || marker.OnActivate.Name == "" {
				disabledAttr = " disabled"
			}
			viewport.WriteString(fmt.Sprintf(`<button class="%s" type="submit" name="%s" value="%s" aria-label="%s" title="%s" data-poem-image-marker data-image-x="%s" data-image-y="%s"%s></button>`,
				classes, html.EscapeString(fieldName), html.EscapeString(marker.ID), html.EscapeString(marker.Label), html.EscapeString(marker.Label),
				app.FloatPayload(marker.X), app.FloatPayload(marker.Y), disabledAttr))
		}
		viewport.WriteString(`</div>`)
		return template.HTML(viewport.String())

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

	case app.CollectionNode:
		var out strings.Builder
		out.WriteString(fmt.Sprintf(`<section class="poem-collection" aria-label="%s"><div class="poem-collection__items">`, html.EscapeString(n.Semantic.Name)))
		for index, item := range n.Items {
			classes := "poem-collection-item"
			if item.Selected {
				classes += " is-selected"
			}
			out.WriteString(fmt.Sprintf(`<article class="%s" id="%s"><div class="poem-collection-item__content"><h3>%s</h3>`, classes, html.EscapeString(semanticPath(path, item.ID)), html.EscapeString(item.Title)))
			if item.Description != "" {
				out.WriteString(`<p>` + html.EscapeString(item.Description) + `</p>`)
			}
			if len(item.Metadata) > 0 {
				out.WriteString(`<dl>`)
				for _, field := range item.Metadata {
					out.WriteString(`<div><dt>` + html.EscapeString(field.Label) + `</dt><dd>` + html.EscapeString(field.Value) + `</dd></div>`)
				}
				out.WriteString(`</dl>`)
			}
			out.WriteString(`</div><div class="poem-collection-item__actions">`)
			for actionIndex, itemAction := range item.Actions {
				out.WriteString(string(renderNode(itemAction, childPath(childPath(path, index), actionIndex))))
			}
			out.WriteString(`</div></article>`)
		}
		out.WriteString(`</div></section>`)
		return template.HTML(out.String())

	case app.SectionNode:
		var body strings.Builder
		for i, child := range n.Children {
			body.WriteString(string(renderNode(child, childPath(path, i))))
		}
		description := ""
		if n.Description != "" {
			description = `<p class="poem-section__description">` + html.EscapeString(n.Description) + `</p>`
		}
		return template.HTML(fmt.Sprintf(`<section class="poem-section" aria-labelledby="%s-title"><header><h2 id="%s-title">%s</h2>%s</header><div class="poem-section__body">%s</div></section>`, html.EscapeString(path), html.EscapeString(path), html.EscapeString(n.Title), description, body.String()))

	case app.WorkspaceNode:
		var header, content, tools, status strings.Builder
		for i, child := range n.Header {
			header.WriteString(string(renderNode(child, childPath(path+"/header", i))))
		}
		for i, child := range n.Content {
			content.WriteString(string(renderNode(child, childPath(path+"/content", i))))
		}
		for i, child := range n.Tools {
			tools.WriteString(string(renderNode(child, childPath(path+"/tools", i))))
		}
		if n.Status != nil {
			status.WriteString(string(renderNode(n.Status, path+"/status")))
		}
		navigation := ""
		if n.Navigation != nil {
			navigation = string(renderNode(n.Navigation, path+"/navigation"))
		}
		return template.HTML(fmt.Sprintf(`<div class="poem-workspace" data-poem-workspace><header class="poem-workspace__header"><div><h1>%s</h1><p>%s</p></div><div class="poem-workspace__status">%s</div><div class="poem-workspace__actions">%s</div></header>%s<div class="poem-workspace__body"><main class="poem-workspace__content">%s</main><aside class="poem-workspace__tools" aria-label="Planning tools" data-poem-workspace-tools><button class="poem-workspace__handle" type="button" aria-label="Resize planning panel" data-poem-workspace-handle><span></span></button><div class="poem-workspace__tools-scroll">%s</div></aside></div></div>`, html.EscapeString(n.Title), html.EscapeString(n.Subtitle), status.String(), header.String(), navigation, content.String(), tools.String()))

	case app.ResponsiveNode:
		breakpoint := n.Breakpoint
		if breakpoint <= 0 {
			breakpoint = 600
		}
		var compact, wide strings.Builder
		for i, child := range n.Compact {
			compact.WriteString(string(renderNode(child, childPath(path+"/compact", i))))
		}
		for i, child := range n.Wide {
			wide.WriteString(string(renderNode(child, childPath(path+"/wide", i))))
		}
		return template.HTML(fmt.Sprintf(
			`<div class="trellis-responsive" data-poem-responsive data-breakpoint="%d"><fieldset data-poem-compact>%s</fieldset><fieldset data-poem-wide hidden disabled>%s</fieldset></div>`,
			breakpoint, compact.String(), wide.String()))

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
		fmt.Fprintf(&b, `<div class="%s trellis-gap-%d trellis-padding-%d">`, class, semanticSpacingClass(n.Gap), semanticSpacingClass(n.Padding))
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

func actionLabel(icon app.IconID, label string) string {
	if glyph := iconGlyph(icon); glyph != "" {
		return glyph + "  " + label
	}
	return label
}

func iconGlyph(icon app.IconID) string {
	switch icon {
	case app.IconSearch:
		return "⌕"
	case app.IconRoute, app.IconDirections:
		return "↝"
	case app.IconRefresh:
		return "↻"
	case app.IconAdd:
		return "+"
	case app.IconRemove:
		return "−"
	case app.IconSettings:
		return "⚙"
	case app.IconMore:
		return "⋯"
	case app.IconClose, app.IconClear:
		return "×"
	case app.IconMap:
		return "▧"
	case app.IconLocation:
		return "●"
	case app.IconSwap:
		return "⇅"
	case app.IconConnection:
		return "◉"
	default:
		return ""
	}
}

func actionWebVariant(node app.ActionNode) components.Variant {
	if node.Command != "" && node.Semantic.Error != "" {
		return components.VariantDanger
	}
	switch node.Importance {
	case app.ImportancePrimary:
		return components.VariantPrimary
	case app.ImportanceSubtle:
		return components.VariantNeutral
	default:
		return components.VariantSecondary
	}
}

func actionPlacementName(value app.ActionPlacement) string {
	switch value {
	case app.PlacementToolbar:
		return "toolbar"
	case app.PlacementContextual:
		return "contextual"
	case app.PlacementOverflow:
		return "overflow"
	default:
		return "content"
	}
}

// fieldKind selects how the /__event handler reads a posted field's value
// back out of the submitted form. Most fields are always present and carry
// their value directly (valueField); an HTML checkbox is only present in the
// POST body when checked, so its presence itself is the value
// (checkboxField) rather than something to read out of it.
type fieldKind int

func validImageMarker(marker app.ImageMarker) bool {
	return !math.IsNaN(marker.X) && !math.IsInf(marker.X, 0) && !math.IsNaN(marker.Y) && !math.IsInf(marker.Y, 0) &&
		marker.X >= 0 && marker.X <= 1 && marker.Y >= 0 && marker.Y <= 1
}

func webVariantName(variant app.Variant) string {
	switch variant {
	case app.VariantPrimary:
		return "primary"
	case app.VariantSecondary:
		return "secondary"
	case app.VariantDanger:
		return "danger"
	case app.VariantSuccess:
		return "success"
	case app.VariantWarning:
		return "warning"
	default:
		return "neutral"
	}
}

const (
	valueField fieldKind = iota
	checkboxField
	imageTransformField
	viewportPointField
	imageMarkerField
)

// postedField pairs the Msg an interactive Node fires with how to read its
// current value back out of a form submission.
type postedField struct {
	msg      app.Msg
	kind     fieldKind
	minScale float64
	maxScale float64
	expected string
}

func (f postedField) validImageTransform(payload string) bool {
	transform, ok := (app.Msg{Payload: payload}).ImageTransform()
	return ok && transform.Scale >= f.minScale && transform.Scale <= f.maxScale
}

func (f postedField) validPayload(payload string) bool {
	switch f.kind {
	case imageTransformField:
		return f.validImageTransform(payload)
	case viewportPointField:
		_, ok := (app.Msg{Payload: payload}).ViewportPoint()
		return ok
	case imageMarkerField:
		return payload != "" && payload == f.expected
	default:
		return true
	}
}

// collectFields walks node collecting every interactive Node's posted field
// name mapped to how to read and dispatch it, so the /__event handler can
// translate a posted form into Update calls before applying the message that
// actually submitted the form.
func collectFields(node app.Node, path string, out map[string]postedField) {
	switch n := node.(type) {
	case app.IdentityNode:
		collectFields(n.Child, semanticPath(path, n.Semantic.ID), out)
	case app.CollectionNode:
		for itemIndex, item := range n.Items {
			for actionIndex, action := range item.Actions {
				collectFields(action, childPath(childPath(path, itemIndex), actionIndex), out)
			}
		}
	case app.SectionNode:
		for i, child := range n.Children {
			collectFields(child, childPath(path, i), out)
		}
	case app.WorkspaceNode:
		for i, child := range n.Header {
			collectFields(child, childPath(path+"/header", i), out)
		}
		if n.Navigation != nil {
			collectFields(n.Navigation, path+"/navigation", out)
		}
		if n.Status != nil {
			collectFields(n.Status, path+"/status", out)
		}
		for i, child := range n.Content {
			collectFields(child, childPath(path+"/content", i), out)
		}
		for i, child := range n.Tools {
			collectFields(child, childPath(path+"/tools", i), out)
		}

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
	case app.ImageViewportNode:
		if !n.Disabled {
			minScale, maxScale := n.MinScale, n.MaxScale
			if minScale <= 0 {
				minScale = 0.5
			}
			if maxScale < minScale {
				maxScale = math.Max(8, minScale)
			}
			out[fieldPrefix+path] = postedField{msg: n.OnChange, kind: imageTransformField, minScale: minScale, maxScale: maxScale}
			if n.OnActivate.Name != "" {
				out[fieldPrefix+path+"/activate"] = postedField{msg: n.OnActivate, kind: viewportPointField}
			}
			for index, marker := range n.Markers {
				if marker.ID != "" && !marker.Disabled && marker.OnActivate.Name != "" && validImageMarker(marker) {
					out[fieldPrefix+path+fmt.Sprintf("/marker-%d", index)] = postedField{msg: marker.OnActivate, kind: imageMarkerField, expected: marker.ID}
				}
			}
		}
	case app.ContainerNode:
		for i, child := range n.Children {
			collectFields(child, childPath(path, i), out)
		}
	case app.ResponsiveNode:
		for i, child := range n.Compact {
			collectFields(child, childPath(path+"/compact", i), out)
		}
		for i, child := range n.Wide {
			collectFields(child, childPath(path+"/wide", i), out)
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
	case app.IdentityNode:
		collectMessages(n.Child, out)
	case app.ActionNode:
		if n.Semantic.Enabled && !n.Semantic.Running && n.Invoke.Name != "" {
			out[n.Invoke.Name] = true
		}
	case app.CollectionNode:
		for _, item := range n.Items {
			for _, action := range item.Actions {
				collectMessages(action, out)
			}
		}
	case app.SectionNode:
		for _, child := range n.Children {
			collectMessages(child, out)
		}
	case app.WorkspaceNode:
		for _, child := range n.Header {
			collectMessages(child, out)
		}
		if n.Navigation != nil {
			collectMessages(n.Navigation, out)
		}
		if n.Status != nil {
			collectMessages(n.Status, out)
		}
		for _, child := range n.Content {
			collectMessages(child, out)
		}
		for _, child := range n.Tools {
			collectMessages(child, out)
		}

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
	case app.ImageViewportNode:
		if !n.Disabled && n.OnChange.Name != "" {
			out[n.OnChange.Name] = true
		}
	case app.ContainerNode:
		for _, child := range n.Children {
			collectMessages(child, out)
		}
	case app.ResponsiveNode:
		for _, child := range n.Compact {
			collectMessages(child, out)
		}
		for _, child := range n.Wide {
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

func semanticPath(parent, id string) string {
	if id == "" {
		return parent
	}
	return parent + "-id-" + id
}

func semanticSpacingClass(value int) int {
	switch {
	case value <= 0:
		return 0
	case value <= 4:
		return 1
	case value <= 8:
		return 2
	case value <= 12:
		return 3
	case value <= 16:
		return 4
	default:
		return 5
	}
}
