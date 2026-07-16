// Package data renders tabular data components.
package data

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components/internal/markup"
)

// TableColumn describes a Table column.
type TableColumn struct {
	Key   string
	Label string
}

// TableRow maps column keys to cell values.
type TableRow map[string]string

// Table renders tabular data with optional client-side enhancement.
type Table struct {
	Caption    string
	Columns    []TableColumn
	Rows       []TableRow
	Filterable bool
	Sortable   bool
}

func (t Table) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="poem-table-wrap"`)
	if t.Filterable {
		b.WriteString(` data-poem-table-filterable`)
	}
	if t.Sortable {
		b.WriteString(` data-poem-table-sortable`)
	}
	b.WriteString(`>`)
	if t.Filterable {
		b.WriteString(`<label class="poem-table-filter"><span>Filter table</span><input class="poem-input" type="search" data-poem-table-filter></label>`)
	}
	b.WriteString(`<table class="poem-table">`)
	if t.Caption != "" {
		b.WriteString(`<caption>` + markup.Text(t.Caption) + `</caption>`)
	}
	b.WriteString(`<thead><tr>`)
	for _, c := range t.Columns {
		b.WriteString(`<th scope="col"`)
		if t.Sortable {
			b.WriteString(` aria-sort="none"`)
		}
		b.WriteString(`>`)
		if t.Sortable {
			b.WriteString(fmt.Sprintf(`<button type="button" data-poem-sort="%s">%s</button>`, markup.Attr(c.Key), markup.Text(c.Label)))
		} else {
			b.WriteString(markup.Text(c.Label))
		}
		b.WriteString(`</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range t.Rows {
		b.WriteString(`<tr>`)
		for _, c := range t.Columns {
			b.WriteString(`<td>` + markup.Text(row[c.Key]) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return markup.Markup(b.String())
}
