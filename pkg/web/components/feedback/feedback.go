// Package feedback renders user-facing status and notification components.
package feedback

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components"
	"github.com/mulavdm/poem/pkg/web/components/internal/markup"
)

// Badge renders a compact semantic status label.
type Badge struct {
	Text    string
	Variant components.Variant
}

func (b Badge) HTML() template.HTML {
	return markup.Markup(fmt.Sprintf(`<span class="%s">%s</span>`, markup.ClassList("poem-badge", markup.VariantClass("poem-badge", b.Variant)), markup.Text(b.Text)))
}

// Alert renders a status message. BodyHTML is caller-trusted HTML.
type Alert struct {
	Title       string
	Text        string
	BodyHTML    template.HTML
	Variant     components.Variant
	Dismissible bool
}

func (a Alert) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<section class="`)
	b.WriteString(markup.ClassList("poem-alert", markup.VariantClass("poem-alert", a.Variant)))
	b.WriteString(`" role="status" aria-live="polite">`)
	if a.Title != "" {
		b.WriteString(`<strong class="poem-alert__title">`)
		b.WriteString(markup.Text(a.Title))
		b.WriteString(`</strong>`)
	}
	if a.BodyHTML != "" {
		b.WriteString(`<div class="poem-alert__body">`)
		b.WriteString(string(a.BodyHTML))
		b.WriteString(`</div>`)
	} else if a.Text != "" {
		b.WriteString(`<p class="poem-alert__body">`)
		b.WriteString(markup.Text(a.Text))
		b.WriteString(`</p>`)
	}
	if a.Dismissible {
		b.WriteString(`<button class="poem-alert__dismiss" type="button" data-poem-dismiss aria-label="Dismiss alert">x</button>`)
	}
	b.WriteString(`</section>`)
	return markup.Markup(b.String())
}

// Progress renders a determinate or indeterminate progress indicator using
// the native <progress> element, so it needs no JavaScript and is announced by
// assistive technology out of the box. When Indeterminate is set the value
// attribute is omitted, which the browser renders as an ongoing activity bar.
type Progress struct {
	Label         string
	Value         float64
	Max           float64
	Indeterminate bool
}

func (p Progress) HTML() template.HTML {
	max := p.Max
	if max <= 0 {
		max = 100
	}
	value := ""
	if !p.Indeterminate {
		value = fmt.Sprintf(` value="%s"`, strconv.FormatFloat(p.Value, 'f', -1, 64))
	}
	label := ""
	if p.Label != "" {
		label = fmt.Sprintf(` aria-label="%s"`, markup.Attr(p.Label))
	}
	return markup.Markup(fmt.Sprintf(`<progress class="poem-progress" max="%s"%s%s></progress>`, strconv.FormatFloat(max, 'f', -1, 64), value, label))
}

// Toast renders a live status message.
type Toast struct {
	ID          string
	Message     string
	Variant     components.Variant
	Dismissible bool
}

func (t Toast) HTML() template.HTML {
	if t.ID == "" {
		t.ID = "poem-toast"
	}
	var b strings.Builder
	b.WriteString(`<div class="`)
	b.WriteString(markup.ClassList("poem-toast", markup.VariantClass("poem-toast", t.Variant)))
	b.WriteString(`" id="`)
	b.WriteString(markup.Attr(t.ID))
	b.WriteString(`" role="status" aria-live="polite" data-poem-toast>`)
	b.WriteString(markup.Text(t.Message))
	if t.Dismissible {
		b.WriteString(`<button class="poem-toast__dismiss" type="button" data-poem-dismiss aria-label="Dismiss notification">x</button>`)
	}
	b.WriteString(`</div>`)
	return markup.Markup(b.String())
}
