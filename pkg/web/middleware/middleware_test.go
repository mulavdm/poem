package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestIDAndSecurityHeaders(t *testing.T) {
	handler := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestIDFromContext(r.Context()) == "" {
			t.Fatal("request ID missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}), RequestID(), SecurityHeaders())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, header := range []string{"X-Request-ID", "Content-Security-Policy", "Permissions-Policy", "X-Content-Type-Options"} {
		if recorder.Header().Get(header) == "" {
			t.Fatalf("header %s missing", header)
		}
	}
}

func TestRequestIDRejectsUntrustedShape(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestIDFromContext(r.Context()) == "bad value" {
			t.Fatal("invalid request ID was trusted")
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "bad value")
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("X-Request-ID") == "bad value" {
		t.Fatal("invalid request ID was forwarded")
	}
}

func TestRecoveryAndRequestLogging(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), RequestID(), LogRequests(logger), Recover(logger))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !bytes.Contains(logs.Bytes(), []byte(`status=500`)) {
		t.Fatalf("recovered request was not access-logged: %s", logs.String())
	}
}

func TestStatusRecorderUsesFirstStatusAndUnwraps(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &statusRecorder{ResponseWriter: recorder, status: http.StatusOK}
	wrapped.WriteHeader(http.StatusCreated)
	wrapped.WriteHeader(http.StatusInternalServerError)
	if wrapped.status != http.StatusCreated || recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, recorder = %d", wrapped.status, recorder.Code)
	}
	if wrapped.Unwrap() != recorder {
		t.Fatal("Unwrap did not return the original response writer")
	}
}

func TestStatusRecorderSupportsResponseControllerUnwrap(t *testing.T) {
	writer := &deadlineWriter{header: make(http.Header)}
	wrapped := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
	if err := http.NewResponseController(wrapped).SetWriteDeadline(time.Now()); err != nil {
		t.Fatalf("SetWriteDeadline() error = %v", err)
	}
	if !writer.deadlineSet {
		t.Fatal("response controller did not unwrap the recorder")
	}
}

type deadlineWriter struct {
	header      http.Header
	deadlineSet bool
}

func (w *deadlineWriter) Header() http.Header              { return w.header }
func (w *deadlineWriter) Write(body []byte) (int, error)   { return len(body), nil }
func (w *deadlineWriter) WriteHeader(int)                  {}
func (w *deadlineWriter) SetWriteDeadline(time.Time) error { w.deadlineSet = true; return nil }
