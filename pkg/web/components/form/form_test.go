package form

import (
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/web/components/action"
)

func TestControlsRenderLabelsAndValidation(t *testing.T) {
	html := string(Input{Name: "email", Label: "Email", Required: true, Error: "Required"}.HTML())
	for _, wanted := range []string{`for="email"`, `id="email"`, "required", `role="alert"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
}
func TestFormActionsUseActionButtons(t *testing.T) {
	if !strings.Contains(string(FormActions{Primary: action.Button{Text: "Save"}}.HTML()), "poem-form-actions") {
		t.Fatal("form actions missing class")
	}
}

func TestSwitchRendersToggleSemantics(t *testing.T) {
	html := string(Switch{Name: "wifi", Label: "Wi-Fi", Checked: true}.HTML())
	for _, wanted := range []string{`class="poem-switch"`, `type="checkbox"`, `role="switch"`, "checked", "Wi-Fi"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
	if strings.Contains(string(Switch{Name: "wifi", Label: "Wi-Fi"}.HTML()), "checked") {
		t.Fatal("Switch: checked attribute should be absent when Checked is false")
	}
}

func TestRadioGroupRendersSharedNameAndSelection(t *testing.T) {
	html := string(RadioGroup{
		Name:  "plan",
		Label: "Plan",
		Value: "pro",
		Options: []Option{
			{Label: "Free", Value: "free"},
			{Label: "Pro", Value: "pro"},
		},
	}.HTML())
	for _, wanted := range []string{`class="poem-radiogroup"`, "<legend", `name="plan"`, `type="radio"`, `value="free"`, `value="pro"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
	// Exactly the selected option carries checked.
	if strings.Count(html, "checked") != 1 {
		t.Fatalf("expected exactly one checked radio, got %d: %q", strings.Count(html, "checked"), html)
	}
	proIdx := strings.Index(html, `value="pro"`)
	if !strings.Contains(html[proIdx:proIdx+30], "checked") {
		t.Fatalf("selected option 'pro' not marked checked: %q", html)
	}
}

func TestRangeRendersBoundsAndValue(t *testing.T) {
	html := string(Range{Name: "volume", Label: "Volume", Min: 0, Max: 10, Step: 0.5, Value: 2.5}.HTML())
	for _, wanted := range []string{`class="poem-range"`, `type="range"`, `min="0"`, `max="10"`, `step="0.5"`, `value="2.5"`, `for="volume"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
	// Step is omitted when not positive.
	if strings.Contains(string(Range{Name: "v", Min: 0, Max: 1, Value: 0}.HTML()), "step=") {
		t.Fatal("Range: step attribute should be absent when Step is zero")
	}
}

func TestControlsRenderDisabledAttribute(t *testing.T) {
	cases := []struct {
		name string
		html string
	}{
		{"Input", string(Input{Name: "email", Label: "Email", Disabled: true}.HTML())},
		{"Textarea", string(Textarea{Name: "notes", Label: "Notes", Disabled: true}.HTML())},
		{"Select", string(Select{Name: "status", Label: "Status", Disabled: true}.HTML())},
		{"Checkbox", string(Checkbox{Name: "notify", Label: "Notify", Disabled: true}.HTML())},
		{"Switch", string(Switch{Name: "wifi", Label: "Wi-Fi", Disabled: true}.HTML())},
	}
	for _, test := range cases {
		if !strings.Contains(test.html, "disabled") {
			t.Fatalf("%s: expected disabled attribute in %q", test.name, test.html)
		}
	}
	if strings.Contains(string(Input{Name: "email", Label: "Email"}.HTML()), "disabled") {
		t.Fatal("Input: disabled attribute should be absent when Disabled is false")
	}
}
