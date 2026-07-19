package app

import (
	"context"
	"testing"
)

type testPreferences struct{}

func (testPreferences) Get(context.Context, string) ([]byte, error) { return []byte("ok"), nil }
func (testPreferences) Put(context.Context, string, []byte) error   { return nil }
func (testPreferences) Delete(context.Context, string) error        { return nil }

func TestPlatformServicesStayInContext(t *testing.T) {
	ctx := WithPlatformServices(context.Background(), PlatformServices{Preferences: testPreferences{}})
	services := ServicesFromContext(ctx)
	if services.Preferences == nil {
		t.Fatal("preferences service missing")
	}
	body, err := services.Preferences.Get(ctx, "quality")
	if err != nil || string(body) != "ok" {
		t.Fatalf("body=%q err=%v", body, err)
	}
	if ServicesFromContext(context.Background()).Preferences != nil {
		t.Fatal("service escaped its context")
	}
}
