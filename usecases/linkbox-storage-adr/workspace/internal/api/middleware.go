package api

import (
	"net/http"
	"time"
)

// userHandler is a handler that runs for an authenticated user.
type userHandler func(w http.ResponseWriter, r *http.Request, user string)

// requireUser authenticates the X-API-Key header and passes the user on, or answers 401.
func (s *Server) requireUser(h userHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.keys.User(r.Header.Get("X-API-Key"))
		if !ok {
			w.Header().Set("WWW-Authenticate", `APIKey realm="linkbox"`)
			writeError(w, http.StatusUnauthorized, "missing or unknown API key")
			return
		}
		h(w, r, user)
	})
}

// statusRecorder remembers the status code a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration", time.Since(start))
	})
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				s.log.Error("handler panicked", "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
