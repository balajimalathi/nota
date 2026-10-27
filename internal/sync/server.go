package sync

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"nota/internal/manifest"
)

// Handler serves a vault for HTTP sync.
//
//	GET    /v1/manifest
//	GET    /v1/file?path=
//	PUT    /v1/file?path=
//	DELETE /v1/file?path=
func Handler(root, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/manifest", func(w http.ResponseWriter, r *http.Request) {
		if !allow(w, r, token) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		m, err := manifest.Scan(root, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m)
	})
	mux.HandleFunc("/v1/file", func(w http.ResponseWriter, r *http.Request) {
		if !allow(w, r, token) {
			return
		}
		rel, ok := cleanRel(r.URL.Query().Get("path"))
		if !ok {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		switch r.Method {
		case http.MethodGet:
			b, err := os.ReadFile(abs)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(b)
		case http.MethodPut:
			b, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := os.WriteFile(abs, b, 0o644); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func allow(w http.ResponseWriter, r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if got != token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func cleanRel(p string) (string, bool) {
	p = filepath.ToSlash(filepath.Clean(p))
	if p == "." || p == "" || strings.HasPrefix(p, "../") || p == ".." || strings.Contains(p, ":") {
		return "", false
	}
	return p, true
}
