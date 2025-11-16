package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Server provides HTTP API endpoints for the home automation system
type Server struct {
	stateManager *state.Manager
	logger       *zap.Logger
	server       *http.Server
}

// NewServer creates a new API server
func NewServer(stateManager *state.Manager, logger *zap.Logger, port int) *Server {
	s := &Server{
		stateManager: stateManager,
		logger:       logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", s.handleGetState)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// StateResponse represents the JSON response for the state endpoint
type StateResponse struct {
	Booleans map[string]bool    `json:"booleans"`
	Numbers  map[string]float64 `json:"numbers"`
	Strings  map[string]string  `json:"strings"`
	JSONs    map[string]any     `json:"jsons"`
}

// handleGetState returns all state variables as JSON
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := StateResponse{
		Booleans: make(map[string]bool),
		Numbers:  make(map[string]float64),
		Strings:  make(map[string]string),
		JSONs:    make(map[string]any),
	}

	// Collect all state variables by type
	for _, variable := range state.AllVariables {
		switch variable.Type {
		case state.TypeBool:
			value, err := s.stateManager.GetBool(variable.Key)
			if err != nil {
				s.logger.Error("Failed to get boolean variable",
					zap.String("key", variable.Key),
					zap.Error(err))
				continue
			}
			response.Booleans[variable.Key] = value

		case state.TypeNumber:
			value, err := s.stateManager.GetNumber(variable.Key)
			if err != nil {
				s.logger.Error("Failed to get number variable",
					zap.String("key", variable.Key),
					zap.Error(err))
				continue
			}
			response.Numbers[variable.Key] = value

		case state.TypeString:
			value, err := s.stateManager.GetString(variable.Key)
			if err != nil {
				s.logger.Error("Failed to get string variable",
					zap.String("key", variable.Key),
					zap.Error(err))
				continue
			}
			response.Strings[variable.Key] = value

		case state.TypeJSON:
			var value map[string]interface{}
			if err := s.stateManager.GetJSON(variable.Key, &value); err != nil {
				s.logger.Error("Failed to get JSON variable",
					zap.String("key", variable.Key),
					zap.Error(err))
				continue
			}
			response.JSONs[variable.Key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("State request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// Start begins serving HTTP requests
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP API server", zap.String("addr", s.server.Addr))

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop() error {
	s.logger.Info("Stopping HTTP API server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	return nil
}
