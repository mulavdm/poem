package app

import "testing"

func TestMapPreferencesStrictRoundTrip(t *testing.T) {
	preferences := DefaultMapPreferences()
	preferences.Locale = "nl-NL"
	preferences.Units = UnitsMetric
	preferences.Camera = MapCamera{Latitude: 52.37, Longitude: 4.9, Zoom: 12, Bearing: 20, Pitch: 35}
	body, err := MarshalMapPreferences(preferences)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseMapPreferences(body)
	if err != nil || decoded != preferences {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	for _, invalid := range [][]byte{
		{},
		[]byte(`{"version":1,"unknown":true}`),
		[]byte(`{"version":2}`),
		[]byte(`{"version":1,"locale":"nl_NL"}`),
		[]byte(`{"version":1,"poi_filters":4294967295}`),
	} {
		if _, err := ParseMapPreferences(invalid); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
}

func TestMapPreferencesRejectsSecretScalePayloads(t *testing.T) {
	preferences := DefaultMapPreferences()
	preferences.CacheBytes = 65 << 30
	if _, err := MarshalMapPreferences(preferences); err == nil {
		t.Fatal("accepted oversized cache budget")
	}
}
