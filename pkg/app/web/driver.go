// Package web drives a Trellis app.App as a real running, stateful web
// application built from GopherWeb's public component renderers.
package web

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/mulavdm/poem/pkg/web/components"
	"github.com/mulavdm/poem/pkg/web/middleware"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/design"
)

const rootPath = "trellis-root"

const (
	csrfFieldName      = "trellis_csrf"
	commitFieldName    = "trellis_field"
	commitValueName    = "trellis_value"
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
	store := newSessionStore(a, signer, options.SessionTTL, options.MaxSessions, backend, options.Logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", homeHandler(a, title))
	mux.HandleFunc("POST /__event", eventHandler(a))
	mux.HandleFunc("POST /__field", fieldCommitHandler(a))
	mux.HandleFunc("GET /__poem/image-viewport.js", imageViewportScriptHandler)
	mux.HandleFunc("GET /__poem/responsive.js", responsiveScriptHandler)
	mux.HandleFunc("GET /__poem/workspace.js", workspaceScriptHandler)
	mux.HandleFunc("GET /__poem/app.css", staticCSSHandler(appCSS))
	designSystem := a.Design
	if designSystem == nil {
		designSystem = design.DefaultSystem()
	}
	mux.HandleFunc("GET /__poem/design.css", staticCSSHandler(designSystem.CSSForWeb()))
	mux.Handle("GET /components/static/", http.StripPrefix("/components/static/", components.AssetHandlerWithOptions(components.AssetOptions{})))

	handler := middleware.Chain(mux, sessionMiddleware(store, options), middleware.RequestID(), middleware.SecurityHeadersWithOptions(middleware.SecurityOptions{
		// ImageNode ships images as data: URIs (self-contained, no asset
		// endpoint), so the app driver's CSP must allow them for img-src
		// while everything else stays self-only.
		ContentSecurityPolicy: "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; base-uri 'self'; frame-ancestors 'none'; form-action 'self'",
	}), middleware.LogRequests(options.Logger), middleware.Recover(options.Logger))
	return handler, options, nil
}

func homeHandler[S any](a app.App[S], title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := sessionFromContext[S](r.Context())
		state, pending := session.Snapshot()
		writePage(w, a, title, state, session.CSRFToken(), pending)
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

		state, _ := session.Snapshot()

		root := app.ResolvedView(a, state)
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
					if err := session.Dispatch(app.Msg{Name: field.msg.Name, Payload: r.PostForm.Get(fieldName)}); err != nil {
						http.Error(w, "save session", http.StatusInternalServerError)
						return
					}
				}
			case checkboxField:
				if err := session.Dispatch(app.Msg{Name: field.msg.Name, Payload: app.BoolPayload(r.PostForm.Has(fieldName))}); err != nil {
					http.Error(w, "save session", http.StatusInternalServerError)
					return
				}
			case imageTransformField, viewportPointField, imageMarkerField:
				if r.PostForm.Has(fieldName) {
					payload := r.PostForm.Get(fieldName)
					if !field.validPayload(payload) {
						http.Error(w, "invalid field value", http.StatusBadRequest)
						return
					}
					if err := session.Dispatch(app.Msg{Name: field.msg.Name, Payload: payload}); err != nil {
						http.Error(w, "save session", http.StatusInternalServerError)
						return
					}
				}
			}
		}

		allowed := map[string]bool{}
		collectMessages(root, allowed)
		if msgName := r.PostForm.Get(msgFieldName); msgName != "" {
			if !allowed[msgName] {
				http.Error(w, "invalid event", http.StatusBadRequest)
				return
			}
			if err := session.Dispatch(app.Msg{Name: msgName}); err != nil {
				http.Error(w, "save session", http.StatusInternalServerError)
				return
			}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// fieldCommitHandler lets progressively enhanced controls commit one current
// field without submitting unrelated form inputs. The field path is resolved
// against the current View, so callers cannot forge a completion name.
func fieldCommitHandler[S any](a app.App[S]) http.HandlerFunc {
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
		state, _ := session.Snapshot()
		fields := map[string]postedField{}
		collectFields(app.ResolvedView(a, state), rootPath, fields)
		field, ok := fields[r.PostForm.Get(commitFieldName)]
		if !ok || (field.kind != imageTransformField && field.kind != viewportPointField && field.kind != imageMarkerField) || field.msg.Name == "" {
			http.Error(w, "invalid field", http.StatusBadRequest)
			return
		}
		payload := r.PostForm.Get(commitValueName)
		if !field.validPayload(payload) {
			http.Error(w, "invalid field value", http.StatusBadRequest)
			return
		}
		if err := session.Dispatch(app.Msg{Name: field.msg.Name, Payload: payload}); err != nil {
			http.Error(w, "save session", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func writePage[S any](w http.ResponseWriter, a app.App[S], title string, state S, csrf string, pending bool) {
	body := renderNode(app.ResolvedView(a, state), rootPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderShell(w, title, csrf, body, pending); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func staticCSSHandler(css string) http.HandlerFunc {
	sum := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(css)))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", sum)
		if r.Header.Get("If-None-Match") == sum {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte(css))
	}
}

const appCSS = `
html,body{height:100%}
body{font-family:var(--poem-font);color:var(--poem-text);background:var(--poem-surface-muted);margin:0;padding:var(--poem-space-lg);box-sizing:border-box}
body>form{min-height:100%}
*,*::before,*::after{box-sizing:border-box}
.trellis-container{display:flex;min-width:0}
.trellis-container--vertical{flex-direction:column}
.trellis-container--horizontal{flex-direction:row;align-items:center;flex-wrap:wrap}
.trellis-gap-0{gap:0}.trellis-gap-1{gap:var(--poem-space-xs)}.trellis-gap-2{gap:var(--poem-space-sm)}.trellis-gap-3{gap:var(--poem-space-md)}.trellis-gap-4{gap:var(--poem-space-lg)}.trellis-gap-5{gap:var(--poem-space-xl)}
.trellis-padding-0{padding:0}.trellis-padding-1{padding:var(--poem-space-xs)}.trellis-padding-2{padding:var(--poem-space-sm)}.trellis-padding-3{padding:var(--poem-space-md)}.trellis-padding-4{padding:var(--poem-space-lg)}.trellis-padding-5{padding:var(--poem-space-xl)}
.trellis-text{margin:0}
.trellis-image-viewport{position:relative;width:100%;aspect-ratio:8/5;overflow:hidden;background:var(--poem-surface);touch-action:none;user-select:none}
.trellis-image-viewport:focus-visible{outline:.125rem solid var(--poem-focus);outline-offset:.125rem}
.trellis-image-viewport img{position:absolute;inset:0;width:100%;height:100%;object-fit:contain;transform-origin:center;will-change:transform;pointer-events:none}
.trellis-image-marker{position:absolute;left:50%;top:50%;z-index:2;width:var(--poem-target);height:var(--poem-target);padding:0;border:var(--poem-space-md) solid transparent;border-radius:50%;background:var(--poem-neutral);background-clip:content-box;transform:translate(-50%,-50%);cursor:pointer}
.trellis-image-marker--primary{background-color:var(--poem-primary)}.trellis-image-marker--secondary{background-color:var(--poem-secondary)}.trellis-image-marker--danger{background-color:var(--poem-danger)}.trellis-image-marker--success{background-color:var(--poem-success)}.trellis-image-marker--warning{background-color:var(--poem-warning)}
.trellis-image-marker.is-selected,.trellis-image-marker:focus-visible{outline:.1875rem solid var(--poem-focus);outline-offset:.0625rem}
.trellis-responsive>fieldset{min-width:0;margin:0;padding:0;border:0}.trellis-responsive>fieldset:not([hidden]){display:flex;flex-direction:column;gap:var(--poem-space-sm)}
.poem-workspace{min-height:calc(100vh - 2 * var(--poem-space-lg));display:grid;grid-template-rows:auto minmax(0,1fr);overflow:hidden;border:1px solid var(--poem-line);border-radius:var(--poem-radius-lg);background:var(--poem-surface);box-shadow:0 var(--poem-space-sm) var(--poem-space-lg) var(--poem-shadow)}
.poem-workspace__header{display:grid;grid-template-columns:minmax(0,1fr) auto auto;align-items:center;gap:var(--poem-space-md);padding:var(--poem-space-md) var(--poem-space-lg);border-bottom:1px solid var(--poem-line);background:var(--poem-surface-raised)}
.poem-workspace__header h1{margin:0;font-size:1.25rem}.poem-workspace__header p{margin:.2rem 0 0;color:var(--poem-muted)}
.poem-workspace__status,.poem-workspace__actions{display:flex;align-items:center;gap:var(--poem-space-sm);min-width:0}.poem-workspace__status>*{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.poem-workspace__body{display:grid;grid-template-columns:minmax(20rem,24rem) minmax(0,1fr);min-height:0}.poem-workspace__content{grid-column:2;grid-row:1;min-width:0;min-height:0;background:var(--poem-surface-muted)}.poem-workspace__tools{grid-column:1;grid-row:1;min-width:0;min-height:0;border-right:1px solid var(--poem-line);background:var(--poem-surface-raised);overflow:hidden}.poem-workspace__tools-scroll{height:100%;overflow:auto;padding:var(--poem-space-lg);display:flex;flex-direction:column;gap:var(--poem-space-md)}.poem-workspace__handle{display:none}
.poem-workspace .trellis-image-viewport{height:100%;max-width:none!important;min-height:22rem;border-radius:0}
.poem-section{background:var(--poem-surface);border:1px solid var(--poem-line);border-radius:var(--poem-radius-md);padding:var(--poem-space-md);display:grid;gap:var(--poem-space-md)}.poem-section h2{font-size:1rem;margin:0}.poem-section__description{color:var(--poem-muted);margin:.25rem 0 0}.poem-section__body{display:flex;flex-direction:column;gap:var(--poem-space-sm)}
.poem-collection__items{display:grid;gap:var(--poem-space-sm)}.poem-collection-item{display:grid;gap:var(--poem-space-sm);padding:var(--poem-space-md);border:1px solid var(--poem-line);border-radius:var(--poem-radius-md);background:var(--poem-surface)}.poem-collection-item.is-selected{border-color:var(--poem-primary);box-shadow:inset .2rem 0 var(--poem-primary)}.poem-collection-item h3,.poem-collection-item p{margin:0}.poem-collection-item p{color:var(--poem-muted)}.poem-collection-item dl{display:flex;flex-wrap:wrap;gap:var(--poem-space-md);margin:.5rem 0 0}.poem-collection-item dl div{display:flex;gap:.35rem}.poem-collection-item dt{color:var(--poem-muted)}.poem-collection-item dd{margin:0;font-weight:700}.poem-collection-item__actions{display:flex;flex-wrap:wrap;gap:var(--poem-space-sm)}
@media(max-width:599px){body{padding:0}.poem-workspace{min-height:100vh;border:0;border-radius:0}.poem-workspace__header{grid-template-columns:auto minmax(0,1fr);padding:var(--poem-space-sm) var(--poem-space-md)}.poem-workspace__header p{display:none}.poem-workspace__status{grid-column:2;justify-self:end;max-width:100%;font-size:.8rem}.poem-workspace__actions{display:none}.poem-workspace__body{display:block;position:relative;min-height:0}.poem-workspace__content{position:absolute;inset:0}.poem-workspace__tools{position:absolute;z-index:4;left:0;right:0;bottom:0;height:46vh;border:0;border-radius:var(--poem-radius-lg) var(--poem-radius-lg) 0 0;box-shadow:0 calc(-1 * var(--poem-space-sm)) var(--poem-space-lg) var(--poem-shadow)}.poem-workspace__handle{display:flex;width:100%;height:1.5rem;border:0;background:transparent;align-items:center;justify-content:center;touch-action:none}.poem-workspace__handle span{width:3rem;height:.25rem;border-radius:100vmax;background:var(--poem-line-strong)}.poem-workspace__tools-scroll{height:calc(100% - 1.5rem);padding:var(--poem-space-md);scrollbar-color:var(--poem-line-strong) var(--poem-surface-raised)}.poem-workspace .trellis-image-viewport{min-height:100%}}
`
