package feedback

import (
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/web/components"
)

func TestBadgeCarriesVariantClass(t *testing.T) {
	html := string(Badge{Text: "Live", Variant: components.VariantSuccess}.HTML())
	for _, wanted := range []string{"poem-badge", "success", "Live"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
}

func TestProgressRendersDeterminateValueAndMax(t *testing.T) {
	html := string(Progress{Label: "Upload", Value: 40, Max: 100}.HTML())
	for _, wanted := range []string{`class="poem-progress"`, `max="100"`, `value="40"`, `aria-label="Upload"`} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
}

func TestProgressIndeterminateOmitsValue(t *testing.T) {
	html := string(Progress{Label: "Working", Indeterminate: true}.HTML())
	if strings.Contains(html, "value=") {
		t.Fatalf("indeterminate progress must omit value attribute: %q", html)
	}
	if !strings.Contains(html, `max="100"`) {
		t.Fatalf("expected default max=100: %q", html)
	}
}
