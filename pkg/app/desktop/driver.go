package desktop

import (
	"context"
	"image"

	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/app/internal/command"
)

const rootPage = "trellis-root"

// Configure returns config with its BuildPagesFn replaced by the
// application-driven implementation, leaving every other field unchanged.
// Windows DLL entrypoints register the result with pkg/windows; Android
// c-shared entrypoints pass it to pkg/mobile.Start.
func Configure[S any](a app.App[S], config render.AppConfig) render.AppConfig {
	ctx, cancel := context.WithCancel(context.Background())
	configured, executor := configure(ctx, a, config)
	previousStop := configured.OnStop
	configured.OnStop = func() {
		cancel()
		executor.Close()
		if previousStop != nil {
			previousStop()
		}
	}
	return configured
}

func configure[S any](ctx context.Context, a app.App[S], config render.AppConfig) (render.AppConfig, *command.Executor) {
	if config.Design == nil {
		config.Design = a.Design
	}
	state := a.Init
	var startup app.Cmd
	if a.Start != nil {
		state, startup = a.Start(state)
	}
	var executor *command.Executor
	executor = command.New(ctx, func(msg app.Msg) (app.Cmd, error) {
		var cmd app.Cmd
		state, cmd = a.Update(state, msg)
		return cmd, nil
	}, render.RequestRepaint, nil)
	_ = executor.Start(startup)

	config.BuildPagesFn = func(rstate *render.ApplicationState) {
		dispatch := func(msg app.Msg) {
			_ = executor.Dispatch(msg)
		}
		var root render.Component
		executor.Read(func() {
			root = build(app.ResolvedView(a, state), rootPage, dispatch)
		})

		w, h := rstate.GetWindowSize()
		root.SetBounds(image.Rect(0, 0, w, h))

		// The page scrolls when the app's content is taller than the window:
		// a ScrollView wraps the root on every target (wheel on desktop,
		// drag-to-scroll translated to wheel events by the Android presenter).
		// With content that fits it has no scrollbar and scrolls nothing, so
		// short pages behave exactly as before.
		scroll := render.NewScrollView(rootPage+"/scroll", root)
		scroll.SetBounds(image.Rect(0, 0, w, h))

		rstate.CurrentPage = rootPage
		rstate.Pages = map[string][]render.Component{rootPage: {scroll}}
	}
	return config, executor
}
