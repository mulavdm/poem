// Package command runs application commands while serializing reducer and
// view access for the target drivers.
package command

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/mulavdm/poem/pkg/app"
)

var errCommandPanic = errors.New("asynchronous command failed")
var errSubscriptionPanic = errors.New("asynchronous subscription failed")

type entry struct {
	generation uint64
	cancel     context.CancelFunc
}

type subscriptionEntry struct {
	generation uint64
	revision   string
	cancel     context.CancelFunc
}

// Executor owns keyed command lifetimes. apply is always called under the
// same lock used by Read, so reducers, completion delivery, and views cannot
// observe or mutate application state concurrently.
type Executor struct {
	mu                  sync.Mutex
	ctx                 context.Context
	cancel              context.CancelFunc
	apply               func(app.Msg) (app.Cmd, error)
	onCompletion        func()
	logger              *slog.Logger
	commands            map[string]entry
	subscriptions       map[string]subscriptionEntry
	deriveSubscriptions func() []app.Subscription
	next                uint64
	closed              bool
}

// New constructs an executor rooted in ctx.
func New(ctx context.Context, apply func(app.Msg) (app.Cmd, error), onCompletion func(), logger *slog.Logger) *Executor {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	root, cancel := context.WithCancel(ctx)
	return &Executor{ctx: root, cancel: cancel, apply: apply, onCompletion: onCompletion, logger: logger, commands: make(map[string]entry), subscriptions: make(map[string]subscriptionEntry)}
}

// SetSubscriptions installs the state-derived subscription registry and
// starts its initial generation. derive is called only while application
// state is serialized under the executor lock.
func (e *Executor) SetSubscriptions(derive func() []app.Subscription) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return context.Canceled
	}
	e.deriveSubscriptions = derive
	e.reconcileSubscriptionsLocked()
	return nil
}

// Dispatch serializes a user or platform message through the reducer.
func (e *Executor) Dispatch(msg app.Msg) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return context.Canceled
	}
	cmd, err := e.apply(msg)
	if err != nil {
		return err
	}
	e.startLocked(cmd)
	e.reconcileSubscriptionsLocked()
	return nil
}

// Start begins work that was produced outside the reducer, such as an
// application's once-per-lifetime startup command. It observes the same keyed
// cancellation and completion serialization rules as reducer-produced work.
func (e *Executor) Start(cmd app.Cmd) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return context.Canceled
	}
	e.startLocked(cmd)
	return nil
}

// Read runs fn while reducer/completion dispatch is excluded.
func (e *Executor) Read(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn()
}

// Inspect runs fn while state access is serialized and reports the pending
// status from the same instant.
func (e *Executor) Inspect(fn func(pending bool)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(len(e.commands) != 0)
}

// Pending reports whether any command is currently active.
func (e *Executor) Pending() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.commands) != 0
}

// Close cancels every active command and rejects future dispatches.
func (e *Executor) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	e.cancel()
	for _, active := range e.commands {
		active.cancel()
	}
	for _, active := range e.subscriptions {
		active.cancel()
	}
	e.commands = make(map[string]entry)
	e.subscriptions = make(map[string]subscriptionEntry)
	e.mu.Unlock()
}

func (e *Executor) startLocked(cmd app.Cmd) {
	if cmd.Run == nil {
		return
	}
	if cmd.Name == "" {
		panic("app.Cmd: Name must not be empty when Run is set")
	}
	if previous, ok := e.commands[cmd.Name]; ok {
		previous.cancel()
	}
	e.next++
	generation := e.next
	ctx, cancel := context.WithCancel(e.ctx)
	e.commands[cmd.Name] = entry{generation: generation, cancel: cancel}
	go e.execute(ctx, cmd, generation)
}

func (e *Executor) execute(ctx context.Context, cmd app.Cmd, generation uint64) {
	var value any
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = errCommandPanic
				e.logger.Error("application command panicked", "command", cmd.Name)
			}
		}()
		value, err = cmd.Run(ctx)
	}()

	e.mu.Lock()
	active, ok := e.commands[cmd.Name]
	if e.closed || !ok || active.generation != generation {
		e.mu.Unlock()
		return
	}
	delete(e.commands, cmd.Name)
	active.cancel()
	next, applyErr := e.apply(app.Msg{Name: cmd.Name, Value: value, Err: err})
	if applyErr == nil {
		e.startLocked(next)
		e.reconcileSubscriptionsLocked()
	}
	e.mu.Unlock()

	if applyErr != nil {
		e.logger.Error("persist application command result", "command", cmd.Name, "error", applyErr)
		return
	}
	if e.onCompletion != nil {
		e.onCompletion()
	}
}

func (e *Executor) reconcileSubscriptionsLocked() {
	if e.deriveSubscriptions == nil || e.closed {
		return
	}
	wanted := make(map[string]app.Subscription)
	for _, subscription := range e.deriveSubscriptions() {
		if !subscription.Valid() {
			continue
		}
		if _, duplicate := wanted[subscription.Name]; duplicate {
			panic("app.Subscription: duplicate Name " + subscription.Name)
		}
		wanted[subscription.Name] = subscription
	}
	for name, active := range e.subscriptions {
		subscription, keep := wanted[name]
		if keep && subscription.Revision == active.revision {
			delete(wanted, name)
			continue
		}
		active.cancel()
		delete(e.subscriptions, name)
	}
	for name, subscription := range wanted {
		e.next++
		generation := e.next
		ctx, cancel := context.WithCancel(e.ctx)
		e.subscriptions[name] = subscriptionEntry{generation: generation, revision: subscription.Revision, cancel: cancel}
		go e.executeSubscription(ctx, subscription, generation)
	}
}

func (e *Executor) executeSubscription(ctx context.Context, subscription app.Subscription, generation uint64) {
	emit := func(msg app.Msg) {
		e.mu.Lock()
		active, ok := e.subscriptions[subscription.Name]
		if e.closed || !ok || active.generation != generation || ctx.Err() != nil {
			e.mu.Unlock()
			return
		}
		next, err := e.apply(msg)
		if err == nil {
			e.startLocked(next)
			e.reconcileSubscriptionsLocked()
		}
		e.mu.Unlock()
		if err != nil {
			e.logger.Error("persist application subscription event", "subscription", subscription.Name, "error", err)
		}
	}
	var runErr error
	func() {
		defer func() {
			if recover() != nil {
				runErr = errSubscriptionPanic
				e.logger.Error("application subscription panicked", "subscription", subscription.Name)
			}
		}()
		runErr = subscription.Run(ctx, emit)
	}()

	e.mu.Lock()
	active, ok := e.subscriptions[subscription.Name]
	if e.closed || !ok || active.generation != generation {
		e.mu.Unlock()
		return
	}
	delete(e.subscriptions, subscription.Name)
	active.cancel()
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		next, err := e.apply(app.Msg{Name: subscription.Name, Err: runErr})
		if err == nil {
			e.startLocked(next)
			e.reconcileSubscriptionsLocked()
		}
	}
	e.mu.Unlock()
}
