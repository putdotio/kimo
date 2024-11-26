package server

import (
	"net/http"
	"time"
)

// UpdateHealth updates server health status
func (s *Server) UpdateHealth(err error) {
	s.healthMutex.Lock()
	defer s.healthMutex.Unlock()

	if err == nil {
		s.lastSuccessfulPoll = time.Now()
		s.lastPollError = nil
	} else {
		s.lastPollError = err
	}
}

// Health is the endpoint for health checks
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	s.healthMutex.RLock()
	defer s.healthMutex.RUnlock()

	pollThreshold := s.Config.PollInterval * 3 // Allow for up to 3 missed polls

	if time.Since(s.lastSuccessfulPoll) > pollThreshold {
		// todo: log
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if s.lastPollError != nil {
		// todo: log
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
