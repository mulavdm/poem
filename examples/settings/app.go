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
	Commands: func(State) []app.Command {
		return []app.Command{{ID: "save", Label: "Save", Invoke: app.Msg{Name: "save"}, Enabled: true, Visible: true}}
	},
	View: view,
	Update: func(state State, msg app.Msg) (State, app.Cmd) {
		switch msg.Name {
		case "subscribed":
			state.Subscribed = msg.Bool()
		case "theme":
			state.Theme = msg.Payload
		}
		return state, app.Cmd{}
	},
}

func view(state State) app.Node {
	return app.WorkspaceNode{Semantic: app.Semantic{ID: "settings-workspace", Name: "Settings", Enabled: true}, Title: "Settings", Subtitle: "Application preferences", Content: []app.Node{
		app.LabelNode{Semantic: app.Semantic{ID: "summary", Name: "Current settings", Enabled: true}, Text: "Subscribed: " + boolText(state.Subscribed) + " / Theme: " + state.Theme},
		app.DialogNode{Semantic: app.Semantic{ID: "settings-dialog", Name: "Settings dialog", Enabled: true}, Trigger: "Open Settings", Title: "Settings", Content: []app.Node{
			app.ToggleFieldNode{Semantic: app.Semantic{ID: "subscribed", Enabled: true}, Label: "Subscribe to updates", Value: state.Subscribed, OnChange: app.Msg{Name: "subscribed"}},
			app.ChoiceFieldNode{Semantic: app.Semantic{ID: "theme", Enabled: true}, Label: "Theme", Options: themeOptions, Value: state.Theme, OnChange: app.Msg{Name: "theme"}},
			app.ActionNode{Semantic: app.Semantic{ID: "save", Enabled: true}, Command: "save"},
		}},
	}}
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
