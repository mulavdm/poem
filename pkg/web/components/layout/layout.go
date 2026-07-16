// Package layout renders structural content components.
package layout

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components/action"
	"github.com/mulavdm/poem/pkg/web/components/internal/markup"
)

// Card renders a framed content item. HTML fields are caller-trusted.
type Card struct {
	Title       string
	Description string
	Body        string
	BodyHTML    template.HTML
	Footer      string
	FooterHTML  template.HTML
}

func (c Card) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<article class="poem-card">`)
	if c.Title != "" || c.Description != "" {
		b.WriteString(`<header class="poem-card__header">`)
		if c.Title != "" {
			b.WriteString(`<h3>` + markup.Text(c.Title) + `</h3>`)
		}
		if c.Description != "" {
			b.WriteString(`<p>` + markup.Text(c.Description) + `</p>`)
		}
		b.WriteString(`</header>`)
	}
	if c.BodyHTML != "" || c.Body != "" {
		b.WriteString(`<div class="poem-card__body">`)
		if c.BodyHTML != "" {
			b.WriteString(string(c.BodyHTML))
		} else {
			b.WriteString(markup.Text(c.Body))
		}
		b.WriteString(`</div>`)
	}
	if c.FooterHTML != "" || c.Footer != "" {
		b.WriteString(`<footer class="poem-card__footer">`)
		if c.FooterHTML != "" {
			b.WriteString(string(c.FooterHTML))
		} else {
			b.WriteString(markup.Text(c.Footer))
		}
		b.WriteString(`</footer>`)
	}
	b.WriteString(`</article>`)
	return markup.Markup(b.String())
}

// EmptyState renders a no-data message and optional action.
type EmptyState struct {
	Title       string
	Description string
	Action      action.Button
}

func (e EmptyState) HTML() template.HTML {
	return markup.Markup(fmt.Sprintf(`<section class="poem-empty"><h3>%s</h3><p>%s</p>%s</section>`, markup.Text(e.Title), markup.Text(e.Description), e.Action.HTML()))
}

// Toolbar renders a title with action buttons.
type Toolbar struct {
	Title   string
	Actions []action.Button
}

func (t Toolbar) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="poem-toolbar"><h2>` + markup.Text(t.Title) + `</h2><div class="poem-toolbar__actions">`)
	for _, a := range t.Actions {
		b.WriteString(string(a.HTML()))
	}
	b.WriteString(`</div></div>`)
	return markup.Markup(b.String())
}

// StatCard renders a labelled metric.
type StatCard struct {
	Label string
	Value string
	Delta string
}

func (s StatCard) HTML() template.HTML {
	return markup.Markup(fmt.Sprintf(`<article class="poem-stat"><span>%s</span><strong>%s</strong><small>%s</small></article>`, markup.Text(s.Label), markup.Text(s.Value), markup.Text(s.Delta)))
}
