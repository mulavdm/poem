package web

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/app/internal/command"
)

// sessionStore owns session identity, expiry, application state, and one
// keyed command executor per visitor while delegating persistence to a
// StateStore.
type sessionStore[S any] struct {
	mu      sync.Mutex
	app     app.App[S]
	signer  *sessionSigner
	backend StateStore[S]
	states  map[string]*sessionEntry[S]
	locks   map[string]*sync.Mutex
	ttl     time.Duration
	max     int
	logger  *slog.Logger
}

type sessionEntry[S any] struct {
	state    S
	csrf     string
	lastSeen time.Time
	executor *command.Executor
}

func newSessionStore[S any](application app.App[S], signer *sessionSigner, ttl time.Duration, max int, backend StateStore[S], logger *slog.Logger) *sessionStore[S] {
	return &sessionStore[S]{app: application, signer: signer, backend: backend, states: make(map[string]*sessionEntry[S]), locks: make(map[string]*sync.Mutex), ttl: ttl, max: max, logger: logger}
}

func (s *sessionStore[S]) lockFor(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.locks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		s.locks[id] = lock
	}
	return lock
}

func (s *sessionStore[S]) load(id string, r *http.Request) (*sessionEntry[S], error) {
	now := time.Now()
	var evicted []*sessionEntry[S]

	s.mu.Lock()
	for key, entry := range s.states {
		if key != id && s.ttl > 0 && now.Sub(entry.lastSeen) > s.ttl {
			delete(s.states, key)
			if lock := s.locks[key]; lock != nil && lock.TryLock() {
				lock.Unlock()
				delete(s.locks, key)
			}
			evicted = append(evicted, entry)
		}
	}
	if existing := s.states[id]; existing != nil {
		existing.lastSeen = now
		s.mu.Unlock()
		closeEntries(evicted)
		return existing, nil
	}
	if s.max > 0 && len(s.states) >= s.max {
		var oldest string
		var oldestAt time.Time
		for key, entry := range s.states {
			if oldest == "" || entry.lastSeen.Before(oldestAt) {
				oldest, oldestAt = key, entry.lastSeen
			}
		}
		if oldest != "" {
			evicted = append(evicted, s.states[oldest])
			delete(s.states, oldest)
			if lock := s.locks[oldest]; lock != nil && lock.TryLock() {
				lock.Unlock()
				delete(s.locks, oldest)
			}
		}
	}
	s.mu.Unlock()
	closeEntries(evicted)

	state, found, err := s.backend.Load(r.Context(), id)
	if err != nil {
		return nil, err
	}
	var startup app.Cmd
	if !found {
		state = s.app.Init
		if s.app.Start != nil {
			state, startup = s.app.Start(state)
		}
		if err := s.backend.Save(r.Context(), id, state); err != nil {
			return nil, err
		}
	}

	entry := &sessionEntry[S]{state: state, csrf: csrfForSession(s.signer, id), lastSeen: now}
	entry.executor = command.New(context.Background(), func(msg app.Msg) (app.Cmd, error) {
		next, cmd := s.app.Update(entry.state, msg)
		if err := s.backend.Save(context.Background(), id, next); err != nil {
			return app.Cmd{}, err
		}
		entry.state = next
		s.mu.Lock()
		entry.lastSeen = time.Now()
		s.mu.Unlock()
		return cmd, nil
	}, nil, s.logger)
	if err := entry.executor.Start(startup); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.states[id] = entry
	s.mu.Unlock()
	return entry, nil
}

func closeEntries[S any](entries []*sessionEntry[S]) {
	for _, entry := range entries {
		if entry != nil && entry.executor != nil {
			entry.executor.Close()
		}
	}
}

// Session is the per-request handle to one visitor's state, attached to the
// request context by the session middleware.
type Session[S any] struct {
	ID    string
	csrf  string
	entry *sessionEntry[S]
}

// CSRFToken returns the per-session token that must accompany a state-changing form.
func (s *Session[S]) CSRFToken() string { return s.csrf }

// Snapshot returns the current state and whether commands are pending.
func (s *Session[S]) Snapshot() (state S, pending bool) {
	s.entry.executor.Inspect(func(active bool) {
		state = s.entry.state
		pending = active
	})
	return state, pending
}

// Dispatch applies msg, persists the resulting state, and starts its command.
func (s *Session[S]) Dispatch(msg app.Msg) error {
	return s.entry.executor.Dispatch(msg)
}

type sessionContextKey struct{}

func sessionMiddleware[S any](store *sessionStore[S], options Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := readOrIssueSessionID(w, r, store.signer, options.SecureCookies, int(options.SessionTTL/time.Second))
			lock := store.lockFor(id)
			lock.Lock()
			defer lock.Unlock()
			entry, err := store.load(id, r)
			if err != nil {
				http.Error(w, "load session", http.StatusInternalServerError)
				return
			}
			session := &Session[S]{ID: id, csrf: entry.csrf, entry: entry}
			ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sessionFromContext[S any](ctx context.Context) *Session[S] {
	session, _ := ctx.Value(sessionContextKey{}).(*Session[S])
	return session
}
