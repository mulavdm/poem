package action

import (
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/web/components"
)

func TestButtonEscapesAndValidatesStyling(t *testing.T) {
	html := string(Button{Text: `<script>alert(1)</script>`, Variant: components.Variant(`bad" onclick="x`), Size: components.Size(`bad`)}.HTML())
	for _, unwanted := range []string{"<script>", "onclick"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("unsafe output %q", html)
		}
	}
	for _, wanted := range []string{"poem-button--neutral", "poem-button--medium"} {
		if !strings.Contains(html, wanted) {
			t.Fatalf("missing %q in %q", wanted, html)
		}
	}
}
