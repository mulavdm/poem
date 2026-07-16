// Package markup contains rendering helpers shared by GopherWeb component families.
package markup

import (
	"html"
	"html/template"
	"net/url"
	"sort"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components"
)

// Text escapes plain text for HTML content.
func Text(value string) string { return html.EscapeString(value) }

// Attr escapes an HTML attribute value.
func Attr(value string) string { return html.EscapeString(value) }

// Markup converts internally constructed, escaped markup into template.HTML.
func Markup(value string) template.HTML { return template.HTML(value) }

// ClassList joins non-empty CSS classes.
func ClassList(values ...string) string {
	classes := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			classes = append(classes, value)
		}
	}
	return strings.Join(classes, " ")
}

// Attributes renders supported attributes in deterministic order.
func Attributes(attrs components.Attrs) string {
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		if !safeAttrName(key) {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteString(`="`)
		b.WriteString(Attr(attrs[key]))
		b.WriteByte('"')
	}
	return b.String()
}

// SafeURL restricts links to local, fragment, HTTP(S), mailto, and telephone URLs.
func SafeURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "#"
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "#") {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "#"
	}
	switch parsed.Scheme {
	case "http", "https", "mailto", "tel":
		return value
	default:
		return "#"
	}
}

// VariantClass returns a validated variant class.
func VariantClass(prefix string, variant components.Variant) string {
	if !validVariant(variant) {
		variant = components.VariantNeutral
	}
	return prefix + "--" + string(variant)
}

// SizeClass returns a validated size class.
func SizeClass(prefix string, size components.Size) string {
	if !validSize(size) {
		size = components.SizeMedium
	}
	return prefix + "--" + string(size)
}

// Bool returns an HTML boolean attribute when enabled.
func Bool(name string, enabled bool) string {
	if !enabled {
		return ""
	}
	return " " + name
}

// TabIndex returns the roving tab index for an active item.
func TabIndex(active bool) int {
	if active {
		return 0
	}
	return -1
}

func safeAttrName(name string) bool {
	if name == "" || strings.HasPrefix(strings.ToLower(name), "on") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ':' {
			continue
		}
		return false
	}
	return true
}

func validVariant(variant components.Variant) bool {
	switch variant {
	case components.VariantPrimary, components.VariantSecondary, components.VariantDanger, components.VariantSuccess, components.VariantWarning, components.VariantNeutral:
		return true
	default:
		return false
	}
}

func validSize(size components.Size) bool {
	switch size {
	case components.SizeSmall, components.SizeMedium, components.SizeLarge:
		return true
	default:
		return false
	}
}
