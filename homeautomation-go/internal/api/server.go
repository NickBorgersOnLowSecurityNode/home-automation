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
	mux.HandleFunc("/", s.handleSitemap)
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

// Endpoint represents an API endpoint with its documentation
type Endpoint struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
}

// handleSitemap returns a list of all available API endpoints
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	// Only handle requests to the root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	endpoints := []Endpoint{
		{
			Path:        "/",
			Method:      "GET",
			Description: "This sitemap - lists all available API endpoints",
		},
		{
			Path:        "/api/state",
			Method:      "GET",
			Description: "Get all state variables (booleans, numbers, strings, jsons)",
		},
		{
			Path:        "/health",
			Method:      "GET",
			Description: "Health check endpoint - returns {\"status\": \"ok\"}",
		},
	}

	// Determine if the request is from a browser (check Accept header)
	acceptHeader := r.Header.Get("Accept")
	preferHTML := false
	if acceptHeader != "" {
		// Simple check - if Accept contains text/html, prefer HTML
		for _, part := range []string{"text/html", "*/*"} {
			if len(acceptHeader) > 0 && (acceptHeader == part || len(acceptHeader) > len(part) && acceptHeader[:len(part)] == part) {
				preferHTML = true
				break
			}
		}
	}

	// Return 404 status code (for automation compatibility) but with helpful body
	w.WriteHeader(http.StatusNotFound)

	if preferHTML {
		// HTML format for browsers
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Home Automation API</title>
    <style>
        body { font-family: monospace; margin: 40px; background: #1e1e1e; color: #d4d4d4; }
        h1 { color: #4ec9b0; }
        h2 { color: #569cd6; margin-top: 30px; }
        .endpoint { background: #2d2d2d; padding: 15px; margin: 10px 0; border-left: 3px solid #007acc; }
        .method { color: #4ec9b0; font-weight: bold; }
        .path { color: #ce9178; }
        .description { color: #9cdcfe; margin-top: 5px; }
        a { color: #569cd6; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>Home Automation API</h1>
    <p>Welcome! This API provides access to the home automation system state.</p>
    <h2>Available Endpoints</h2>
`)
		for _, ep := range endpoints {
			fmt.Fprintf(w, `    <div class="endpoint">
        <div><span class="method">%s</span> <span class="path">%s</span></div>
        <div class="description">%s</div>
    </div>
`, ep.Method, ep.Path, ep.Description)
		}
		fmt.Fprintf(w, `    <h2>Examples</h2>
    <div class="endpoint">
        <div>Get all state variables:</div>
        <div class="description">curl <a href="/api/state">http://localhost:8081/api/state</a></div>
    </div>
    <div class="endpoint">
        <div>Health check:</div>
        <div class="description">curl <a href="/health">http://localhost:8081/health</a></div>
    </div>
</body>
</html>
`)
	} else {
		// Plain text format for terminal
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Home Automation API\n")
		fmt.Fprintf(w, "===================\n\n")
		fmt.Fprintf(w, "Available endpoints:\n\n")
		for _, ep := range endpoints {
			fmt.Fprintf(w, "  %-10s %-20s %s\n", ep.Method, ep.Path, ep.Description)
		}
		fmt.Fprintf(w, "\nExamples:\n\n")
		fmt.Fprintf(w, "  Get all state variables:\n")
		fmt.Fprintf(w, "    curl http://localhost:8081/api/state\n\n")
		fmt.Fprintf(w, "  Health check:\n")
		fmt.Fprintf(w, "    curl http://localhost:8081/health\n\n")
		fmt.Fprintf(w, "  Pretty print JSON:\n")
		fmt.Fprintf(w, "    curl http://localhost:8081/api/state | jq\n\n")
	}

	s.logger.Debug("Sitemap request served",
		zap.String("remote_addr", r.RemoteAddr),
		zap.Bool("html_format", preferHTML))
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
