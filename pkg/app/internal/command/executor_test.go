package command

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mulavdm/poem/pkg/app"
)

func waitValue[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for command completion")
		var zero T
		return zero
	}
}

func TestSameNameSupersedesAndDiscardsLateResult(t *testing.T) {
	firstRelease := make(chan struct{})
	secondRelease := make(chan struct{})
	values := make(chan any, 2)
	var starts int
	executor := New(context.Background(), func(msg app.Msg) (app.Cmd, error) {
		if msg.Name == "start" {
			starts++
			release, value := firstRelease, "old"
			if starts == 2 {
				release, value = secondRelease, "new"
			}
			return app.Cmd{Name: "work.done", Run: func(context.Context) (any, error) {
				<-release // Deliberately ignore cancellation to test generation rejection.
				return value, nil
			}}, nil
		}
		if msg.Name == "work.done" {
			values <- msg.Value
		}
		return app.Cmd{}, nil
	}, nil, nil)
	defer executor.Close()

	if err := executor.Dispatch(app.Msg{Name: "start"}); err != nil {
		t.Fatal(err)
	}
	if err := executor.Dispatch(app.Msg{Name: "start"}); err != nil {
		t.Fatal(err)
	}
	close(secondRelease)
	if got := waitValue(t, values); got != "new" {
		t.Fatalf("completion = %v", got)
	}
	close(firstRelease)
	select {
	case got := <-values:
		t.Fatalf("superseded completion delivered: %v", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDifferentNamesRunConcurrentlyAndChain(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	completed := make(chan any, 3)
	executor := New(context.Background(), func(msg app.Msg) (app.Cmd, error) {
		switch msg.Name {
		case "start-a", "start-b":
			name := msg.Name + ".done"
			return app.Cmd{Name: name, Run: func(context.Context) (any, error) {
				started <- name
				<-release
				return name, nil
			}}, nil
		case "start-a.done":
			completed <- msg.Value
			return app.Cmd{Name: "chain.done", Run: func(context.Context) (any, error) { return "chain", nil }}, nil
		case "start-b.done", "chain.done":
			completed <- msg.Value
		}
		return app.Cmd{}, nil
	}, nil, nil)
	defer executor.Close()

	_ = executor.Dispatch(app.Msg{Name: "start-a"})
	_ = executor.Dispatch(app.Msg{Name: "start-b"})
	seen := map[string]bool{waitValue(t, started): true, waitValue(t, started): true}
	if !seen["start-a.done"] || !seen["start-b.done"] {
		t.Fatalf("commands did not overlap: %v", seen)
	}
	close(release)
	for range 3 {
		waitValue(t, completed)
	}
}

func TestPanicBecomesSanitizedCompletionAndCloseCancels(t *testing.T) {
	completion := make(chan app.Msg, 1)
	cancelled := make(chan struct{})
	var mu sync.Mutex
	mode := "panic"
	executor := New(context.Background(), func(msg app.Msg) (app.Cmd, error) {
		if msg.Name == "start" {
			mu.Lock()
			current := mode
			mu.Unlock()
			if current == "panic" {
				return app.Cmd{Name: "panic.done", Run: func(context.Context) (any, error) { panic("secret") }}, nil
			}
			return app.Cmd{Name: "wait.done", Run: func(ctx context.Context) (any, error) {
				<-ctx.Done()
				close(cancelled)
				return nil, ctx.Err()
			}}, nil
		}
		completion <- msg
		return app.Cmd{}, nil
	}, nil, nil)

	_ = executor.Dispatch(app.Msg{Name: "start"})
	msg := waitValue(t, completion)
	if !errors.Is(msg.Err, errCommandPanic) {
		t.Fatalf("panic error = %v", msg.Err)
	}
	mu.Lock()
	mode = "wait"
	mu.Unlock()
	_ = executor.Dispatch(app.Msg{Name: "start"})
	executor.Close()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("close did not cancel active command")
	}
}

func TestExecutorReconcilesSubscriptionGenerations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var revision atomic.Value
	revision.Store("one")
	var received []string
	started := make(chan string, 2)
	stopped := make(chan string, 2)
	executor := New(ctx, func(msg app.Msg) (app.Cmd, error) {
		mu.Lock()
		received = append(received, msg.Payload)
		mu.Unlock()
		return app.Cmd{}, nil
	}, nil, nil)
	defer executor.Close()
	if err := executor.SetSubscriptions(func() []app.Subscription {
		captured := revision.Load().(string)
		return []app.Subscription{{Name: "location", Revision: captured, Run: func(ctx context.Context, emit func(app.Msg)) error {
			started <- captured
			emit(app.Msg{Name: "location", Payload: captured})
			<-ctx.Done()
			stopped <- captured
			emit(app.Msg{Name: "location", Payload: "late-" + captured})
			return ctx.Err()
		}}}
	}); err != nil {
		t.Fatal(err)
	}
	if got := waitValue(t, started); got != "one" {
		t.Fatalf("started %q", got)
	}
	revision.Store("two")
	if err := executor.Dispatch(app.Msg{Name: "change"}); err != nil {
		t.Fatal(err)
	}
	if got := waitValue(t, stopped); got != "one" {
		t.Fatalf("stopped %q", got)
	}
	if got := waitValue(t, started); got != "two" {
		t.Fatalf("started %q", got)
	}

	contains := func(values []string, wanted string) bool {
		for _, value := range values {
			if value == wanted {
				return true
			}
		}
		return false
	}
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		got := append([]string(nil), received...)
		mu.Unlock()
		if contains(got, "one") && contains(got, "two") {
			if contains(got, "late-one") {
				t.Fatalf("accepted stale emission: %v", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("events = %v", got)
		}
		time.Sleep(time.Millisecond)
	}
}

func BenchmarkDispatchNoCommand(b *testing.B) {
	executor := New(context.Background(), func(app.Msg) (app.Cmd, error) { return app.Cmd{}, nil }, nil, nil)
	defer executor.Close()
	b.ReportAllocs()
	for b.Loop() {
		if err := executor.Dispatch(app.Msg{Name: "noop"}); err != nil {
			b.Fatal(err)
		}
	}
}
