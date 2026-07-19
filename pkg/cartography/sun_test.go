package cartography

import (
	"math"
	"testing"
	"time"
)

// Amsterdam.
const testLat, testLon = 52.37, 4.90

func TestSolarPositionSummerNoonIsHighAndSouth(t *testing.T) {
	// Local solar noon at 4.9°E is ~11:40 UTC; near the summer solstice the sun
	// is highest and due south.
	when := time.Date(2026, 6, 21, 11, 40, 0, 0, time.UTC)
	azimuth, elevation := SolarPosition(testLat, testLon, when)
	// Max elevation ≈ 90 - (lat - 23.44) ≈ 61°.
	if elevation < 57 || elevation > 63 {
		t.Fatalf("summer-noon elevation = %.2f, want ~61", elevation)
	}
	if azimuth < 165 || azimuth > 195 {
		t.Fatalf("summer-noon azimuth = %.2f, want ~180 (south)", azimuth)
	}
}

func TestSolarPositionNightBelowHorizon(t *testing.T) {
	when := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC) // ~01:00 local, deep night
	_, elevation := SolarPosition(testLat, testLon, when)
	if elevation > 0 {
		t.Fatalf("midnight elevation = %.2f, want below horizon", elevation)
	}
}

func TestSolarPositionMorningIsEasterly(t *testing.T) {
	when := time.Date(2026, 6, 21, 5, 0, 0, 0, time.UTC) // ~07:00 local
	azimuth, elevation := SolarPosition(testLat, testLon, when)
	if elevation <= 0 {
		t.Fatalf("morning elevation = %.2f, want above horizon", elevation)
	}
	// Morning sun sits in the eastern half (azimuth < 180).
	if azimuth <= 30 || azimuth >= 150 {
		t.Fatalf("morning azimuth = %.2f, want easterly", azimuth)
	}
}

func TestSolarPositionWinterLowerThanSummer(t *testing.T) {
	summer := time.Date(2026, 6, 21, 11, 40, 0, 0, time.UTC)
	winter := time.Date(2026, 12, 21, 11, 40, 0, 0, time.UTC)
	_, summerEl := SolarPosition(testLat, testLon, summer)
	_, winterEl := SolarPosition(testLat, testLon, winter)
	if winterEl >= summerEl {
		t.Fatalf("winter noon (%.2f) should be lower than summer noon (%.2f)", winterEl, summerEl)
	}
	// Amsterdam winter-solstice noon sun is only ~14° up.
	if winterEl < 10 || winterEl > 18 {
		t.Fatalf("winter-noon elevation = %.2f, want ~14", winterEl)
	}
}

func TestClampVisualElevation(t *testing.T) {
	if got := clampVisualElevation(-5); got != nightAmbientElevation {
		t.Fatalf("night clamp = %.1f", got)
	}
	if got := clampVisualElevation(3); got != minVisualElevation {
		t.Fatalf("low-sun clamp = %.1f", got)
	}
	if got := clampVisualElevation(40); got != 40 {
		t.Fatalf("normal elevation changed = %.1f", got)
	}
}

func TestLightingResolveDefaultsWhenNotDynamic(t *testing.T) {
	azimuth, elevation := Lighting{DynamicSun: false}.resolve(testLat, testLon)
	if azimuth != defaultSunAzimuth || elevation != defaultSunElevation {
		t.Fatalf("static lighting = %.1f/%.1f", azimuth, elevation)
	}
	// Dynamic but zero clock also falls back rather than lighting from the epoch.
	azimuth, elevation = Lighting{DynamicSun: true}.resolve(testLat, testLon)
	if azimuth != defaultSunAzimuth || elevation != defaultSunElevation {
		t.Fatalf("zero-clock lighting = %.1f/%.1f", azimuth, elevation)
	}
}

func TestLightingResolveDynamicMatchesSolarPosition(t *testing.T) {
	when := time.Date(2026, 6, 21, 11, 40, 0, 0, time.UTC)
	azimuth, elevation := Lighting{DynamicSun: true, Clock: when}.resolve(testLat, testLon)
	sunAz, sunEl := SolarPosition(testLat, testLon, when)
	if math.Abs(azimuth-sunAz) > 1e-9 || math.Abs(elevation-clampVisualElevation(sunEl)) > 1e-9 {
		t.Fatalf("dynamic lighting %.3f/%.3f != solar %.3f/%.3f", azimuth, elevation, sunAz, clampVisualElevation(sunEl))
	}
}
