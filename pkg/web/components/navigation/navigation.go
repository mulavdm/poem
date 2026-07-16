// Package navigation renders navigational components.
package navigation

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components/internal/markup"
)

// NavItem is a Navbar entry.
type NavItem struct {
	Label  string
	Href   string
	Active bool
}

// Navbar renders primary navigation.
type Navbar struct {
	Brand string
	Items []NavItem
}

func (n Navbar) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<nav class="poem-navbar" aria-label="Primary"><a class="poem-navbar__brand" href="/">` + markup.Text(n.Brand) + `</a><div class="poem-navbar__items">`)
	for _, item := range n.Items {
		current := ""
		if item.Active {
			current = ` aria-current="page"`
		}
		b.WriteString(fmt.Sprintf(`<a class="poem-navbar__item" href="%s"%s>%s</a>`, markup.Attr(markup.SafeURL(item.Href)), current, markup.Text(item.Label)))
	}
	b.WriteString(`</div></nav>`)
	return markup.Markup(b.String())
}

// BreadcrumbItem is a Breadcrumbs entry.
type BreadcrumbItem struct {
	Label string
	Href  string
}

// Breadcrumbs renders hierarchical navigation.
type Breadcrumbs struct{ Items []BreadcrumbItem }

func (b Breadcrumbs) HTML() template.HTML {
	var out strings.Builder
	out.WriteString(`<nav class="poem-breadcrumbs" aria-label="Breadcrumb"><ol>`)
	for i, item := range b.Items {
		out.WriteString(`<li>`)
		if item.Href != "" && i < len(b.Items)-1 {
			out.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, markup.Attr(markup.SafeURL(item.Href)), markup.Text(item.Label)))
		} else {
			out.WriteString(markup.Text(item.Label))
		}
		out.WriteString(`</li>`)
	}
	out.WriteString(`</ol></nav>`)
	return markup.Markup(out.String())
}

// Pagination renders a windowed set of page links, with the first and last
// page always reachable and an ellipsis marking any skipped range.
type Pagination struct {
	Current int
	Total   int
	BaseURL string

	// MaxButtons caps how many numbered page links appear at once, in
	// addition to the always-shown first/last page. Defaults to 7.
	MaxButtons int
}

func (p Pagination) HTML() template.HTML {
	if p.Total <= 0 {
		return ""
	}
	maxButtons := p.MaxButtons
	if maxButtons <= 0 {
		maxButtons = 7
	}
	if maxButtons > p.Total {
		maxButtons = p.Total
	}
	start := p.Current - maxButtons/2
	if start < 1 {
		start = 1
	}
	end := start + maxButtons - 1
	if end > p.Total {
		end = p.Total
		start = end - maxButtons + 1
	}

	var b strings.Builder
	b.WriteString(`<nav class="poem-pagination" aria-label="Pagination">`)
	link := func(page int) {
		current := ""
		if page == p.Current {
			current = ` aria-current="page"`
		}
		b.WriteString(fmt.Sprintf(`<a href="%s?page=%d"%s>%d</a>`, markup.Attr(markup.SafeURL(p.BaseURL)), page, current, page))
	}
	if start > 1 {
		link(1)
		if start > 2 {
			b.WriteString(`<span class="poem-pagination__ellipsis">&hellip;</span>`)
		}
	}
	for page := start; page <= end; page++ {
		link(page)
	}
	if end < p.Total {
		if end < p.Total-1 {
			b.WriteString(`<span class="poem-pagination__ellipsis">&hellip;</span>`)
		}
		link(p.Total)
	}
	b.WriteString(`</nav>`)
	return markup.Markup(b.String())
}
