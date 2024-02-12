package main

import (
	"log/slog"
	"net/http"
	"time"
)

func LoggingMiddleware(logger *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		elapsed := time.Since(start)
		logger.Info(r.URL.String(), "elapsed", elapsed, "method", r.Method)
	})
}
