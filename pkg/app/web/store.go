package web

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// sessionStore owns session identity, expiry, and bounded in-process metadata
// while delegating application-state persistence to a StateStore.
type sessionStore[S any] struct {
	mu      sync.Mutex
	signer  *sessionSigner
	init    S
	backend StateStore[S]
	states  map[string]sessionEntry[S]
	locks   map[string]*sync.Mutex
	ttl     time.Duration
	max     int
}

type sessionEntry[S any] struct {
	state    S
	csrf     string
	lastSeen time.Time
}

func newSessionStore[S any](init S, signer *sessionSigner, ttl time.Duration, max int, backend StateStore[S]) *sessionStore[S] {
	return &sessionStore[S]{signer: signer, init: init, backend: backend, states: make(map[string]sessionEntry[S]), locks: make(map[string]*sync.Mutex), ttl: ttl, max: max}
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

func (s *sessionStore[S]) load(id string, r *http.Request) (state S, csrf string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.states {
		if key != id && s.ttl > 0 && now.Sub(entry.lastSeen) > s.ttl {
			delete(s.states, key)
			if lock := s.locks[key]; lock != nil && lock.TryLock() {
				lock.Unlock()
				delete(s.locks, key)
			}
		}
	}
	if existing, ok := s.states[id]; ok {
		existing.lastSeen = now
		s.states[id] = existing
		return existing.state, existing.csrf, nil
	}
	if s.max > 0 && len(s.states) >= s.max {
		var oldest string
		var oldestAt time.Time
		for key, entry := range s.states {
			if oldest == "" || entry.lastSeen.Before(oldestAt) {
				oldest, oldestAt = key, entry.lastSeen
			}
		}
		delete(s.states, oldest)
		if lock := s.locks[oldest]; lock != nil && lock.TryLock() {
			lock.Unlock()
			delete(s.locks, oldest)
		}
	}
	csrf = csrfForSession(s.signer, id)
	state, found, err := s.backend.Load(r.Context(), id)
	if err != nil {
		return s.init, csrf, err
	}
	if !found {
		state = s.init
		if err := s.backend.Save(r.Context(), id, state); err != nil {
			return s.init, csrf, err
		}
	}
	s.states[id] = sessionEntry[S]{state: state, csrf: csrf, lastSeen: now}
	return state, csrf, nil
}

func (s *sessionStore[S]) save(ctx context.Context, id string, state S) error {
	if err := s.backend.Save(ctx, id, state); err != nil {
		return err
	}
	s.mu.Lock()
	entry := s.states[id]
	entry.state = state
	entry.lastSeen = time.Now()
	s.states[id] = entry
	s.mu.Unlock()
	return nil
}

// Session is the per-request handle to one visitor's state, attached to the
// request context by the session middleware.
type Session[S any] struct {
	ID    string
	State S
	csrf  string
	ctx   context.Context
	store *sessionStore[S]
}

// CSRFToken returns the per-session token that must accompany a state-changing form.
func (s *Session[S]) CSRFToken() string { return s.csrf }

// Set replaces the session's current state and persists it back into the
// store.
func (s *Session[S]) Set(state S) error {
	s.State = state
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return s.store.save(ctx, s.ID, state)
}

type sessionContextKey struct{}

// sessionMiddleware loads (or creates) the caller's session before the next
// handler runs, following the same func(http.Handler) http.Handler shape and
// unexported-context-key pattern as github.com/mulavdm/poem/pkg/web/middleware.RequestID.
func sessionMiddleware[S any](store *sessionStore[S], options Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := readOrIssueSessionID(w, r, store.signer, options.SecureCookies, int(options.SessionTTL/time.Second))
			lock := store.lockFor(id)
			lock.Lock()
			defer lock.Unlock()
			state, csrf, err := store.load(id, r)
			if err != nil {
				http.Error(w, "load session", http.StatusInternalServerError)
				return
			}
			session := &Session[S]{ID: id, State: state, csrf: csrf, ctx: r.Context(), store: store}
			ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sessionFromContext[S any](ctx context.Context) *Session[S] {
	session, _ := ctx.Value(sessionContextKey{}).(*Session[S])
	return session
}
