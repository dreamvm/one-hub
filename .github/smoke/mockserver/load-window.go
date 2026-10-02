package mockserver

import (
	"net/http"
	"time"
)

// A single bounded synthetic wave, never a generic upstream load generator.
// The server mutex protects all fields, including captures in completion callbacks.
type loadWindow struct {
	Started   int `json:"started"`
	Completed int `json:"completed"`
	Cancelled int `json:"cancelled"`
	Active    int `json:"active"`
	Peak      int `json:"peak"`
}

func (s *Server) controlLoadWindow(w http.ResponseWriter, r *http.Request, request object) {
	if r.Header.Get("Authorization") != "Bearer fixture-load-control" {
		s.reject(w, "missing synthetic load control key")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch request["action"] {
	case "arm":
		if s.load != nil || s.gate != nil {
			w.WriteHeader(http.StatusConflict)
			reply(w, object{"error": "another fixture window is armed"}, false)
			return
		}
		s.load = &loadWindow{}
	case "disarm":
		if s.load != nil && s.load.Active != 0 {
			w.WriteHeader(http.StatusConflict)
			reply(w, object{"error": "fixture requests remain active"}, false)
			return
		}
		s.load = nil
	case "state":
	default:
		w.WriteHeader(http.StatusBadRequest)
		reply(w, object{"error": "unknown fixture load action"}, false)
		return
	}
	reply(w, object{"armed": s.load != nil, "window": s.load}, false)
}

func (s *Server) enterLoadWindow(r *http.Request) (func(), bool) {
	s.mu.Lock()
	window := s.load
	if window == nil {
		s.mu.Unlock()
		return func() {}, true
	}
	if window.Started >= 32 || window.Active >= 4 {
		s.mu.Unlock()
		return func() {}, false
	}
	window.Started++
	window.Active++
	if window.Active > window.Peak {
		window.Peak = window.Active
	}
	s.mu.Unlock()
	finish := func(cancelled bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		window.Active--
		if cancelled {
			window.Cancelled++
		} else {
			window.Completed++
		}
	}
	// Briefly overlap the real gateway requests, with cancellation and no queue.
	timer := time.NewTimer(300 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return func() { finish(false) }, true
	case <-r.Context().Done():
		finish(true)
		return func() {}, false
	}
}
