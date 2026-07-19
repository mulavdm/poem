// Package form renders semantic form controls and actions.
package form

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"github.com/mulavdm/poem/pkg/web/components/action"
	"github.com/mulavdm/poem/pkg/web/components/internal/markup"
)

// Field groups a labelled form control. ControlHTML is caller-trusted composition markup.
type Field struct {
	ID          string
	Label       string
	Help        string
	Error       string
	ControlHTML template.HTML
}

func (f Field) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="poem-field">`)
	if f.Label != "" {
		b.WriteString(`<label class="poem-field__label"`)
		if f.ID != "" {
			b.WriteString(` for="` + markup.Attr(f.ID) + `"`)
		}
		b.WriteString(`>`)
		b.WriteString(markup.Text(f.Label))
		b.WriteString(`</label>`)
	}
	b.WriteString(string(f.ControlHTML))
	if f.Help != "" {
		b.WriteString(`<span class="poem-field__help">`)
		b.WriteString(markup.Text(f.Help))
		b.WriteString(`</span>`)
	}
	if f.Error != "" {
		b.WriteString(`<span class="poem-field__error" role="alert">`)
		b.WriteString(markup.Text(f.Error))
		b.WriteString(`</span>`)
	}
	b.WriteString(`</div>`)
	return markup.Markup(b.String())
}

// Input renders a labelled input control.
type Input struct {
	ID          string
	Name        string
	Label       string
	Type        string
	Value       string
	Placeholder string
	Required    bool
	Disabled    bool
	ReadOnly    bool
	Help        string
	Error       string
}

func (i Input) HTML() template.HTML {
	id := i.ID
	if id == "" {
		id = i.Name
	}
	typ := i.Type
	if typ == "" {
		typ = "text"
	}
	control := fmt.Sprintf(`<input class="poem-input" id="%s" name="%s" type="%s" value="%s" placeholder="%s"%s%s%s>`, markup.Attr(id), markup.Attr(i.Name), markup.Attr(typ), markup.Attr(i.Value), markup.Attr(i.Placeholder), markup.Bool("required", i.Required), markup.Bool("disabled", i.Disabled), markup.Bool("readonly", i.ReadOnly))
	return Field{ID: id, Label: i.Label, Help: i.Help, Error: i.Error, ControlHTML: markup.Markup(control)}.HTML()
}

// Textarea renders a labelled multiline text control.
type Textarea struct {
	ID          string
	Name        string
	Label       string
	Value       string
	Placeholder string
	Required    bool
	Disabled    bool
	Help        string
	Error       string
	Rows        int
}

func (t Textarea) HTML() template.HTML {
	id := t.ID
	if id == "" {
		id = t.Name
	}
	rows := t.Rows
	if rows == 0 {
		rows = 4
	}
	control := fmt.Sprintf(`<textarea class="poem-textarea" id="%s" name="%s" rows="%d" placeholder="%s"%s%s>%s</textarea>`, markup.Attr(id), markup.Attr(t.Name), rows, markup.Attr(t.Placeholder), markup.Bool("required", t.Required), markup.Bool("disabled", t.Disabled), markup.Text(t.Value))
	return Field{ID: id, Label: t.Label, Help: t.Help, Error: t.Error, ControlHTML: markup.Markup(control)}.HTML()
}

// Option is an escaped Select choice.
type Option struct {
	Label string
	Value string
}

// Select renders a labelled select control.
type Select struct {
	ID       string
	Name     string
	Label    string
	Options  []Option
	Value    string
	Required bool
	Disabled bool
	Help     string
	Error    string
}

func (s Select) HTML() template.HTML {
	id := s.ID
	if id == "" {
		id = s.Name
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<select class="poem-select" id="%s" name="%s"%s%s>`, markup.Attr(id), markup.Attr(s.Name), markup.Bool("required", s.Required), markup.Bool("disabled", s.Disabled)))
	for _, o := range s.Options {
		b.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, markup.Attr(o.Value), markup.Bool("selected", o.Value == s.Value), markup.Text(o.Label)))
	}
	b.WriteString(`</select>`)
	return Field{ID: id, Label: s.Label, Help: s.Help, Error: s.Error, ControlHTML: markup.Markup(b.String())}.HTML()
}

// Checkbox renders a labelled checkbox control.
type Checkbox struct {
	Name     string
	Label    string
	Checked  bool
	Disabled bool
	Help     string
}

func (c Checkbox) HTML() template.HTML {
	return markup.Markup(fmt.Sprintf(`<label class="poem-checkbox"><input name="%s" type="checkbox"%s%s><span>%s</span><small>%s</small></label>`, markup.Attr(c.Name), markup.Bool("checked", c.Checked), markup.Bool("disabled", c.Disabled), markup.Text(c.Label), markup.Text(c.Help)))
}

// Range renders a labelled slider control. Its posted value is the numeric
// position as a string, read back the same value-carrying way as Input; Step
// is emitted only when positive so the browser keeps its default otherwise.
type Range struct {
	ID       string
	Name     string
	Label    string
	Min      float64
	Max      float64
	Step     float64
	Value    float64
	Disabled bool
	Help     string
}

func (r Range) HTML() template.HTML {
	id := r.ID
	if id == "" {
		id = r.Name
	}
	step := ""
	if r.Step > 0 {
		step = fmt.Sprintf(` step="%s"`, formatNumber(r.Step))
	}
	control := fmt.Sprintf(`<input class="poem-range" id="%s" name="%s" type="range" min="%s" max="%s"%s value="%s"%s>`, markup.Attr(id), markup.Attr(r.Name), formatNumber(r.Min), formatNumber(r.Max), step, formatNumber(r.Value), markup.Bool("disabled", r.Disabled))
	return Field{ID: id, Label: r.Label, Help: r.Help, ControlHTML: markup.Markup(control)}.HTML()
}

// formatNumber renders a float as a plain decimal with no trailing zeros and
// no scientific notation, suitable for an HTML numeric attribute.
func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// RadioGroup renders a labelled set of mutually exclusive choices. Every
// option shares the group's Name, so a no-JavaScript form submission posts
// exactly one selected value under that name — the same value-carrying
// mechanics as Select, presented as radios instead of a dropdown.
type RadioGroup struct {
	Name     string
	Label    string
	Options  []Option
	Value    string
	Disabled bool
	Help     string
	Error    string
}

func (g RadioGroup) HTML() template.HTML {
	var b strings.Builder
	b.WriteString(`<fieldset class="poem-radiogroup">`)
	if g.Label != "" {
		b.WriteString(`<legend class="poem-radiogroup__legend">` + markup.Text(g.Label) + `</legend>`)
	}
	for _, o := range g.Options {
		b.WriteString(fmt.Sprintf(`<label class="poem-radio"><input name="%s" type="radio" value="%s"%s%s><span>%s</span></label>`, markup.Attr(g.Name), markup.Attr(o.Value), markup.Bool("checked", o.Value == g.Value), markup.Bool("disabled", g.Disabled), markup.Text(o.Label)))
	}
	b.WriteString(`</fieldset>`)
	// A fieldset carries its own legend, so Field's <label> is suppressed
	// (empty Label) to avoid labelling the group twice; Help/Error still render.
	return Field{Label: "", Help: g.Help, Error: g.Error, ControlHTML: markup.Markup(b.String())}.HTML()
}

// Switch renders a labelled on/off toggle. It is a checkbox with switch
// semantics: it posts exactly like Checkbox (present when on, absent when
// off), so a no-JavaScript form submission reads it the same way, while
// role="switch" and its own styling present it as a toggle.
type Switch struct {
	Name     string
	Label    string
	Checked  bool
	Disabled bool
	Help     string
}

func (s Switch) HTML() template.HTML {
	return markup.Markup(fmt.Sprintf(`<label class="poem-switch"><input name="%s" type="checkbox" role="switch"%s%s><span class="poem-switch__track" aria-hidden="true"></span><span class="poem-switch__label">%s</span><small>%s</small></label>`, markup.Attr(s.Name), markup.Bool("checked", s.Checked), markup.Bool("disabled", s.Disabled), markup.Text(s.Label), markup.Text(s.Help)))
}

// FormActions renders primary and secondary action buttons.
type FormActions struct {
	Primary   action.Button
	Secondary action.Button
}

func (f FormActions) HTML() template.HTML {
	return markup.Markup(fmt.Sprintf(`<div class="poem-form-actions">%s%s</div>`, f.Secondary.HTML(), f.Primary.HTML()))
}
