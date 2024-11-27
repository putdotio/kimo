package server

import (
	"net/http"
	"time"

	"github.com/cenkalti/log"
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

	if s.lastSuccessfulPoll.IsZero() {
		log.Warningln("Initial poll is not finished yet!")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	timePassed := time.Since(s.lastSuccessfulPoll)
	if timePassed > pollThreshold {
		log.Errorf("Last successful poll was %s ago!\n", timePassed)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if s.lastPollError != nil {
		log.Errorf("Last poll error: %s\n", s.lastPollError.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
