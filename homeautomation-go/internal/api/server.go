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
	timezone      *time.Location
}

// NewServer creates a new API server
func NewServer(stateManager *state.Manager, shadowTracker *shadowstate.Tracker, logger *zap.Logger, port int, timezone *time.Location) *Server {
	s := &Server{
		stateManager:  stateManager,
		shadowTracker: shadowTracker,
		logger:        logger,
		timezone:      timezone,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleSitemap)
	mux.HandleFunc("/api/state", s.handleGetState)
	mux.HandleFunc("/api/states", s.handleGetStatesByPlugin)
	mux.HandleFunc("/api/shadow", s.handleGetAllShadowStates)
	mux.HandleFunc("/api/shadow/lighting", s.handleGetLightingShadowState)
	mux.HandleFunc("/api/shadow/music", s.handleGetMusicShadowState)
	mux.HandleFunc("/api/shadow/security", s.handleGetSecurityShadowState)
	mux.HandleFunc("/api/shadow/loadshedding", s.handleGetLoadSheddingShadowState)
	mux.HandleFunc("/api/shadow/sleephygiene", s.handleGetSleepHygieneShadowState)
	mux.HandleFunc("/api/shadow/energy", s.handleGetEnergyShadowState)
	mux.HandleFunc("/api/shadow/statetracking", s.handleGetStateTrackingShadowState)
	mux.HandleFunc("/api/shadow/dayphase", s.handleGetDayPhaseShadowState)
	mux.HandleFunc("/api/shadow/tv", s.handleGetTVShadowState)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/dashboard", s.handleDashboard)

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
			Path:        "/api/shadow/music",
			Method:      "GET",
			Description: "Get shadow state for music plugin - shows current mode, playlist, speaker group, and playback state",
		},
		{
			Path:        "/api/shadow/security",
			Method:      "GET",
			Description: "Get shadow state for security plugin - shows lockdown status, doorbell events, and garage actions",
		},
		{
			Path:        "/api/shadow/loadshedding",
			Method:      "GET",
			Description: "Get shadow state for load shedding plugin - shows load shedding status, thermostat settings, and energy-based decisions",
		},
		{
			Path:        "/api/shadow/sleephygiene",
			Method:      "GET",
			Description: "Get shadow state for sleep hygiene plugin - shows wake sequence status, fade-out progress, TTS announcements, and reminders",
		},
		{
			Path:        "/api/shadow/energy",
			Method:      "GET",
			Description: "Get shadow state for energy plugin - shows sensor readings, battery/solar/overall energy levels, and computation timestamps",
		},
		{
			Path:        "/api/shadow/statetracking",
			Method:      "GET",
			Description: "Get shadow state for state tracking plugin - shows derived presence/sleep states, timer states, and arrival announcements",
		},
		{
			Path:        "/api/shadow/dayphase",
			Method:      "GET",
			Description: "Get shadow state for day phase plugin - shows sun event, day phase, and calculation timestamps",
		},
		{
			Path:        "/api/shadow/tv",
			Method:      "GET",
			Description: "Get shadow state for TV plugin - shows Apple TV state, TV power, HDMI input, and playback status",
		},
		{
			Path:        "/health",
			Method:      "GET",
			Description: "Health check endpoint - returns {\"status\": \"ok\"}",
		},
		{
			Path:        "/dashboard",
			Method:      "GET",
			Description: "Shadow State Dashboard - web UI to visualize plugin states",
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
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Lighting shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetMusicShadowState returns the music plugin shadow state
func (s *Server) handleGetMusicShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("music")
	if !ok {
		http.Error(w, "Music shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Music shadow state request served",
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
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Security shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetLoadSheddingShadowState returns the shadow state for the load shedding plugin
func (s *Server) handleGetLoadSheddingShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("loadshedding")
	if !ok {
		http.Error(w, "Load shedding shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Load shedding shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetSleepHygieneShadowState returns the sleep hygiene plugin shadow state
func (s *Server) handleGetSleepHygieneShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("sleephygiene")
	if !ok {
		http.Error(w, "Sleep hygiene shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Sleep hygiene shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetEnergyShadowState returns the energy plugin shadow state
func (s *Server) handleGetEnergyShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("energy")
	if !ok {
		http.Error(w, "Energy shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Energy shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetStateTrackingShadowState returns the state tracking plugin shadow state
func (s *Server) handleGetStateTrackingShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("statetracking")
	if !ok {
		http.Error(w, "State tracking shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("State tracking shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetDayPhaseShadowState returns the day phase plugin shadow state
func (s *Server) handleGetDayPhaseShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("dayphase")
	if !ok {
		http.Error(w, "Day phase shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("Day phase shadow state request served",
		zap.String("remote_addr", r.RemoteAddr))
}

// handleGetTVShadowState returns the TV plugin shadow state
func (s *Server) handleGetTVShadowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, ok := s.shadowTracker.GetPluginState("tv")
	if !ok {
		http.Error(w, "TV shadow state not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := s.writeJSONWithLocalTimestamps(w, state); err != nil {
		s.logger.Error("Failed to encode shadow state response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.logger.Debug("TV shadow state request served",
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
	if err := s.writeJSONWithLocalTimestamps(w, response); err != nil {
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

// localTimeFormat is the human-readable format for local timestamps
const localTimeFormat = "Jan 2, 2006 3:04:05 PM MST"

// addLocalTimestamps recursively walks a JSON-compatible structure and adds
// "*Local" fields for any timestamp fields, formatted in the configured timezone.
// It returns a new structure with the local time fields added.
func (s *Server) addLocalTimestamps(data interface{}) interface{} {
	if s.timezone == nil {
		return data
	}

	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		// First pass: copy all existing keys
		for key, val := range v {
			result[key] = s.addLocalTimestamps(val)
		}
		// Second pass: add Local versions for timestamp strings
		for key, val := range v {
			if strVal, ok := val.(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, strVal); err == nil {
					localKey := key + "Local"
					result[localKey] = t.In(s.timezone).Format(localTimeFormat)
				} else if t, err := time.Parse(time.RFC3339, strVal); err == nil {
					localKey := key + "Local"
					result[localKey] = t.In(s.timezone).Format(localTimeFormat)
				}
			}
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = s.addLocalTimestamps(val)
		}
		return result

	default:
		return data
	}
}

// writeJSONWithLocalTimestamps encodes the given data as JSON, adding local
// timestamp fields for any RFC3339 timestamps found in the structure.
func (s *Server) writeJSONWithLocalTimestamps(w http.ResponseWriter, data interface{}) error {
	// First marshal to JSON, then unmarshal to map so we can transform
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var genericData interface{}
	if err := json.Unmarshal(jsonBytes, &genericData); err != nil {
		return err
	}

	// Add local timestamps
	transformed := s.addLocalTimestamps(genericData)

	// Encode the transformed data
	return json.NewEncoder(w).Encode(transformed)
}

// handleDashboard serves a web UI for visualizing shadow state
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)

	s.logger.Debug("Dashboard request served",
		zap.String("remote_addr", r.RemoteAddr))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadow State Dashboard</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #1a1a2e;
            color: #eee;
            min-height: 100vh;
            padding: 20px;
        }

        .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
            gap: 15px;
            margin-bottom: 20px;
            padding-bottom: 15px;
            border-bottom: 1px solid #0f3460;
        }

        .header h1 {
            font-size: 1.5rem;
            font-weight: 600;
            color: #eee;
        }

        .header-right {
            display: flex;
            align-items: center;
            gap: 20px;
            flex-wrap: wrap;
        }

        .last-updated {
            color: #888;
            font-size: 0.875rem;
        }

        .toggle-container {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 0.875rem;
        }

        .toggle-switch {
            position: relative;
            width: 44px;
            height: 24px;
            background: #333;
            border-radius: 12px;
            cursor: pointer;
            transition: background 0.2s;
        }

        .toggle-switch.active {
            background: #4ade80;
        }

        .toggle-switch::after {
            content: '';
            position: absolute;
            top: 2px;
            left: 2px;
            width: 20px;
            height: 20px;
            background: #fff;
            border-radius: 50%;
            transition: left 0.2s;
        }

        .toggle-switch.active::after {
            left: 22px;
        }

        .refresh-indicator {
            display: none;
            width: 16px;
            height: 16px;
            border: 2px solid #4ade80;
            border-top-color: transparent;
            border-radius: 50%;
            animation: spin 1s linear infinite;
        }

        .refresh-indicator.visible {
            display: inline-block;
        }

        @keyframes spin {
            to { transform: rotate(360deg); }
        }

        .plugins-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 15px;
        }

        .plugin-card {
            background: #16213e;
            border: 1px solid #0f3460;
            border-radius: 8px;
            overflow: hidden;
            transition: box-shadow 0.2s;
        }

        .plugin-card:hover {
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        }

        .plugin-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 15px;
            cursor: pointer;
            user-select: none;
        }

        .plugin-header:hover {
            background: rgba(255, 255, 255, 0.03);
        }

        .plugin-name {
            font-weight: 600;
            font-size: 1rem;
        }

        .plugin-meta {
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .plugin-updated {
            color: #888;
            font-size: 0.8rem;
        }

        .plugin-updated.stale-warning {
            color: #fbbf24;
        }

        .plugin-updated.stale-error {
            color: #f87171;
        }

        .expand-icon {
            color: #888;
            font-size: 0.8rem;
            transition: transform 0.2s;
        }

        .plugin-card.expanded .expand-icon {
            transform: rotate(180deg);
        }

        .plugin-content {
            display: none;
            padding: 0 15px 15px;
            border-top: 1px solid #0f3460;
        }

        .plugin-card.expanded .plugin-content {
            display: block;
        }

        .section-title {
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
            color: #888;
            margin: 15px 0 10px;
            letter-spacing: 0.5px;
        }

        .tree-node {
            font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Fira Mono', monospace;
            font-size: 0.85rem;
            line-height: 1.6;
        }

        .tree-item {
            display: flex;
            align-items: flex-start;
            padding: 2px 0;
        }

        .tree-prefix {
            color: #555;
            white-space: pre;
            flex-shrink: 0;
        }

        .tree-key {
            color: #9cdcfe;
            margin-right: 4px;
        }

        .tree-value {
            color: #ce9178;
            word-break: break-word;
        }

        .tree-value.bool-true {
            color: #4ade80;
        }

        .tree-value.bool-false {
            color: #f87171;
        }

        .tree-value.number {
            color: #b5cea8;
        }

        .tree-value.null {
            color: #888;
            font-style: italic;
        }

        .tree-toggle {
            cursor: pointer;
            color: #569cd6;
        }

        .tree-toggle:hover {
            text-decoration: underline;
        }

        .tree-children {
            margin-left: 0;
        }

        .tree-children.collapsed {
            display: none;
        }

        .error-message {
            background: rgba(248, 113, 113, 0.1);
            border: 1px solid #f87171;
            border-radius: 8px;
            padding: 20px;
            text-align: center;
            color: #f87171;
        }

        .loading {
            text-align: center;
            padding: 40px;
            color: #888;
        }

        @media (max-width: 640px) {
            body {
                padding: 15px;
            }

            .header {
                flex-direction: column;
                align-items: flex-start;
            }

            .plugins-grid {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Shadow State Dashboard</h1>
        <div class="header-right">
            <span class="last-updated" id="lastUpdated">Loading...</span>
            <div class="refresh-indicator" id="refreshIndicator"></div>
            <div class="toggle-container">
                <span>Auto-refresh</span>
                <div class="toggle-switch active" id="autoRefreshToggle" onclick="toggleAutoRefresh()"></div>
            </div>
        </div>
    </div>

    <div id="content" class="loading">Loading shadow state...</div>

    <script>
        let autoRefresh = true;
        let refreshInterval = null;
        const REFRESH_INTERVAL_MS = 30000;
        const STALE_WARNING_MS = 5 * 60 * 1000;  // 5 minutes
        const STALE_ERROR_MS = 15 * 60 * 1000;   // 15 minutes

        // Track which plugins and nodes are expanded
        const expandedPlugins = new Set();
        const expandedNodes = new Set();

        function toggleAutoRefresh() {
            autoRefresh = !autoRefresh;
            const toggle = document.getElementById('autoRefreshToggle');
            toggle.classList.toggle('active', autoRefresh);

            if (autoRefresh) {
                startAutoRefresh();
            } else {
                stopAutoRefresh();
            }
        }

        function startAutoRefresh() {
            if (refreshInterval) clearInterval(refreshInterval);
            refreshInterval = setInterval(fetchData, REFRESH_INTERVAL_MS);
        }

        function stopAutoRefresh() {
            if (refreshInterval) {
                clearInterval(refreshInterval);
                refreshInterval = null;
            }
        }

        function togglePlugin(pluginName) {
            if (expandedPlugins.has(pluginName)) {
                expandedPlugins.delete(pluginName);
            } else {
                expandedPlugins.add(pluginName);
            }

            const card = document.getElementById('plugin-' + pluginName);
            if (card) {
                card.classList.toggle('expanded', expandedPlugins.has(pluginName));
            }
        }

        function toggleNode(nodeId) {
            if (expandedNodes.has(nodeId)) {
                expandedNodes.delete(nodeId);
            } else {
                expandedNodes.add(nodeId);
            }

            const children = document.getElementById(nodeId);
            if (children) {
                children.classList.toggle('collapsed', !expandedNodes.has(nodeId));
            }

            const toggle = document.querySelector('[data-toggle="' + nodeId + '"]');
            if (toggle) {
                toggle.textContent = expandedNodes.has(nodeId) ? '▼' : '▶';
            }
        }

        function shouldShowKey(key, obj) {
            // Hide UTC keys if a *Local version exists
            if (obj && obj[key + 'Local'] !== undefined) {
                return false;
            }
            return true;
        }

        function formatRelativeTime(dateStr) {
            if (!dateStr) return '';

            try {
                const date = new Date(dateStr);
                const now = new Date();
                const diffMs = now - date;
                const diffSec = Math.floor(diffMs / 1000);
                const diffMin = Math.floor(diffSec / 60);
                const diffHour = Math.floor(diffMin / 60);
                const diffDay = Math.floor(diffHour / 24);

                if (diffSec < 60) return 'just now';
                if (diffMin < 60) return diffMin + 'm ago';
                if (diffHour < 24) return diffHour + 'h ago';
                return diffDay + 'd ago';
            } catch (e) {
                return '';
            }
        }

        function getStaleClass(dateStr) {
            if (!dateStr) return '';

            try {
                const date = new Date(dateStr);
                const now = new Date();
                const diffMs = now - date;

                if (diffMs >= STALE_ERROR_MS) return 'stale-error';
                if (diffMs >= STALE_WARNING_MS) return 'stale-warning';
                return '';
            } catch (e) {
                return '';
            }
        }

        function getPluginLastUpdated(pluginData) {
            // Try to find lastUpdated in metadata
            if (pluginData && pluginData.metadata && pluginData.metadata.lastUpdated) {
                return pluginData.metadata.lastUpdated;
            }
            // Try lastUpdatedLocal in metadata
            if (pluginData && pluginData.metadata && pluginData.metadata.lastUpdatedLocal) {
                return pluginData.metadata.lastUpdatedLocal;
            }
            return null;
        }

        let nodeIdCounter = 0;

        function renderValue(value, key, parentObj, prefix, depth) {
            const nodeId = 'node-' + (nodeIdCounter++);

            if (value === null || value === undefined) {
                return '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                       '<span class="tree-key">' + escapeHtml(key) + ':</span> ' +
                       '<span class="tree-value null">null</span></div>';
            }

            if (typeof value === 'boolean') {
                const icon = value ? '✓' : '✗';
                const cls = value ? 'bool-true' : 'bool-false';
                return '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                       '<span class="tree-key">' + escapeHtml(key) + ':</span> ' +
                       '<span class="tree-value ' + cls + '">' + icon + '</span></div>';
            }

            if (typeof value === 'number') {
                return '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                       '<span class="tree-key">' + escapeHtml(key) + ':</span> ' +
                       '<span class="tree-value number">' + value + '</span></div>';
            }

            if (typeof value === 'string') {
                return '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                       '<span class="tree-key">' + escapeHtml(key) + ':</span> ' +
                       '<span class="tree-value">"' + escapeHtml(value) + '"</span></div>';
            }

            if (Array.isArray(value)) {
                if (value.length === 0) {
                    return '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                           '<span class="tree-key">' + escapeHtml(key) + ':</span> ' +
                           '<span class="tree-value">[]</span></div>';
                }

                const isExpanded = expandedNodes.has(nodeId);
                let html = '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                           '<span class="tree-toggle" data-toggle="' + nodeId + '" onclick="toggleNode(\'' + nodeId + '\')">' +
                           (isExpanded ? '▼' : '▶') + '</span> ' +
                           '<span class="tree-key">' + escapeHtml(key) + '</span> [' + value.length + ']</div>';

                html += '<div id="' + nodeId + '" class="tree-children' + (isExpanded ? '' : ' collapsed') + '">';
                const childPrefix = prefix.replace(/[├└]/g, '│').replace(/─/g, ' ') + '  ';
                for (let i = 0; i < value.length; i++) {
                    const itemPrefix = childPrefix + (i === value.length - 1 ? '└─ ' : '├─ ');
                    html += renderValue(value[i], '[' + i + ']', value, itemPrefix, depth + 1);
                }
                html += '</div>';
                return html;
            }

            if (typeof value === 'object') {
                const keys = Object.keys(value).filter(k => shouldShowKey(k, value));
                if (keys.length === 0) {
                    return '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                           '<span class="tree-key">' + escapeHtml(key) + ':</span> ' +
                           '<span class="tree-value">{}</span></div>';
                }

                const isExpanded = expandedNodes.has(nodeId);
                let html = '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                           '<span class="tree-toggle" data-toggle="' + nodeId + '" onclick="toggleNode(\'' + nodeId + '\')">' +
                           (isExpanded ? '▼' : '▶') + '</span> ' +
                           '<span class="tree-key">' + escapeHtml(key) + '</span></div>';

                html += '<div id="' + nodeId + '" class="tree-children' + (isExpanded ? '' : ' collapsed') + '">';
                const childPrefix = prefix.replace(/[├└]/g, '│').replace(/─/g, ' ') + '  ';
                for (let i = 0; i < keys.length; i++) {
                    const itemPrefix = childPrefix + (i === keys.length - 1 ? '└─ ' : '├─ ');
                    html += renderValue(value[keys[i]], keys[i], value, itemPrefix, depth + 1);
                }
                html += '</div>';
                return html;
            }

            return '<div class="tree-item"><span class="tree-prefix">' + prefix + '</span>' +
                   '<span class="tree-key">' + escapeHtml(key) + ':</span> ' +
                   '<span class="tree-value">' + escapeHtml(String(value)) + '</span></div>';
        }

        function renderSection(title, data) {
            if (!data || (typeof data === 'object' && Object.keys(data).length === 0)) {
                return '';
            }

            let html = '<div class="section-title">' + escapeHtml(title) + '</div>';
            html += '<div class="tree-node">';

            if (typeof data === 'object' && !Array.isArray(data)) {
                const keys = Object.keys(data).filter(k => shouldShowKey(k, data));
                for (let i = 0; i < keys.length; i++) {
                    const prefix = (i === keys.length - 1 ? '└─ ' : '├─ ');
                    html += renderValue(data[keys[i]], keys[i], data, prefix, 0);
                }
            } else {
                html += renderValue(data, title, null, '', 0);
            }

            html += '</div>';
            return html;
        }

        function renderPlugin(name, data) {
            const lastUpdated = getPluginLastUpdated(data);
            const relativeTime = formatRelativeTime(lastUpdated);
            const staleClass = getStaleClass(lastUpdated);
            const isExpanded = expandedPlugins.has(name);

            let html = '<div class="plugin-card' + (isExpanded ? ' expanded' : '') + '" id="plugin-' + escapeHtml(name) + '">';
            html += '<div class="plugin-header" onclick="togglePlugin(\'' + escapeHtml(name) + '\')">';
            html += '<span class="plugin-name">' + escapeHtml(name) + '</span>';
            html += '<div class="plugin-meta">';
            if (relativeTime) {
                html += '<span class="plugin-updated ' + staleClass + '">Updated ' + relativeTime + '</span>';
            }
            html += '<span class="expand-icon">▼</span>';
            html += '</div></div>';

            html += '<div class="plugin-content">';

            // Render outputs first (most important)
            if (data.outputs) {
                html += renderSection('Outputs', data.outputs);
            }

            // Then inputs
            if (data.inputs) {
                html += renderSection('Inputs', data.inputs);
            }

            // Then metadata
            if (data.metadata) {
                html += renderSection('Metadata', data.metadata);
            }

            html += '</div></div>';
            return html;
        }

        function escapeHtml(str) {
            if (typeof str !== 'string') return str;
            const div = document.createElement('div');
            div.textContent = str;
            return div.innerHTML;
        }

        async function fetchData() {
            const indicator = document.getElementById('refreshIndicator');
            indicator.classList.add('visible');

            try {
                const response = await fetch('/api/shadow');
                if (!response.ok) {
                    throw new Error('HTTP ' + response.status);
                }

                const data = await response.json();

                // Update last updated time
                const now = new Date();
                document.getElementById('lastUpdated').textContent =
                    'Last updated: ' + now.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});

                // Reset node counter for consistent IDs
                nodeIdCounter = 0;

                // Render plugins
                const content = document.getElementById('content');

                if (!data.plugins || Object.keys(data.plugins).length === 0) {
                    content.innerHTML = '<div class="error-message">No plugins found in shadow state</div>';
                    return;
                }

                let html = '<div class="plugins-grid">';
                const pluginNames = Object.keys(data.plugins).sort();
                for (const name of pluginNames) {
                    html += renderPlugin(name, data.plugins[name]);
                }
                html += '</div>';

                content.innerHTML = html;
                content.classList.remove('loading');

            } catch (error) {
                console.error('Failed to fetch shadow state:', error);
                document.getElementById('content').innerHTML =
                    '<div class="error-message">Failed to load shadow state: ' + escapeHtml(error.message) + '</div>';
            } finally {
                indicator.classList.remove('visible');
            }
        }

        // Initial fetch and start auto-refresh
        fetchData();
        startAutoRefresh();
    </script>
</body>
</html>
`
