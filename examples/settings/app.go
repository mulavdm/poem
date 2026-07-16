// Package settings is the shared Settings application: the same
// Checkbox/Select pairing as the Preferences example, but wrapped in a Modal
// this time — the Node kind added to stress-test whether the architecture
// holds up for something whose open/closed state lives outside App[S].
package settings

import "github.com/mulavdm/poem/pkg/app"

// State is the Settings app's entire application state.
type State struct {
	Subscribed bool
	Theme      string
}

var themeOptions = []app.Option{
	{Label: "Light", Value: "light"},
	{Label: "Dark", Value: "dark"},
	{Label: "System", Value: "system"},
}

// App is the shared Settings definition. Both example mains import this and
// hand it to their respective backend's Run.
var App = app.App[State]{
	Init: State{Subscribed: false, Theme: "system"},
	View: view,
	Update: func(state State, msg app.Msg) State {
		switch msg.Name {
		case "subscribed":
			state.Subscribed = msg.Bool()
		case "theme":
			state.Theme = msg.Payload
		}
		return state
	},
}

func view(state State) app.Node {
	return app.Container(app.Vertical, 12,
		app.Text("Subscribed: "+boolText(state.Subscribed)+" / Theme: "+state.Theme),
		app.Modal("Open Settings", "Settings",
			app.Checkbox("Subscribe to updates", state.Subscribed, app.Msg{Name: "subscribed"}),
			app.Text("Theme"),
			app.Select(themeOptions, state.Theme, app.Msg{Name: "theme"}),
			app.Button("Save", app.Msg{Name: "save"}),
		),
	)
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
