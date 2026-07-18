// Package web drives a Trellis app.App as a real running, stateful web
// application built from GopherWeb's public component renderers.
package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/mulavdm/poem/pkg/web/components"
	"github.com/mulavdm/poem/pkg/web/middleware"

	"github.com/mulavdm/poem/pkg/app"
)

const rootPath = "trellis-root"

const (
	csrfFieldName      = "trellis_csrf"
	defaultSessionTTL  = 24 * time.Hour
	defaultMaxSessions = 10000
	maxEventBody       = 1 << 20
)

// Options controls the web server and session security policy.
type Options struct {
	// SigningKey is the HMAC key for session cookies. Run generates an ephemeral
	// key for local development; production callers should provide a stable key.
	SigningKey     []byte
	SecureCookies  bool
	SessionTTL     time.Duration
	MaxSessions    int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int
	Logger         *slog.Logger
}

func (o Options) withDefaults() Options {
	if o.SessionTTL <= 0 {
		o.SessionTTL = defaultSessionTTL
	}
	if o.MaxSessions <= 0 {
		o.MaxSessions = defaultMaxSessions
	}
	if o.ReadTimeout <= 0 {
		o.ReadTimeout = 10 * time.Second
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = 15 * time.Second
	}
	if o.IdleTimeout <= 0 {
		o.IdleTimeout = 2 * time.Minute
	}
	if o.MaxHeaderBytes <= 0 {
		o.MaxHeaderBytes = 1 << 20
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// Run drives app as a real running web application listening on addr.
// Interactivity travels as plain HTML form submissions — no JavaScript is
// required — matching GopherWeb's own no-JS-baseline philosophy; a browser
// with JavaScript disabled still increments state end to end. State is kept
// server-side per visitor, identified by a signed session cookie (see
// session.go); this is a deliberate departure from GopherWeb's own
// documented stateless-per-request design, scoped to this module only.
func Run[S any](a app.App[S], addr, title string) error {
	handler, options, err := NewHandler(a, title, Options{})
	if err != nil {
		return err
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadTimeout: options.ReadTimeout, WriteTimeout: options.WriteTimeout, IdleTimeout: options.IdleTimeout, MaxHeaderBytes: options.MaxHeaderBytes}
	return server.ListenAndServe()
}

// NewHandler builds a Trellis web handler without starting a listener. The
// default state store is bounded, expiring, and single-process.
func NewHandler[S any](a app.App[S], title string, raw Options) (http.Handler, Options, error) {
	return NewHandlerWithStore(a, title, raw, newMemoryStateStore(a.Init))
}

// NewHandlerWithStore builds a handler using an application-owned durable or
// shared StateStore. The store must provide atomic Save semantics appropriate
// for the deployment; the built-in file store is single-process.
func NewHandlerWithStore[S any](a app.App[S], title string, raw Options, backend StateStore[S]) (http.Handler, Options, error) {
	if backend == nil {
		return nil, Options{}, fmt.Errorf("trellis: state store is nil")
	}
	options := raw.withDefaults()
	signer, err := newConfiguredSigner(options.SigningKey)
	if err != nil {
		return nil, Options{}, err
	}
	store := newSessionStore(a.Init, signer, options.SessionTTL, options.MaxSessions, backend)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", homeHandler(a, title))
	mux.HandleFunc("POST /__event", eventHandler(a))
	mux.Handle("GET /components/static/", http.StripPrefix("/components/static/", components.AssetHandlerWithOptions(components.AssetOptions{})))

	handler := middleware.Chain(mux, sessionMiddleware(store, options), middleware.RequestID(), middleware.SecurityHeadersWithOptions(middleware.SecurityOptions{
		// ImageNode ships images as data: URIs (self-contained, no asset
		// endpoint), so the app driver's CSP must allow them for img-src
		// while everything else stays self-only.
		ContentSecurityPolicy: "default-src 'self'; img-src 'self' data:; base-uri 'self'; frame-ancestors 'none'; form-action 'self'",
	}), middleware.LogRequests(options.Logger), middleware.Recover(options.Logger))
	return handler, options, nil
}

func homeHandler[S any](a app.App[S], title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := sessionFromContext[S](r.Context())
		writePage(w, a, title, session.State, session.CSRFToken())
	}
}

// eventHandler applies a submitted form to the visitor's session state: every
// interactive field's posted value is applied via its OnChange Msg first (so
// a value typed or toggled alongside a button click is not lost), then the
// message named by the submitting button, if any, is applied. Fields read
// differently depending on kind — see fieldKind.
func eventHandler[S any](a app.App[S]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := sessionFromContext[S](r.Context())
		r.Body = http.MaxBytesReader(w, r.Body, maxEventBody)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if !validCSRFToken(session.CSRFToken(), r.PostForm.Get(csrfFieldName)) {
			http.Error(w, "invalid csrf token", http.StatusForbidden)
			return
		}

		state := session.State

		root := a.View(state)
		fields := map[string]postedField{}
		collectFields(root, rootPath, fields)
		fieldNames := make([]string, 0, len(fields))
		for fieldName := range fields {
			fieldNames = append(fieldNames, fieldName)
		}
		sort.Strings(fieldNames)
		for _, fieldName := range fieldNames {
			field := fields[fieldName]
			if field.msg.Name == "" {
				continue
			}
			switch field.kind {
			case valueField:
				if r.PostForm.Has(fieldName) {
					state = a.Update(state, app.Msg{Name: field.msg.Name, Payload: r.PostForm.Get(fieldName)})
				}
			case checkboxField:
				state = a.Update(state, app.Msg{Name: field.msg.Name, Payload: app.BoolPayload(r.PostForm.Has(fieldName))})
			}
		}

		allowed := map[string]bool{}
		collectMessages(root, allowed)
		if msgName := r.PostForm.Get(msgFieldName); msgName != "" {
			if !allowed[msgName] {
				http.Error(w, "invalid event", http.StatusBadRequest)
				return
			}
			state = a.Update(state, app.Msg{Name: msgName})
		}

		if err := session.Set(state); err != nil {
			http.Error(w, "save session", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func writePage[S any](w http.ResponseWriter, a app.App[S], title string, state S, csrf string) {
	body := renderNode(a.View(state), rootPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderShell(w, title, csrf, body); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}
