package main

import (
	"log/slog"
	"net/http"
	"zombiezen.com/go/sqlite/sqlitex"
)

func addRoutes(mux *http.ServeMux, logger *slog.Logger, conn *sqlitex.Pool) {
	mux.HandleFunc("/healthz", handleHealthzPlease(logger, conn))
	mux.Handle("/", handleDb(logger, conn))

}
