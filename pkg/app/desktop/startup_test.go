package desktop

import (
	"sync/atomic"
	"testing"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/render"
)

func TestConfigureRunsStartOnce(t *testing.T) {
	var calls atomic.Int32
	a := app.App[int]{Init: 1, Start: func(state int) (int, app.Cmd) { calls.Add(1); return state + 1, app.Cmd{} }, View: func(state int) app.Node { return app.Text("ready") }, Update: func(state int, _ app.Msg) (int, app.Cmd) { return state, app.Cmd{} }}
	configured := Configure(a, render.AppConfig{})
	if calls.Load() != 1 {
		t.Fatalf("Start calls=%d", calls.Load())
	}
	state := &render.ApplicationState{}
	configured.BuildPagesFn(state)
	configured.BuildPagesFn(state)
	if calls.Load() != 1 {
		t.Fatalf("Start reran during rebuild: %d", calls.Load())
	}
	configured.OnStop()
}
