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
	View: view,
	Update: func(state State, msg app.Msg) State {
		switch msg.Name {
		case "increment":
			state.Count++
		}
		return state
	},
}

func view(state State) app.Node {
	return app.Container(app.Vertical, 12,
		app.Text("Count: "+strconv.Itoa(state.Count)),
		app.Button("Increment", app.Msg{Name: "increment"}),
	)
}
