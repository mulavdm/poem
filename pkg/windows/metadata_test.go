package windows

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestMetadataContract(t *testing.T) {
	want := Metadata{Identity: "com.mapps.client", Title: "MAPPS & POEM", Width: 900, Height: 700}
	encoded, err := encodeMetadata(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Metadata
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("metadata = %#v, want %#v", got, want)
	}
	if string(encoded) == "" || bytes.Contains(encoded, []byte(`\u0026`)) {
		t.Fatalf("metadata unnecessarily escaped title: %s", encoded)
	}
}

func TestMetadataRejectsUnsafeValues(t *testing.T) {
	for _, metadata := range []Metadata{
		{Identity: `..\evil`, Title: "x", Width: 640, Height: 480},
		{Identity: "com.example.app", Width: 640, Height: 480},
		{Identity: "com.example.app", Title: "x", Width: 0, Height: 480},
		{Identity: "com.example.app", Title: "x", Width: 640, Height: 20000},
	} {
		if err := metadata.Validate(); err == nil {
			t.Fatalf("accepted %#v", metadata)
		}
	}
}
