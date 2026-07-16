package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// Middleware wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

type requestIDKey struct{}

// Chain applies middleware in the order supplied, from outermost to innermost.
func Chain(handler http.Handler, middleware ...Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

// RequestID creates or forwards X-Request-ID and adds it to the request context.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if !validRequestID(id) {
				id = newRequestID()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// RequestIDFromContext returns the request ID stored by RequestID.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// SecurityOptions controls SecurityHeaders. HSTS is disabled by default because
// applications should enable it only after HTTPS is enforced for their domain.
type SecurityOptions struct {
	// ContentSecurityPolicy overrides the default CSP when non-empty.
	ContentSecurityPolicy string
	// PermissionsPolicy overrides the default Permissions-Policy when non-empty.
	PermissionsPolicy string
	// HSTS enables Strict-Transport-Security for HTTPS-only deployments.
	HSTS bool
}

// SecurityHeaders adds baseline browser security headers. Passing no options
// uses the safe defaults; passing one option customizes the policy.
func SecurityHeaders() Middleware { return SecurityHeadersWithOptions(SecurityOptions{}) }

// SecurityHeadersWithOptions adds security headers using explicit options.
func SecurityHeadersWithOptions(options SecurityOptions) Middleware {
	config := SecurityOptions{
		ContentSecurityPolicy: "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'",
		PermissionsPolicy:     "camera=(), geolocation=(), microphone=()",
	}
	if options.ContentSecurityPolicy != "" {
		config.ContentSecurityPolicy = options.ContentSecurityPolicy
	}
	if options.PermissionsPolicy != "" {
		config.PermissionsPolicy = options.PermissionsPolicy
	}
	config.HSTS = options.HSTS
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			if config.HSTS {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LogRequests records method, path, status, duration, and request ID.
func LogRequests(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			started := time.Now()
			next.ServeHTTP(recorder, r)
			logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration", time.Since(started), "request_id", RequestIDFromContext(r.Context()))
		})
	}
}

// Recover logs an unexpected panic and returns an internal-server-error response.
func Recover(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered", "error", err, "request_id", RequestIDFromContext(r.Context()), "stack", string(debug.Stack()))
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

// Unwrap preserves access to optional net/http response capabilities.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(bytes[:])
}
