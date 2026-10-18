// Package api serves the linkbox HTTP API.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"linkbox/internal/auth"
	"linkbox/internal/store"
)

// maxBodyBytes bounds a request body; a link is far smaller.
const maxBodyBytes = 64 << 10

// Server holds what the handlers need.
type Server struct {
	store *store.Store
	keys  *auth.Keys
	log   *slog.Logger
}

// NewServer returns a server over st that authenticates with keys.
func NewServer(st *store.Store, keys *auth.Keys, log *slog.Logger) *Server {
	return &Server{store: st, keys: keys, log: log}
}

// Handler returns the API with its middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("POST /links", s.requireUser(s.createLink))
	mux.Handle("GET /links", s.requireUser(s.listLinks))
	mux.Handle("GET /links/{id}", s.requireUser(s.getLink))
	mux.Handle("DELETE /links/{id}", s.requireUser(s.deleteLink))
	return s.recoverPanics(s.logRequests(mux))
}

type createRequest struct {
	URL   string   `json:"url"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

func (s *Server) createLink(w http.ResponseWriter, r *http.Request, user string) {
	var req createRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "body must hold one JSON object")
		return
	}
	l, err := s.store.Add(user, req.URL, req.Title, req.Tags)
	var ve *store.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusUnprocessableEntity, ve.Error())
		return
	case err != nil:
		s.log.Error("adding a link", "user", user, "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) listLinks(w http.ResponseWriter, r *http.Request, user string) {
	links := s.store.List(user, r.URL.Query().Get("tag"))
	if links == nil {
		links = []store.Link{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

func (s *Server) getLink(w http.ResponseWriter, r *http.Request, user string) {
	l, err := s.store.Get(user, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request, user string) {
	if err := s.store.Delete(user, r.PathValue("id")); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // the status is sent; a failed write means the client went away
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
