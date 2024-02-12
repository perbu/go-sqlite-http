package main

import (
	"log/slog"
	"net/http"
	"time"
)

// customResponseWriter is a wrapper around http.ResponseWriter that allows us to capture the status code.
type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newCustomResponseWriter(w http.ResponseWriter) *customResponseWriter {
	// Default the status code to 200 (OK), since if ServeHTTP never calls WriteHeader explicitly, the net/http package
	// assumes a 200 OK response.
	return &customResponseWriter{w, http.StatusOK}
}

func (crw *customResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs the request method, URL, elapsed time, and status code.
func LoggingMiddleware(logger *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Wrap the original ResponseWriter with our customResponseWriter.
		crw := newCustomResponseWriter(w)
		// Pass our customResponseWriter instead of the original one.
		h.ServeHTTP(crw, r)
		elapsed := time.Since(start)
		// Log the request details along with the status code.
		logger.Info(r.URL.String(), "elapsed", elapsed, "method", r.Method, "status", crw.statusCode)
	})
}

func Verboten(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/forbidden" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}
