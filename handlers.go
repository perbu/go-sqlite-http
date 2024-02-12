package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func handleHealthzPlease(logger *slog.Logger, conn *sqlitex.Pool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("healthz requested")
		w.WriteHeader(http.StatusOK)
	}
}

// handleDb is the default handler for the root path and every other path
// that is not explicitly handled by another handler.
func handleDb(logger *slog.Logger, pool *sqlitex.Pool) http.Handler {
	var (
		init sync.Once
	)
	// pick out a conn from the pool:
	conn := pool.Get(context.Background())
	if conn == nil { // not sure if this actually happens
		logger.Error("pool.Get", "error", "nil conn")
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	}
	defer pool.Put(conn)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		init.Do(func() {
			logger.Debug("db init")
			err := migrate(conn)
			if err != nil {
				logger.Error("migrate", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		})
		path := r.URL.Path[1:]
		if path == "" {
			path = "index.html"
		}
		logger.Debug("db invoked", "path", path, "method", r.Method)
		// pick out a pool from the pool:

		switch r.Method {
		case http.MethodGet:
			handleDbGet(path, w, r, logger, conn)
		case http.MethodPost:
			handleDbPost(path, w, r, logger, conn)
		case http.MethodPut:
			handleDbPut(path, w, r, logger, conn)
		case http.MethodDelete:
			handleDbDelete(path, w, r, logger, conn)
		default:
			logger.Info("request rejected, method not allowed", "method", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})
}

func handleDbGet(path string, w http.ResponseWriter, r *http.Request, logger *slog.Logger, conn *sqlite.Conn) {
	stmt := conn.Prep("SELECT content, content_type, created_at, updated_at FROM content WHERE path = $path")
	stmt.SetText("$path", path)
	// check if there is a row:
	hasRow, err := stmt.Step()
	if err != nil {
		logger.Error("stmt.Step", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !hasRow {
		logger.Info("not found", "path", path, "method", r.Method)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	lastModifiedStr := stmt.GetText("updated_at")
	if lastModifiedStr == "" {
		lastModifiedStr = stmt.GetText("created_at")
	}
	// parse the last modified time
	lastModified, err := time.Parse("2006-01-02 15:04:05", lastModifiedStr)
	if err != nil {
		logger.Error("time.Parse", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
	w.Header().Set("Content-Type", stmt.GetText("content_type"))
	n, err := io.Copy(w, stmt.GetReader("content"))
	if err != nil {
		logger.Error("io.Copy", "error", err, "n", n)
		return
	}
	err = stmt.Reset()
	if err != nil {
		logger.Error("stmt.Reset", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// if it has been more than 60s since the last update we should update the row
	if time.Since(lastModified) > 60*time.Second {
		err = sqlitex.Execute(conn, "UPDATE content SET accessed_at = datetime('now') WHERE path = $path", &sqlitex.ExecOptions{
			Args: []any{path},
		})
		if err != nil {
			logger.Error("sqlitex.Execute(set accessed_at)", "error", err, "path", path)
		}
	}
}

func handleDbPost(path string, w http.ResponseWriter, r *http.Request, logger *slog.Logger, conn *sqlite.Conn) {
	// start a transaction
	// find the length of the content:
	var err error
	clen := r.ContentLength
	if clen < 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ender := sqlitex.Transaction(conn) // consider defer with error here to have a rollback
	defer ender(&err)                  // this is quite a cool pattern.
	err = sqlitex.Execute(conn, `
INSERT INTO content (path, content, content_type, created_at) 
VALUES ($path, zeroblob($bloblen), $content_type, datetime('now'))`,
		&sqlitex.ExecOptions{
			Args: []any{path, clen, r.Header.Get("Content-Type")},
		})
	if err != nil {
		logger.Error("sqlitex.Execute", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	rowId := conn.LastInsertRowID()
	blob, err := conn.OpenBlob("", "content", "content", rowId, true)
	if err != nil {
		logger.Error("conn.OpenBlob", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer blob.Close()
	n, err := io.Copy(blob, r.Body)
	if err != nil {
		logger.Error("io.Copy", "error", err, "n", n)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = blob.Close()
	if err != nil {
		logger.Error("blob.Close", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger.Debug("blob written", "n", n, "path", path, "content-type", r.Header.Get("Content-Type"))
}

func handleDbPut(path string, w http.ResponseWriter, r *http.Request, logger *slog.Logger, conn *sqlite.Conn) {

}

func handleDbDelete(path string, w http.ResponseWriter, r *http.Request, logger *slog.Logger, conn *sqlite.Conn) {
	err := sqlitex.Execute(conn, "DELETE FROM content WHERE path = $path", &sqlitex.ExecOptions{
		Args: []any{path},
	})
	if err != nil {
		logger.Error("sqlitex.Execute", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// return a 404 if the row was not found
	if conn.Changes() == 0 {
		logger.Info("not found", "path", path, "method", r.Method)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

//go:embed init.sql
var initSQL string

func migrate(conn *sqlite.Conn) error {
	// check if there is a content table:

	sth, _, err := conn.PrepareTransient("SELECT name FROM sqlite_master WHERE type='table' AND name='content'")
	if err != nil {
		return fmt.Errorf("conn.Prepare(migration check): %w", err)
	}
	// return if the table exists
	if hasRow, err := sth.Step(); err != nil {
		return fmt.Errorf("sth.Step(migration check): %w", err)
	} else if hasRow {
		return nil
	}
	err = sth.Finalize()
	if err != nil {
		return fmt.Errorf("sth.Finalize(migration check): %w", err)
	}

	err = sqlitex.ExecScript(conn, initSQL)
	if err != nil {
		return fmt.Errorf("sqlitex.ExecScript: %w", err)
	}
	return nil
}
