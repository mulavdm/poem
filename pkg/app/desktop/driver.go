package desktop

import (
	"image"

	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/pkg/app"
)

const rootPage = "trellis-root"

// Run drives app as a real running POEM desktop application. config's
// BuildPagesFn field is overwritten with the Trellis-driven implementation;
// every other AppConfig field (Title, Width, Height, Theme, ...) is passed
// through unchanged.
func Run[S any](a app.App[S], config render.AppConfig) {
	render.Run(Configure(a, config))
}

// Configure returns config with its BuildPagesFn replaced by the
// Trellis-driven implementation for app, leaving every other field unchanged.
// Run uses it with POEM's desktop entry; hosts that embed the engine
// in-process (POEM's Android presenter, via poem/pkg/mobile.Start) call it
// directly to obtain the config for their own launch path.
func Configure[S any](a app.App[S], config render.AppConfig) render.AppConfig {
	state := a.Init

	config.BuildPagesFn = func(rstate *render.ApplicationState) {
		dispatch := func(msg app.Msg) {
			state = a.Update(state, msg)
		}
		root := build(a.View(state), rootPage, dispatch)

		w, h := rstate.GetWindowSize()
		root.SetBounds(image.Rect(0, 0, w, h))

		rstate.CurrentPage = rootPage
		rstate.Pages = map[string][]render.Component{rootPage: {root}}
	}
	return config
}
