package app

import "context"

// Subscription is a keyed, long-lived source of runtime-local messages.
// Run must return when ctx is cancelled. emit is concurrency-safe; drivers
// serialize accepted messages through the reducer and discard stale emissions.
type Subscription struct {
	Name string
	// Revision is an application-defined identity for captured inputs. Keeping
	// Name and Revision unchanged preserves the running subscription; changing
	// Revision cancels and replaces it.
	Revision string
	Run      func(ctx context.Context, emit func(Msg)) error
}

func (s Subscription) Valid() bool { return s.Name != "" && s.Run != nil }
