package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
)

// requestIDHeader is both read and written: a caller that already correlates
// requests keeps its own id, and everyone else gets one to quote in a bug report.
const requestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

// withRequestID puts a correlation id on every request and echoes it back.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id)))
	})
}

func requestIDOf(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

func newRequestID() string {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		// A correlation id is a convenience, never a reason to fail a request.
		return "unknown"
	}
	return hex.EncodeToString(buffer[:])
}

// recoverPanics turns a panicking handler into the same JSON 500 every other
// server-side failure produces, instead of a dropped connection.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer reportPanic(w, r)
		next.ServeHTTP(w, r)
	})
}

// reportPanic runs deferred, so it calls recover itself rather than taking the
// recovered value as an argument.
func reportPanic(w http.ResponseWriter, r *http.Request) {
	recovered := recover()
	if recovered == nil {
		return
	}
	slog.ErrorContext(r.Context(), "handler panicked",
		slog.Any("panic", recovered),
		slog.String("request_id", requestIDOf(r.Context())),
	)
	applicationError(w, r, apperr.New(apperr.CodeInternal, "handler panicked"))
}

// logRequests records one line per request: without it an owner-only service
// has no way to tell a slow plan from a failing one.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r)
		slog.InfoContext(r.Context(), "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.status),
			slog.Duration("duration", time.Since(started)),
			slog.String("request_id", requestIDOf(r.Context())),
		)
	})
}

// statusRecorder remembers the status the handler wrote so the access log can
// report it.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.written {
		r.status = status
		r.written = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}
