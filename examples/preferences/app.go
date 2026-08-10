// Package preferences is the shared Preferences application, exercising the
// interactive form Node kinds — Checkbox, Switch, Select, RadioGroup, Slider,
// and the multi-line TextArea — plus the non-interactive display kinds Badge,
// ProgressBar, and a read-only Table, beyond what the Counter example (a bare
// Button) covers.
package preferences

import (
	"strconv"

	"github.com/mulavdm/poem/pkg/app"
)

// State is the Preferences app's entire application state.
type State struct {
	Subscribed bool
	Compact    bool
	Theme      string
	Plan       string
	Volume     float64
	Notes      string
	Beta       bool
	Tab        string
}

var themeOptions = []app.Option{
	{Label: "Light", Value: "light"},
	{Label: "Dark", Value: "dark"},
	{Label: "System", Value: "system"},
}

var planOptions = []app.Option{
	{Label: "Free", Value: "free"},
	{Label: "Pro", Value: "pro"},
	{Label: "Team", Value: "team"},
}

// App is the shared Preferences definition. Both example mains import this
// and hand it to their respective backend's Run.
var App = app.App[State]{
	Init: State{Subscribed: false, Theme: "system", Plan: "free", Volume: 3},
	Commands: func(State) []app.Command {
		return []app.Command{{ID: "save", Label: "Save preferences", Invoke: app.Msg{Name: "save"}, Enabled: true, Visible: true}}
	},
	View: view,
	Update: func(state State, msg app.Msg) (State, app.Cmd) {
		switch msg.Name {
		case "subscribed":
			state.Subscribed = msg.Bool()
		case "compact":
			state.Compact = msg.Bool()
		case "theme":
			state.Theme = msg.Payload
		case "plan":
			state.Plan = msg.Payload
		case "volume":
			state.Volume = msg.Float()
		case "notes":
			state.Notes = msg.Payload
		case "beta":
			state.Beta = msg.Bool()
		case "tab":
			state.Tab = msg.Payload
		}
		return state, app.Cmd{}
	},
}

func view(state State) app.Node {
	// The web backend's transport is plain HTML forms (see docs/design/architecture/
	// web-backend.md): a checkbox or select alone has no way to submit
	// itself, so a Save button is what actually applies whatever the visitor
	// changed. POEM's own Checkbox/Select fire OnChange immediately on
	// interaction regardless — Save is a web-transport necessity here, not a
	// POEM one, and this view is intentionally identical for both backends.
	return app.FormNode{Semantic: app.Semantic{ID: "preferences-form", Name: "Preferences", Enabled: true}, Children: []app.Node{
		app.Checkbox("Subscribe to updates", state.Subscribed, app.Msg{Name: "subscribed"}),
		app.Switch("Compact layout", state.Compact, app.Msg{Name: "compact"}),
		app.Text("Theme"),
		app.Select(themeOptions, state.Theme, app.Msg{Name: "theme"}),
		app.Text("Plan"),
		app.RadioGroup(planOptions, state.Plan, app.Msg{Name: "plan"}),
		app.Text("Volume"),
		app.Slider(0, 10, state.Volume, app.Msg{Name: "volume"}),
		app.ProgressBar(state.Volume, 10),
		app.Text("Notes"),
		app.TextArea(state.Notes, "Anything else?", app.Msg{Name: "notes"}),
		app.Accordion(
			app.AccordionSection{Title: "Advanced", Content: []app.Node{
				app.Checkbox("Enable beta features", state.Beta, app.Msg{Name: "beta"}),
			}},
			app.AccordionSection{Title: "About", DefaultOpen: true, Content: []app.Node{
				app.Text("Preferences demo exercising every Trellis Node kind."),
			}},
		),
		app.Button("Save", app.Msg{Name: "save"}),
		app.Container(app.Horizontal, int(app.SpaceRelated),
			app.Text("Status:"),
			app.Badge(subscriptionStatus(state.Subscribed), subscriptionVariant(state.Subscribed)),
		),
		app.Tabs([]app.Tab{
			{ID: "profile", Label: "Profile", Content: []app.Node{
				app.Text("Profile tab content"),
				app.Checkbox("Subscribe to updates", state.Subscribed, app.Msg{Name: "subscribed"}),
			}},
			{ID: "audio", Label: "Audio", Content: []app.Node{
				app.Text("Audio tab content"),
				app.ProgressBar(state.Volume, 10),
			}},
		}, state.Tab, app.Msg{Name: "tab"}),
		app.Text("Current settings"),
		app.Table(
			[]app.TableColumn{{Key: "setting", Label: "Setting"}, {Key: "value", Label: "Value"}},
			[]app.TableRow{
				{Cells: map[string]string{"setting": "Plan", "value": state.Plan}},
				{Cells: map[string]string{"setting": "Theme", "value": state.Theme}},
				{Cells: map[string]string{"setting": "Volume", "value": strconv.FormatFloat(state.Volume, 'f', -1, 64)}},
				{Cells: map[string]string{"setting": "Beta", "value": boolText(state.Beta)}},
			},
		),
	}}
}

func subscriptionStatus(subscribed bool) string {
	if subscribed {
		return "Subscribed"
	}
	return "Not subscribed"
}

func subscriptionVariant(subscribed bool) app.Variant {
	if subscribed {
		return app.VariantSuccess
	}
	return app.VariantNeutral
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
