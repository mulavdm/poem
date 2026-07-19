// Package counter is the shared Counter application: one Trellis App
// definition, driven as both a real POEM desktop window and a real
// GopherWeb-style web app by the two example mains in this directory.
package counter

import (
	"strconv"

	"github.com/mulavdm/poem/pkg/app"
)

// State is the Counter's entire application state.
type State struct {
	Count int
}

// App is the shared Counter definition. Both example mains import this and
// hand it to their respective backend's Run.
var App = app.App[State]{
	Init: State{Count: 0},
	Commands: func(State) []app.Command {
		return []app.Command{{ID: "increment", Label: "Increment", Icon: app.IconAdd, Invoke: app.Msg{Name: "increment"}, Enabled: true, Visible: true}}
	},
	View: view,
	Update: func(state State, msg app.Msg) (State, app.Cmd) {
		switch msg.Name {
		case "increment":
			state.Count++
		}
		return state, app.Cmd{}
	},
}

func view(state State) app.Node {
	return app.FormNode{Semantic: app.Semantic{ID: "counter", Name: "Counter", Enabled: true}, Children: []app.Node{
		app.LabelNode{Semantic: app.Semantic{ID: "count", Name: "Current count", Enabled: true}, Text: "Count: " + strconv.Itoa(state.Count)},
		app.ActionNode{Semantic: app.Semantic{ID: "increment", Enabled: true}, Command: "increment"},
	}}
}
