package theme

import "testing"

func TestBuiltInThemesValidate(t *testing.T) {
	for _, candidate := range []Theme{ModernDark(), ModernLight(), Windows(), HighContrast(), Editorial()} {
		if err := candidate.Validate(); err != nil {
			t.Fatalf("%s: %v", candidate.Name, err)
		}
	}
}

func TestDerivedThemeDoesNotMutateSource(t *testing.T) {
	source := ModernDark()
	derived := source.With("compact", func(next *Theme) { next.Density = DensityCompact })
	if source.Density == derived.Density || source.Name == derived.Name {
		t.Fatal("derived theme mutated or failed to differ from source")
	}
}

func TestManagerRejectsInvalidTheme(t *testing.T) {
	mgr := NewManager(ModernDark())
	_, before := mgr.Current()
	if err := mgr.Set(Theme{}); err == nil {
		t.Fatal("expected invalid theme error")
	}
	_, after := mgr.Current()
	if after != before {
		t.Fatal("invalid update changed manager revision")
	}
}

func TestThemeRejectsMissingSemanticForeground(t *testing.T) {
	candidate := ModernDark()
	candidate.Colors.OnDanger.A = 0
	if err := candidate.Validate(); err == nil {
		t.Fatal("theme accepted a missing destructive-control foreground")
	}
}

func TestBuiltInThemeContrast(t *testing.T) {
	for _, candidate := range []Theme{ModernDark(), ModernLight(), Windows(), HighContrast(), Editorial()} {
		if err := candidate.Validate(); err != nil {
			t.Fatalf("%s contrast: %v", candidate.Name, err)
		}
	}
}

func BenchmarkManagerCurrent(b *testing.B) {
	mgr := NewManager(ModernDark())
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.Current()
	}
}
