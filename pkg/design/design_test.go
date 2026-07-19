package design

import (
	"image/color"
	"sync"
	"testing"
)

func TestClassifyWidth(t *testing.T) {
	tests := []struct {
		width int
		want  WindowClass
	}{{599, WindowCompact}, {600, WindowMedium}, {839, WindowMedium}, {840, WindowExpanded}, {1199, WindowExpanded}, {1200, WindowUltraWide}}
	for _, test := range tests {
		if got := ClassifyWidth(test.width); got != test.want {
			t.Errorf("ClassifyWidth(%d) = %v, want %v", test.width, got, test.want)
		}
	}
}

func TestCustomSystemInitializesCacheConcurrently(t *testing.T) {
	custom := *DefaultSystem()
	custom.cache = nil
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			_ = custom.Resolve(Environment{Platform: PlatformWeb, Width: 800})
		}()
	}
	group.Wait()
}

func TestResolveProfilesAndFallback(t *testing.T) {
	system := DefaultSystem()
	windows := system.Resolve(Environment{Platform: PlatformWindows, Width: 1000, Input: InputCapabilities{Pointer: PointerFine, Keyboard: true}})
	if windows.Mode != RenderingModePlatformNative || windows.Theme.Typography.BodyFamily != "Segoe UI Variable" {
		t.Fatalf("unexpected Windows resolution: %#v", windows)
	}
	android := system.Resolve(Environment{Platform: PlatformAndroid, Width: 400, Input: InputCapabilities{Pointer: PointerCoarse, Touch: true}})
	if android.Theme.Typography.BodyFamily != "Roboto" || android.Theme.Controls.Medium < 48 {
		t.Fatalf("unexpected Android resolution: %#v", android)
	}
	fallback := system.Resolve(Environment{Platform: PlatformUnknown, Width: 400})
	if fallback.Mode != RenderingModeAdaptiveHouse {
		t.Fatalf("unknown platform resolved to %v, want house style", fallback.Mode)
	}
}

func TestResolveAccentAndAccessibility(t *testing.T) {
	system := DefaultSystem()
	accent := color.RGBA{245, 220, 40, 255}
	resolved := system.Resolve(Environment{Platform: PlatformWeb, Width: 900, SystemAccent: &accent, ReducedMotion: true})
	if resolved.Theme.Colors.Accent != accent {
		t.Fatalf("accent = %v, want %v", resolved.Theme.Colors.Accent, accent)
	}
	if !resolved.Theme.Motion.Reduced {
		t.Fatal("reduced motion was not preserved")
	}
	if err := resolved.Theme.Validate(); err != nil {
		t.Fatalf("resolved theme is invalid: %v", err)
	}
}

func TestFinePointerSelectsCompactDensity(t *testing.T) {
	resolved := DefaultSystem().Resolve(Environment{Platform: PlatformWindows, Width: 500, Input: InputCapabilities{Pointer: PointerFine}})
	if resolved.Environment.Density != DensityCompact {
		t.Fatalf("density = %v, want compact", resolved.Environment.Density)
	}
}

func TestProductOverridesAreImmutableAndContrastSafe(t *testing.T) {
	system := DefaultSystem()
	accent := color.RGBA{39, 103, 236, 255}
	canvas := color.RGBA{246, 248, 252, 255}
	branded := system.WithOverrides(Overrides{Light: ColorOverrides{Accent: &accent, Canvas: &canvas}})
	resolved := branded.Resolve(Environment{Platform: PlatformWeb, Width: 900})
	if resolved.Theme.Colors.Accent != accent || resolved.Theme.Colors.Surface != canvas {
		t.Fatalf("overrides not resolved: %+v", resolved.Theme.Colors)
	}
	if system.Resolve(Environment{Platform: PlatformWeb, Width: 900}).Theme.Colors.Accent == accent {
		t.Fatal("WithOverrides mutated receiver")
	}
	if err := resolved.Theme.Validate(); err != nil {
		t.Fatalf("invalid branded theme: %v", err)
	}
}

func BenchmarkResolveCached(b *testing.B) {
	system := DefaultSystem()
	environment := Environment{Platform: PlatformWindows, Width: 1024, Height: 768, Input: InputCapabilities{Pointer: PointerFine, Keyboard: true}}
	system.Resolve(environment)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		system.Resolve(environment)
	}
}
