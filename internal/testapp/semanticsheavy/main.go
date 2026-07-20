//go:build windows

// Command semanticsheavy is a test fixture for the Windows host startup smoke
// test. It renders a deliberately large, deeply semantic tree so the engine
// publishes a big SemanticTree while the host is still completing its startup
// handshake (WM_SIZE -> SendEvent -> WriteMessage). That is the window in which
// a synchronous semantics publish deadlocks the UI thread; the bundled examples
// are too small to lose that race reliably, so they cannot guard it.
package main

import (
	"fmt"

	"github.com/mulavdm/poem/pkg/app"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
	"github.com/mulavdm/poem/pkg/render"
	poemwindows "github.com/mulavdm/poem/pkg/windows"
)

// nodeCount is large enough to make the semantic tree substantial without
// making the fixture slow to render.
const nodeCount = 600

type state struct{ Ticks int }

func view(s state) app.Node {
	children := make([]app.Node, 0, nodeCount)
	for index := 0; index < nodeCount; index++ {
		id := fmt.Sprintf("row-%d", index)
		children = append(children, app.Container(app.Horizontal, int(app.SpaceRelated),
			app.LabelNode{Semantic: app.Semantic{ID: id + "-label", Name: "Row " + id + " label", Enabled: true},
				Text: fmt.Sprintf("Semantic row %d of %d", index, nodeCount)},
			app.ActionNode{Semantic: app.Semantic{ID: id + "-action", Name: "Activate row " + id, Enabled: true},
				Label: fmt.Sprintf("Action %d", index), Invoke: app.Msg{Name: "tick"}},
		))
	}
	return app.WorkspaceNode{
		Semantic: app.Semantic{ID: "workspace", Name: "Semantics heavy fixture", Enabled: true},
		Title:    "Semantics Heavy",
		Content:  children,
	}
}

func update(s state, m app.Msg) (state, app.Cmd) {
	if m.Name == "tick" {
		s.Ticks++
	}
	return s, app.Cmd{}
}

func init() {
	application := app.App[state]{Init: state{}, View: view, Update: update}
	config := desktop.Configure(application, render.AppConfig{Title: "Semantics Heavy", Width: 900, Height: 700})
	poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "POEM.SemanticsHeavy", Title: config.Title, Width: config.Width, Height: config.Height})
}

func main() {}
