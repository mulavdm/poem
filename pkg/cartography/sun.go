package cartography

import (
	"math"
	"time"
)

// Default fixed illumination used when dynamic sun is off, chosen so relief and
// building extrusions read clearly: a north-west key light at a comfortable
// elevation. Azimuth is degrees clockwise from north; elevation is degrees
// above the horizon.
const (
	defaultSunAzimuth   = 315.0
	defaultSunElevation = 45.0
	// minVisualElevation keeps the sun off the horizon so shadows stay usable
	// and lighting never degenerates to grazing incidence.
	minVisualElevation = 8.0
	// nightAmbientElevation is the clamped elevation reported when the true sun
	// is below the horizon, so night scenes render with soft overhead ambient
	// instead of a black, unlit surface.
	nightAmbientElevation = 12.0
)

// Lighting controls scene illumination. The clock is injected so lighting is
// deterministic and testable; a zero Clock with DynamicSun set falls back to
// the fixed default rather than an epoch-time sun.
type Lighting struct {
	DynamicSun bool
	Clock      time.Time
}

// resolve returns the azimuth and clamped visual elevation to light a scene at
// the given location. Elevation is clamped to a usable range: a low daytime sun
// is lifted to minVisualElevation, and a below-horizon sun becomes a soft night
// ambient elevation.
func (l Lighting) resolve(latitude, longitude float64) (azimuth, elevation float64) {
	if !l.DynamicSun || l.Clock.IsZero() {
		return defaultSunAzimuth, defaultSunElevation
	}
	azimuth, trueElevation := SolarPosition(latitude, longitude, l.Clock)
	return azimuth, clampVisualElevation(trueElevation)
}

func clampVisualElevation(trueElevation float64) float64 {
	if trueElevation <= 0 {
		return nightAmbientElevation
	}
	if trueElevation < minVisualElevation {
		return minVisualElevation
	}
	if trueElevation > 90 {
		return 90
	}
	return trueElevation
}

// SolarPosition returns the sun's azimuth (degrees clockwise from north) and
// true elevation (degrees above the horizon) for a location and instant. It
// uses the PSA algorithm (Blanco-Muriel et al., 2001), accurate to within about
// half a degree for the modern era — ample for cartographic lighting. A
// negative elevation means the sun is below the horizon.
func SolarPosition(latitude, longitude float64, when time.Time) (azimuth, elevation float64) {
	utc := when.UTC()
	// Decimal hours in UTC.
	decimalHours := float64(utc.Hour()) + float64(utc.Minute())/60 + float64(utc.Second())/3600 + float64(utc.Nanosecond())/3.6e12

	julianDate := julianDay(utc) + decimalHours/24
	elapsed := julianDate - 2451545.0 // days since J2000.0

	const rad = math.Pi / 180
	omega := 2.1429 - 0.0010394594*elapsed
	meanLongitude := 4.8950630 + 0.017202791698*elapsed
	meanAnomaly := 6.2400600 + 0.0172019699*elapsed
	eclipticLongitude := meanLongitude + 0.03341607*math.Sin(meanAnomaly) +
		0.00034894*math.Sin(2*meanAnomaly) - 0.0001134 - 0.0000203*math.Sin(omega)
	eclipticObliquity := 0.4090928 + 6.2140e-9*elapsed + 0.0000396*math.Cos(omega)

	sinEclLon := math.Sin(eclipticLongitude)
	rightAscension := math.Atan2(math.Cos(eclipticObliquity)*sinEclLon, math.Cos(eclipticLongitude))
	if rightAscension < 0 {
		rightAscension += 2 * math.Pi
	}
	declination := math.Asin(math.Sin(eclipticObliquity) * sinEclLon)

	greenwichMeanSiderealTime := 6.6974243242 + 0.0657098283*elapsed + decimalHours
	localMeanSiderealTime := (greenwichMeanSiderealTime*15 + longitude) * rad
	hourAngle := localMeanSiderealTime - rightAscension

	latRad := latitude * rad
	cosHourAngle := math.Cos(hourAngle)
	zenith := math.Acos(math.Cos(latRad)*cosHourAngle*math.Cos(declination) + math.Sin(declination)*math.Sin(latRad))
	azimuthRad := math.Atan2(-math.Sin(hourAngle), math.Tan(declination)*math.Cos(latRad)-math.Sin(latRad)*cosHourAngle)

	// Parallax correction to topocentric elevation.
	const earthMeanRadius = 6371.01
	const astronomicalUnit = 149597890.0
	parallax := (earthMeanRadius / astronomicalUnit) * math.Sin(zenith)
	zenith += parallax

	azimuth = azimuthRad / rad
	if azimuth < 0 {
		azimuth += 360
	}
	elevation = 90 - zenith/rad
	return azimuth, elevation
}

// julianDay returns the Julian Day Number at 00:00 UTC of the given instant's
// calendar date.
func julianDay(utc time.Time) float64 {
	year, month, day := utc.Date()
	y, m := year, int(month)
	if m <= 2 {
		y--
		m += 12
	}
	a := y / 100
	b := 2 - a + a/4
	return math.Floor(365.25*float64(y+4716)) + math.Floor(30.6001*float64(m+1)) + float64(day) + float64(b) - 1524.5
}
