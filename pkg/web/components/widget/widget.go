// Package widget renders progressively enhanced interactive components.
package widget

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components"
	"github.com/mulavdm/poem/pkg/web/components/action"
	"github.com/mulavdm/poem/pkg/web/components/internal/markup"
)

// TabItem describes one Tabs panel. Content is caller-trusted HTML.
type TabItem struct {
	ID      string
	Label   string
	Text    string
	Content template.HTML
	Active  bool
}

// Tabs renders progressive-enhancement tab controls.
type Tabs struct {
	Label string
	Items []TabItem
}

func (t Tabs) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<section class="poem-tabs" data-poem-tabs>`)
	b.WriteString(fmt.Sprintf(`<div class="poem-tabs__list" role="tablist" aria-label="%s">`, markup.Attr(t.Label)))
	activeFound := false
	for i, item := range t.Items {
		id := item.ID
		if id == "" {
			id = fmt.Sprintf("poem-tab-%d", i+1)
		}
		active := item.Active || (!activeFound && i == 0)
		if active {
			activeFound = true
		}
		b.WriteString(fmt.Sprintf(`<button class="poem-tabs__tab" id="%s-tab" type="button" role="tab" aria-selected="%t" aria-controls="%s" data-poem-tab="%s" tabindex="%d">%s</button>`, markup.Attr(id), active, markup.Attr(id), markup.Attr(id), markup.TabIndex(active), markup.Text(item.Label)))
	}
	b.WriteString(`</div>`)
	activeFound = false
	for i, item := range t.Items {
		id := item.ID
		if id == "" {
			id = fmt.Sprintf("poem-tab-%d", i+1)
		}
		active := item.Active || (!activeFound && i == 0)
		if active {
			activeFound = true
		}
		content := string(item.Content)
		if item.Content == "" {
			content = markup.Text(item.Text)
		}
		b.WriteString(fmt.Sprintf(`<div class="poem-tabs__panel" id="%s" role="tabpanel" aria-labelledby="%s-tab"%s>%s</div>`, markup.Attr(id), markup.Attr(id), markup.Bool("hidden", !active), content))
	}
	b.WriteString(`</section>`)
	return markup.Markup(b.String())
}

// FormTabs renders tabs whose selection travels as a form submission rather
// than a client-side toggle. Each tab is a submit button posting Name=<item ID>,
// and only the selected panel is rendered — so the server owns which tab is
// active and it works with JavaScript disabled. This is the server-driven
// sibling of Tabs: use Tabs when the selection is purely client-side chrome,
// FormTabs when the application owns it. PanelHTML is caller-trusted HTML.
type FormTabs struct {
	Label     string
	Name      string
	Items     []TabItem
	Selected  string
	PanelHTML template.HTML
}

func (t FormTabs) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<section class="poem-tabs">`)
	b.WriteString(fmt.Sprintf(`<div class="poem-tabs__list" role="tablist" aria-label="%s">`, markup.Attr(t.Label)))
	selected := t.Selected
	if selected == "" && len(t.Items) > 0 {
		selected = t.Items[0].ID
	}
	panelID := markup.Attr(t.Name) + "-panel"
	for _, item := range t.Items {
		active := item.ID == selected
		b.WriteString(fmt.Sprintf(`<button class="poem-tabs__tab" type="submit" role="tab" name="%s" value="%s" aria-selected="%t" aria-controls="%s">%s</button>`,
			markup.Attr(t.Name), markup.Attr(item.ID), active, panelID, markup.Text(item.Label)))
	}
	b.WriteString(`</div>`)
	b.WriteString(fmt.Sprintf(`<div class="poem-tabs__panel" id="%s" role="tabpanel">%s</div>`, panelID, string(t.PanelHTML)))
	b.WriteString(`</section>`)
	return markup.Markup(b.String())
}

// Modal renders a dialog. ContentHTML is caller-trusted HTML.
type Modal struct {
	ID          string
	Title       string
	Content     string
	ContentHTML template.HTML
	Trigger     action.Button
}

func (m Modal) HTML() template.HTML {
	id := m.ID
	if id == "" {
		id = "poem-modal"
	}
	trigger := m.Trigger
	if trigger.Text == "" {
		trigger.Text = "Open modal"
	}
	attrs := cloneAttrs(trigger.Attributes)
	attrs["data-poem-modal-open"] = id
	trigger.Attributes = attrs
	content := string(m.ContentHTML)
	if m.ContentHTML == "" {
		content = markup.Text(m.Content)
	}
	return markup.Markup(fmt.Sprintf(`%s<div class="poem-modal" id="%s" role="dialog" aria-modal="true" aria-labelledby="%s-title" aria-describedby="%s-body" hidden><div class="poem-modal__panel" tabindex="-1"><header><h3 id="%s-title">%s</h3><button type="button" class="poem-modal__close" data-poem-modal-close aria-label="Close modal">x</button></header><div class="poem-modal__body" id="%s-body">%s</div></div></div>`, trigger.HTML(), markup.Attr(id), markup.Attr(id), markup.Attr(id), markup.Attr(id), markup.Text(m.Title), markup.Attr(id), content))
}

// Disclosure renders a native expandable disclosure. Content is caller-trusted HTML.
type Disclosure struct {
	Summary string
	Text    string
	Content template.HTML
	Open    bool
}

func (d Disclosure) HTML() template.HTML {
	content := string(d.Content)
	if d.Content == "" {
		content = markup.Text(d.Text)
	}
	return markup.Markup(fmt.Sprintf(`<details class="poem-disclosure" data-poem-disclosure%s><summary>%s</summary><div class="poem-disclosure__body">%s</div></details>`, markup.Bool("open", d.Open), markup.Text(d.Summary), content))
}

// CommandItem describes a CommandPalette action.
type CommandItem struct {
	Label string
	Value string
}

// CommandPalette renders a keyboard-enhanced command list.
type CommandPalette struct {
	ID    string
	Items []CommandItem
}

func (c CommandPalette) HTML() template.HTML {
	id := c.ID
	if id == "" {
		id = "poem-command-palette"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<section class="poem-command" id="%s" data-poem-command><label><span>Command palette</span><input class="poem-input" type="search" data-poem-command-input placeholder="Type a command" role="combobox" aria-expanded="true" aria-controls="%s-list" aria-autocomplete="list"></label><ul id="%s-list" role="listbox" data-poem-command-list>`, markup.Attr(id), markup.Attr(id), markup.Attr(id)))
	for _, item := range c.Items {
		b.WriteString(fmt.Sprintf(`<li role="option"><button type="button" data-value="%s">%s</button></li>`, markup.Attr(item.Value), markup.Text(item.Label)))
	}
	b.WriteString(`</ul></section>`)
	return markup.Markup(b.String())
}

func cloneAttrs(attrs components.Attrs) components.Attrs {
	cloned := make(components.Attrs, len(attrs)+1)
	for key, value := range attrs {
		cloned[key] = value
	}
	return cloned
}
