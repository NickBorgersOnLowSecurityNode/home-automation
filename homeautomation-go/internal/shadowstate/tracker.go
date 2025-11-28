package shadowstate

import (
	"sync"
	"time"
)

// Tracker manages shadow state for all plugins
type Tracker struct {
	mu             sync.RWMutex
	pluginStates   map[string]PluginShadowState
	stateProviders map[string]func() PluginShadowState
}

// NewTracker creates a new shadow state tracker
func NewTracker() *Tracker {
	return &Tracker{
		pluginStates:   make(map[string]PluginShadowState),
		stateProviders: make(map[string]func() PluginShadowState),
	}
}

// RegisterPlugin registers a plugin's shadow state
func (t *Tracker) RegisterPlugin(pluginName string, state PluginShadowState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pluginStates[pluginName] = state
}

// RegisterPluginProvider registers a function that provides a plugin's shadow state dynamically
func (t *Tracker) RegisterPluginProvider(pluginName string, provider func() PluginShadowState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stateProviders[pluginName] = provider
}

// GetPluginState retrieves a plugin's shadow state
func (t *Tracker) GetPluginState(pluginName string) (PluginShadowState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Check provider first (dynamic state)
	if provider, ok := t.stateProviders[pluginName]; ok {
		return provider(), true
	}

	// Fall back to static state
	state, ok := t.pluginStates[pluginName]
	return state, ok
}

// GetAllPluginStates retrieves all plugin shadow states
func (t *Tracker) GetAllPluginStates() map[string]PluginShadowState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Create a copy to avoid race conditions
	// Include both static states and provider states
	totalSize := len(t.pluginStates) + len(t.stateProviders)
	states := make(map[string]PluginShadowState, totalSize)

	// Add static states
	for k, v := range t.pluginStates {
		states[k] = v
	}

	// Add provider states (these take precedence if there's a name collision)
	for k, provider := range t.stateProviders {
		states[k] = provider()
	}

	return states
}

// LightingTracker manages shadow state specifically for the lighting plugin
type LightingTracker struct {
	mu    sync.RWMutex
	state *LightingShadowState
}

// NewLightingTracker creates a new lighting shadow state tracker
func NewLightingTracker() *LightingTracker {
	return &LightingTracker{
		state: NewLightingShadowState(),
	}
}

// UpdateCurrentInputs updates the current input values
func (lt *LightingTracker) UpdateCurrentInputs(inputs map[string]interface{}) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	for key, value := range inputs {
		lt.state.Inputs.Current[key] = value
	}
	lt.state.Metadata.LastUpdated = time.Now()
}

// SnapshotInputsForAction captures current inputs as the at-last-action snapshot
func (lt *LightingTracker) SnapshotInputsForAction() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	// Deep copy current inputs to at-last-action
	lt.state.Inputs.AtLastAction = make(map[string]interface{})
	for key, value := range lt.state.Inputs.Current {
		lt.state.Inputs.AtLastAction[key] = value
	}
}

// RecordRoomAction records an action taken on a room
func (lt *LightingTracker) RecordRoomAction(roomName string, actionType string, reason string, activeScene string, turnedOff bool) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	now := time.Now()
	lt.state.Outputs.Rooms[roomName] = RoomState{
		ActiveScene: activeScene,
		TurnedOff:   turnedOff,
		LastAction:  now,
		ActionType:  actionType,
		Reason:      reason,
	}
	lt.state.Outputs.LastActionTime = now
	lt.state.Metadata.LastUpdated = now
}

// GetState returns the current shadow state (thread-safe copy)
func (lt *LightingTracker) GetState() *LightingShadowState {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	stateCopy := &LightingShadowState{
		Plugin: lt.state.Plugin,
		Inputs: LightingInputs{
			Current:      make(map[string]interface{}),
			AtLastAction: make(map[string]interface{}),
		},
		Outputs: LightingOutputs{
			Rooms:          make(map[string]RoomState),
			LastActionTime: lt.state.Outputs.LastActionTime,
		},
		Metadata: lt.state.Metadata,
	}

	// Copy current inputs
	for k, v := range lt.state.Inputs.Current {
		stateCopy.Inputs.Current[k] = v
	}

	// Copy at-last-action inputs
	for k, v := range lt.state.Inputs.AtLastAction {
		stateCopy.Inputs.AtLastAction[k] = v
	}

	// Copy room states
	for k, v := range lt.state.Outputs.Rooms {
		stateCopy.Outputs.Rooms[k] = v
	}

	return stateCopy
}
