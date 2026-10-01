package mockserver

import (
	"net/http"
	"time"
)

// One gate per synthetic server, with no unbounded queue or externally selected
// target. It lets the isolated runner observe admission before changing Redis.
type responseGate struct {
	release       chan struct{}
	claimed       bool
	closed        bool
	streamStarted bool
}

func (s *Server) controlResponseGate(w http.ResponseWriter, r *http.Request, request object) {
	if r.Header.Get("Authorization") != "Bearer fixture-gate-control" {
		s.reject(w, "missing synthetic gate control key")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch request["action"] {
	case "arm":
		if s.gate != nil || s.load != nil {
			w.WriteHeader(http.StatusConflict)
			reply(w, object{"error": "fixture gate already armed"}, false)
			return
		}
		s.gate = &responseGate{release: make(chan struct{})}
	case "release":
		if s.gate != nil && !s.gate.closed {
			close(s.gate.release)
			s.gate.closed = true
			if !s.gate.claimed {
				s.gate = nil
			}
		}
	case "state":
	default:
		w.WriteHeader(http.StatusBadRequest)
		reply(w, object{"error": "unknown fixture gate action"}, false)
		return
	}
	reply(w, object{"armed": s.gate != nil, "entered": s.gate != nil && s.gate.claimed,
		"stream_started": s.gate != nil && s.gate.streamStarted}, false)
}

func (s *Server) waitResponseGate(w http.ResponseWriter, r *http.Request, stream bool) bool {
	s.mu.Lock()
	gate := s.gate
	if gate == nil {
		s.mu.Unlock()
		return true
	}
	if gate.claimed {
		s.mu.Unlock()
		if stream {
			reply(w, object{"error": object{"message": "fixture gate supports one request"}}, true)
		} else {
			s.reject(w, "fixture gate supports one request")
		}
		return false
	}
	gate.claimed = true
	gate.streamStarted = stream
	s.counts["response_gate_entered"]++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.gate == gate {
			s.gate = nil
		}
	}()
	// Below the fixture server's 15-second write and client's 20-second limits.
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	select {
	case <-gate.release:
		return true
	case <-r.Context().Done():
		if !stream {
			w.WriteHeader(http.StatusRequestTimeout)
		}
	case <-timer.C:
		if !stream {
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	}
	reply(w, object{"error": object{"message": "fixture gate cancelled or timed out"}}, stream)
	return false
}
