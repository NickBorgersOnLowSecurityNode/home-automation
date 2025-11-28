package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"homeautomation/internal/shadowstate"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Server provides HTTP API endpoints for the home automation system
type Server struct {
	stateManager  *state.Manager
	shadowTracker *shadowstate.Tracker
	logger        *zap.Logger
	server        *http.Server
}

// NewServer creates a new API server
func NewServer(stateManager *state.Manager, shadowTracker *shadowstate.Tracker, logger *zap.Logger, port int) *Server {
	s := &Server{
		stateManager:  stateManager,
		shadowTracker: shadowTracker,
		logger:        logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleSitemap)
	mux.HandleFunc("/api/state", s.handleGetState)
	mux.HandleFunc("/api/states", s.handleGetStatesByPlugin)
	mux.HandleFunc("/api/shadow", s.handleGetAllShadowStates)
	mux.HandleFunc("/api/shadow/lighting", s.handleGetLightingShadowState)
	mux.HandleFunc("/api/shadow/security", s.handleGetSecurityShadowState)
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

// PluginMetadata describes which state variables a plugin uses
type PluginMetadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Reads       []string `json:"reads"`
	Writes      []string `json:"writes"`
}

// PluginStateValue represents a state variable value with type information
type PluginStateValue struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

// PluginStatesResponse represents the response for /api/states endpoint
type PluginStatesResponse struct {
	Plugins map[string]map[string]PluginStateValue `json:"plugins"`
}

// pluginRegistry defines which state variables each plugin reads/writes
var pluginRegistry = []PluginMetadata{
	{
		Name:        "statetracking",
		Description: "Tracks presence and sleep states, computes derived states",
		Reads:       []string{"isNickHome", "isCarolineHome", "isToriHere"},
		Writes:      []string{"isAnyOwnerHome", "isAnyoneHome", "isAnyoneAsleep", "isEveryoneAsleep", "isMasterAsleep", "isGuestAsleep", "didOwnerJustReturnHome"},
	},
	{
		Name:        "dayphase",
		Description: "Tracks time of day and sun position",
		Reads:       []string{},
		Writes:      []string{"dayPhase", "sunevent"},
	},
	{
		Name:        "music",
		Description: "Manages music playback mode and Sonos control",
		Reads:       []string{"dayPhase", "isAnyoneAsleep", "isAnyoneHome", "musicPlaybackType"},
		Writes:      []string{"musicPlaybackType", "currentlyPlayingMusicUri"},
	},
	{
		Name:        "lighting",
		Description: "Controls lighting scenes based on time, presence, and activity",
		Reads:       []string{"dayPhase", "sunevent", "isAnyoneHome", "isTVPlaying", "isEveryoneAsleep", "isMasterAsleep", "isHaveGuests"},
		Writes:      []string{},
	},
	{
		Name:        "tv",
		Description: "Monitors TV and Apple TV playback state",
		Reads:       []string{"isAppleTVPlaying"},
		Writes:      []string{"isAppleTVPlaying", "isTVon", "isTVPlaying"},
	},
	{
		Name:        "energy",
		Description: "Monitors battery, solar production, and grid availability",
		Reads:       []string{"isGridAvailable", "batteryEnergyLevel", "solarProductionEnergyLevel", "isFreeEnergyAvailable"},
		Writes:      []string{"batteryEnergyLevel", "thisHourSolarGeneration", "remainingSolarGeneration", "solarProductionEnergyLevel", "currentEnergyLevel", "isFreeEnergyAvailable"},
	},
	{
		Name:        "loadshedding",
		Description: "Controls thermostat based on available energy",
		Reads:       []string{"currentEnergyLevel"},
		Writes:      []string{},
	},
	{
		Name:        "sleephygiene",
		Description: "Manages wake-up sequences and bedtime routines",
		Reads:       []string{"alarmTime"},
		Writes:      []string{"isFadeOutInProgress", "currentlyPlayingMusic", "musicPlaybackType"},
	},
	{
		Name:        "security",
		Description: "Manages security automation based on presence and sleep",
		Reads:       []string{"isEveryoneAsleep", "isAnyoneHome", "didOwnerJustReturnHome", "isExpectingSomeone"},
		Writes:      []string{},
	},
	{
		Name:        "reset",
		Description: "Coordinates system-wide state resets",
		Reads:       []string{"reset"},
		Writes:      []string{},
	},
}

// handleGetStatesByPlugin returns state variables grouped by which plugins use them
func (s *Server) handleGetStatesByPlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := PluginStatesResponse{
		Plugins: make(map[string]map[string]PluginStateValue),
	}

	// For each plugin, collect the state variables it uses
	for _, plugin := range pluginRegistry {
		pluginStates := make(map[string]PluginStateValue)

		// Collect all unique variables (both reads and writes)
		variableSet := make(map[string]bool)
		for _, v := range plugin.Reads {
			variableSet[v] = true
		}
		for _, v := range plugin.Writes {
			variableSet[v] = true
		}

		// Get the current value of each variable
		for varName := range variableSet {
			value, varType := s.getStateVariableValue(varName)
			if varType != "" {
				pluginStates[varName] = PluginStateValue{
					Value: value,
					Type:  varType,
				}
			}
		}

		response.Plugins[plugin.Name] = pluginStates
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("States by plugin request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// getStateVariableValue retrieves a state variable's value and type
func (s *Server) getStateVariableValue(key string) (interface{}, string) {
	// Find the variable definition to determine its type
	for _, variable := range state.AllVariables {
		if variable.Key != key {
			continue
		}

		switch variable.Type {
		case state.TypeBool:
			value, err := s.stateManager.GetBool(key)
			if err != nil {
				s.logger.Error("Failed to get boolean variable",
					zap.String("key", key),
					zap.Error(err))
				return nil, ""
			}
			return value, "boolean"

		case state.TypeNumber:
			value, err := s.stateManager.GetNumber(key)
			if err != nil {
				s.logger.Error("Failed to get number variable",
					zap.String("key", key),
					zap.Error(err))
				return nil, ""
			}
			return value, "number"

		case state.TypeString:
			value, err := s.stateManager.GetString(key)
			if err != nil {
				s.logger.Error("Failed to get string variable",
					zap.String("key", key),
					zap.Error(err))
				return nil, ""
			}
			return value, "string"

		case state.TypeJSON:
			var value map[string]interface{}
			if err := s.stateManager.GetJSON(key, &value); err != nil {
				s.logger.Error("Failed to get JSON variable",
					zap.String("key", key),
					zap.Error(err))
				return nil, ""
			}
			return value, "json"
		}
	}

	s.logger.Warn("Unknown state variable requested", zap.String("key", key))
	return nil, ""
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
			Description: "Get all state variables grouped by type (booleans, numbers, strings, jsons)",
		},
		{
			Path:        "/api/states",
			Method:      "GET",
			Description: "Get state variables grouped by plugin - shows which plugins use which variables",
		},
		{
			Path:        "/api/shadow",
			Method:      "GET",
			Description: "Get shadow state for all plugins - shows current inputs, inputs at last action, and outputs",
		},
		{
			Path:        "/api/shadow/lighting",
			Method:      "GET",
			Description: "Get shadow state for lighting plugin - shows room states and lighting decisions",
		},
		{
			Path:        "/api/shadow/security",
			Method:      "GET",
			Description: "Get shadow state for security plugin - shows lockdown status, doorbell events, and garage actions",
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
        <div>Get all state variables (by type):</div>
        <div class="description">curl <a href="/api/state">http://localhost:8081/api/state</a></div>
    </div>
    <div class="endpoint">
        <div>Get state variables by plugin:</div>
        <div class="description">curl <a href="/api/states">http://localhost:8081/api/states</a></div>
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
		fmt.Fprintf(w, "  Get all state variables (by type):\n")
		fmt.Fprintf(w, "    curl http://localhost:8081/api/state\n\n")
		fmt.Fprintf(w, "  Get state variables by plugin:\n")
		fmt.Fprintf(w, "    curl http://localhost:8081/api/states\n\n")
		fmt.Fprintf(w, "  Health check:\n")
		fmt.Fprintf(w, "    curl http://localhost:8081/health\n\n")
		fmt.Fprintf(w, "  Pretty print JSON:\n")
		fmt.Fprintf(w, "    curl http://localhost:8081/api/states | jq\n\n")
	}

	s.logger.Debug("Sitemap request served",
		zap.String("remote_addr", r.RemoteAddr),
		zap.Bool("html_format", preferHTML))
}

// handleGetLightingShadowState returns the shadow state for the lighting plugin
func (s *Server) handleGetLightingShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("lighting")
	if !ok {
		http.Error(w, "Lighting shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Lighting shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetSecurityShadowState returns the shadow state for the security plugin
func (s *Server) handleGetSecurityShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("security")
	if !ok {
		http.Error(w, "Security shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Security shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// AllShadowStatesResponse represents the response for /api/shadow endpoint
type AllShadowStatesResponse struct {
	Plugins  map[string]interface{} `json:"plugins"`
	Metadata ShadowMetadata         `json:"metadata"`
}

// ShadowMetadata contains metadata about the shadow state response
type ShadowMetadata struct {
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// handleGetAllShadowStates returns shadow states for all plugins
func (s *Server) handleGetAllShadowStates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allStates := s.shadowTracker.GetAllPluginStates()

	// Convert to map[string]interface{} for JSON encoding
	pluginsData := make(map[string]interface{})
	for name, state := range allStates {
		pluginsData[name] = state
	}

	response := AllShadowStatesResponse{
		Plugins: pluginsData,
		Metadata: ShadowMetadata{
			Timestamp: time.Now(),
			Version:   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode shadow states response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("All shadow states request served",
		zap.String("remote_addr", r.RemoteAddr),
		zap.Int("plugin_count", len(allStates)))
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
