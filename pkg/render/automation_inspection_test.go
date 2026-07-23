package render

import "testing"

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

// The control-capable surface must stay off unless explicitly asked for.
func TestInspectionOffByDefault(t *testing.T) {
	for _, values := range []map[string]string{
		{},
		{"POEM_INSPECTION": ""},
		{"POEM_INSPECTION": "0"},
		{"POEM_INSPECTION": "true"},
		{"POEM_INSPECTION": "yes"},
		// A port alone must not imply consent to enable it.
		{"POEM_INSPECTION_PORT": "47831"},
	} {
		cfg, err := InspectionAutomationConfig(envFrom(values))
		if err != nil {
			t.Fatalf("%v: unexpected error %v", values, err)
		}
		if cfg != nil {
			t.Fatalf("%v: inspection enabled without POEM_INSPECTION=1", values)
		}
	}
}

func TestInspectionEnabledBindsLoopback(t *testing.T) {
	cfg, err := InspectionAutomationConfig(envFrom(map[string]string{"POEM_INSPECTION": "1"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected an automation config")
	}
	if !cfg.Enabled || cfg.Mode != "http" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("inspection must bind loopback, got %q", cfg.Host)
	}
	if cfg.Port != DefaultInspectionPort {
		t.Fatalf("port = %d, want %d", cfg.Port, DefaultInspectionPort)
	}
}

func TestInspectionPortOverride(t *testing.T) {
	cfg, err := InspectionAutomationConfig(envFrom(map[string]string{
		"POEM_INSPECTION":      "1",
		"POEM_INSPECTION_PORT": "48000",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.Port != 48000 {
		t.Fatalf("port override ignored: %+v", cfg)
	}
}

func TestInspectionRejectsInvalidPort(t *testing.T) {
	for _, raw := range []string{"0", "65536", "-1", "http", "47831.5", "1e5"} {
		cfg, err := InspectionAutomationConfig(envFrom(map[string]string{
			"POEM_INSPECTION":      "1",
			"POEM_INSPECTION_PORT": raw,
		}))
		if err == nil {
			t.Fatalf("port %q: expected an error, got config %+v", raw, cfg)
		}
		if cfg != nil {
			t.Fatalf("port %q: expected no config alongside the error", raw)
		}
	}
}

// Whitespace padding is common when a value comes from a shell or a property.
func TestInspectionTrimsWhitespace(t *testing.T) {
	cfg, err := InspectionAutomationConfig(envFrom(map[string]string{
		"POEM_INSPECTION":      " 1 ",
		"POEM_INSPECTION_PORT": " 48001 ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.Port != 48001 {
		t.Fatalf("whitespace not tolerated: %+v cfg=%v", err, cfg)
	}
}

func TestInspectionNilGetenv(t *testing.T) {
	cfg, err := InspectionAutomationConfig(nil)
	if err != nil || cfg != nil {
		t.Fatalf("nil getenv should disable inspection, got cfg=%+v err=%v", cfg, err)
	}
}
