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

// SecurityTracker manages shadow state specifically for the security plugin
type SecurityTracker struct {
	mu    sync.RWMutex
	state *SecurityShadowState
}

// NewSecurityTracker creates a new security shadow state tracker
func NewSecurityTracker() *SecurityTracker {
	return &SecurityTracker{
		state: NewSecurityShadowState(),
	}
}

// UpdateCurrentInputs updates the current input values
func (st *SecurityTracker) UpdateCurrentInputs(inputs map[string]interface{}) {
	st.mu.Lock()
	defer st.mu.Unlock()

	for key, value := range inputs {
		st.state.Inputs.Current[key] = value
	}
	st.state.Metadata.LastUpdated = time.Now()
}

// SnapshotInputsForAction captures current inputs as the at-last-action snapshot
func (st *SecurityTracker) SnapshotInputsForAction() {
	st.mu.Lock()
	defer st.mu.Unlock()

	// Deep copy current inputs to at-last-action
	st.state.Inputs.AtLastAction = make(map[string]interface{})
	for key, value := range st.state.Inputs.Current {
		st.state.Inputs.AtLastAction[key] = value
	}
}

// RecordLockdownAction records a lockdown activation or deactivation
func (st *SecurityTracker) RecordLockdownAction(active bool, reason string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	st.state.Outputs.Lockdown.Active = active
	st.state.Outputs.Lockdown.Reason = reason

	if active {
		st.state.Outputs.Lockdown.ActivatedAt = now
		st.state.Outputs.Lockdown.WillResetAt = now.Add(5 * time.Second)
	} else {
		st.state.Outputs.Lockdown.ActivatedAt = time.Time{}
		st.state.Outputs.Lockdown.WillResetAt = time.Time{}
	}

	st.state.Outputs.LastActionTime = now
	st.state.Metadata.LastUpdated = now
}

// RecordDoorbellEvent records a doorbell press event
func (st *SecurityTracker) RecordDoorbellEvent(rateLimited bool, ttsSent bool, lightsFlashed bool) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	st.state.Outputs.LastDoorbell = &DoorbellEvent{
		Timestamp:     now,
		RateLimited:   rateLimited,
		TTSSent:       ttsSent,
		LightsFlashed: lightsFlashed,
	}
	st.state.Outputs.LastActionTime = now
	st.state.Metadata.LastUpdated = now
}

// RecordVehicleArrivalEvent records a vehicle arrival event
func (st *SecurityTracker) RecordVehicleArrivalEvent(rateLimited bool, ttsSent bool, wasExpecting bool) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	st.state.Outputs.LastVehicle = &VehicleArrivalEvent{
		Timestamp:    now,
		RateLimited:  rateLimited,
		TTSSent:      ttsSent,
		WasExpecting: wasExpecting,
	}
	st.state.Outputs.LastActionTime = now
	st.state.Metadata.LastUpdated = now
}

// RecordGarageOpenEvent records a garage auto-open event
func (st *SecurityTracker) RecordGarageOpenEvent(reason string, garageWasEmpty bool) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	st.state.Outputs.LastGarageOpen = &GarageOpenEvent{
		Timestamp:      now,
		Reason:         reason,
		GarageWasEmpty: garageWasEmpty,
	}
	st.state.Outputs.LastActionTime = now
	st.state.Metadata.LastUpdated = now
}

// GetState returns the current shadow state (thread-safe copy)
func (st *SecurityTracker) GetState() *SecurityShadowState {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	stateCopy := &SecurityShadowState{
		Plugin: st.state.Plugin,
		Inputs: SecurityInputs{
			Current:      make(map[string]interface{}),
			AtLastAction: make(map[string]interface{}),
		},
		Outputs: SecurityOutputs{
			Lockdown:       st.state.Outputs.Lockdown,
			LastDoorbell:   st.state.Outputs.LastDoorbell,
			LastVehicle:    st.state.Outputs.LastVehicle,
			LastGarageOpen: st.state.Outputs.LastGarageOpen,
			LastActionTime: st.state.Outputs.LastActionTime,
		},
		Metadata: st.state.Metadata,
	}

	// Copy current inputs
	for k, v := range st.state.Inputs.Current {
		stateCopy.Inputs.Current[k] = v
	}

	// Copy at-last-action inputs
	for k, v := range st.state.Inputs.AtLastAction {
		stateCopy.Inputs.AtLastAction[k] = v
	}

	return stateCopy
}

// LoadSheddingTracker manages shadow state specifically for the load shedding plugin
type LoadSheddingTracker struct {
	mu    sync.RWMutex
	state *LoadSheddingShadowState
}

// NewLoadSheddingTracker creates a new load shedding shadow state tracker
func NewLoadSheddingTracker() *LoadSheddingTracker {
	return &LoadSheddingTracker{
		state: NewLoadSheddingShadowState(),
	}
}

// UpdateCurrentInputs updates the current input values
func (lst *LoadSheddingTracker) UpdateCurrentInputs(inputs map[string]interface{}) {
	lst.mu.Lock()
	defer lst.mu.Unlock()

	for key, value := range inputs {
		lst.state.Inputs.Current[key] = value
	}
	lst.state.Metadata.LastUpdated = time.Now()
}

// SnapshotInputsForAction captures current inputs as the at-last-action snapshot
func (lst *LoadSheddingTracker) SnapshotInputsForAction() {
	lst.mu.Lock()
	defer lst.mu.Unlock()

	// Deep copy current inputs to at-last-action
	lst.state.Inputs.AtLastAction = make(map[string]interface{})
	for key, value := range lst.state.Inputs.Current {
		lst.state.Inputs.AtLastAction[key] = value
	}
}

// RecordLoadSheddingAction records a load shedding activation or deactivation
func (lst *LoadSheddingTracker) RecordLoadSheddingAction(active bool, actionType string, reason string, thermostatSettings ThermostatSettings) {
	lst.mu.Lock()
	defer lst.mu.Unlock()

	now := time.Now()
	lst.state.Outputs.Active = active
	lst.state.Outputs.LastActionType = actionType
	lst.state.Outputs.LastActionReason = reason
	lst.state.Outputs.ThermostatSettings = thermostatSettings
	lst.state.Outputs.LastActionTime = now
	lst.state.Metadata.LastUpdated = now
}

// GetState returns the current shadow state (thread-safe copy)
func (lst *LoadSheddingTracker) GetState() *LoadSheddingShadowState {
	lst.mu.RLock()
	defer lst.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	stateCopy := &LoadSheddingShadowState{
		Plugin: lst.state.Plugin,
		Inputs: LoadSheddingInputs{
			Current:      make(map[string]interface{}),
			AtLastAction: make(map[string]interface{}),
		},
		Outputs: LoadSheddingOutputs{
			Active:             lst.state.Outputs.Active,
			LastActionType:     lst.state.Outputs.LastActionType,
			LastActionReason:   lst.state.Outputs.LastActionReason,
			ThermostatSettings: lst.state.Outputs.ThermostatSettings,
			LastActionTime:     lst.state.Outputs.LastActionTime,
		},
		Metadata: lst.state.Metadata,
	}

	// Copy current inputs
	for k, v := range lst.state.Inputs.Current {
		stateCopy.Inputs.Current[k] = v
	}

	// Copy at-last-action inputs
	for k, v := range lst.state.Inputs.AtLastAction {
		stateCopy.Inputs.AtLastAction[k] = v
	}

	return stateCopy
}
