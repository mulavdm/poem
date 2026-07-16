package app

// App defines a Trellis application: an initial state, a pure function
// computing the current view from state, and a pure reducer applying a fired
// Msg to produce the next state. This is the one piece of API an application
// author writes against; poem.Run and web.Run each drive it as a real
// running interface.
type App[S any] struct {
	Init   S
	View   func(state S) Node
	Update func(state S, msg Msg) S
}
