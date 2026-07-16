// Package action renders common user actions.
package action

import (
	"fmt"
	"html/template"

	"github.com/mulavdm/poem/pkg/web/components"
	"github.com/mulavdm/poem/pkg/web/components/internal/markup"
)

// Button renders a button or a safe link.
type Button struct {
	Text       string
	Type       string
	Href       string
	Variant    components.Variant
	Size       components.Size
	Disabled   bool
	Attributes components.Attrs
}

// HTML returns Button markup.
func (b Button) HTML() template.HTML {
	classes := markup.ClassList("poem-button", markup.VariantClass("poem-button", b.Variant), markup.SizeClass("poem-button", b.Size))
	if b.Href != "" {
		return markup.Markup(fmt.Sprintf(`<a class="%s" href="%s"%s>%s</a>`, classes, markup.Attr(markup.SafeURL(b.Href)), markup.Attributes(b.Attributes), markup.Text(b.Text)))
	}
	typ := b.Type
	if typ == "" {
		typ = "button"
	}
	return markup.Markup(fmt.Sprintf(`<button class="%s" type="%s"%s%s>%s</button>`, classes, markup.Attr(typ), markup.Bool("disabled", b.Disabled), markup.Attributes(b.Attributes), markup.Text(b.Text)))
}
