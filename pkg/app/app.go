package app

import "github.com/mulavdm/poem/pkg/design"

// App defines a Trellis application: an initial state, a pure function
// computing the current view from state, and a pure reducer applying a fired
// Msg to produce the next state plus optional asynchronous work. This is the
// one piece of API an application author writes against; poem.Run and web.Run
// each drive it as a real running interface.
type App[S any] struct {
	Init S
	// Start optionally derives the initial visible state and one asynchronous
	// command. Drivers invoke it exactly once for a native engine lifetime or
	// newly-created web session, never for repaints or restored sessions.
	Start func(state S) (S, Cmd)
	// Design selects the token system shared by every renderer. Nil uses the
	// built-in platform-native profiles with Adaptive House fallback.
	Design *design.System
	// Commands derives the currently available semantic actions from state.
	// Nodes refer to these by ID; invocation still travels as a transport-safe
	// Msg through Update.
	Commands func(state S) []Command
	View     func(state S) Node
	Update   func(state S, msg Msg) (S, Cmd)
}

// ResolvedView lowers the semantic tree with the current command registry.
// Drivers call it while holding the same state snapshot used by View.
func ResolvedView[S any](application App[S], state S) Node {
	var commands []Command
	if application.Commands != nil {
		commands = application.Commands(state)
	}
	return LowerSemantic(application.View(state), commands)
}
