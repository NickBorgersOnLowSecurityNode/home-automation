package shadowstate

import "time"

// PluginShadowState is the interface that all plugin shadow states must implement
type PluginShadowState interface {
	GetCurrentInputs() map[string]interface{}
	GetLastActionInputs() map[string]interface{}
	GetOutputs() interface{}
	GetMetadata() StateMetadata
}

// InputSnapshot represents a snapshot of input values at a specific time
type InputSnapshot struct {
	Timestamp time.Time              `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
}

// StateMetadata contains metadata about the shadow state
type StateMetadata struct {
	LastUpdated time.Time `json:"lastUpdated"`
	PluginName  string    `json:"pluginName"`
}

// ActionRecord represents a single action taken by a plugin
type ActionRecord struct {
	Timestamp  time.Time              `json:"timestamp"`
	ActionType string                 `json:"actionType"`
	Reason     string                 `json:"reason"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// LightingShadowState represents the shadow state for the lighting plugin
type LightingShadowState struct {
	Plugin   string          `json:"plugin"`
	Inputs   LightingInputs  `json:"inputs"`
	Outputs  LightingOutputs `json:"outputs"`
	Metadata StateMetadata   `json:"metadata"`
}

// LightingInputs tracks current and last-action input values
type LightingInputs struct {
	Current      map[string]interface{} `json:"current"`
	AtLastAction map[string]interface{} `json:"atLastAction"`
}

// LightingOutputs tracks the state of lighting control outputs
type LightingOutputs struct {
	Rooms          map[string]RoomState `json:"rooms"`
	LastActionTime time.Time            `json:"lastActionTime"`
}

// RoomState represents the state of a single room
type RoomState struct {
	ActiveScene string    `json:"activeScene,omitempty"`
	TurnedOff   bool      `json:"turnedOff,omitempty"`
	LastAction  time.Time `json:"lastAction"`
	ActionType  string    `json:"actionType"` // "activate_scene" or "turn_off"
	Reason      string    `json:"reason"`
}

// GetCurrentInputs implements PluginShadowState
func (l *LightingShadowState) GetCurrentInputs() map[string]interface{} {
	return l.Inputs.Current
}

// GetLastActionInputs implements PluginShadowState
func (l *LightingShadowState) GetLastActionInputs() map[string]interface{} {
	return l.Inputs.AtLastAction
}

// GetOutputs implements PluginShadowState
func (l *LightingShadowState) GetOutputs() interface{} {
	return l.Outputs
}

// GetMetadata implements PluginShadowState
func (l *LightingShadowState) GetMetadata() StateMetadata {
	return l.Metadata
}

// NewLightingShadowState creates a new lighting shadow state
func NewLightingShadowState() *LightingShadowState {
	return &LightingShadowState{
		Plugin: "lighting",
		Inputs: LightingInputs{
			Current:      make(map[string]interface{}),
			AtLastAction: make(map[string]interface{}),
		},
		Outputs: LightingOutputs{
			Rooms:          make(map[string]RoomState),
			LastActionTime: time.Time{},
		},
		Metadata: StateMetadata{
			LastUpdated: time.Now(),
			PluginName:  "lighting",
		},
	}
}

// SecurityShadowState represents the shadow state for the security plugin
type SecurityShadowState struct {
	Plugin   string          `json:"plugin"`
	Inputs   SecurityInputs  `json:"inputs"`
	Outputs  SecurityOutputs `json:"outputs"`
	Metadata StateMetadata   `json:"metadata"`
}

// SecurityInputs tracks current and last-action input values
type SecurityInputs struct {
	Current      map[string]interface{} `json:"current"`
	AtLastAction map[string]interface{} `json:"atLastAction"`
}

// SecurityOutputs tracks the state of security control outputs
type SecurityOutputs struct {
	Lockdown       LockdownState        `json:"lockdown"`
	LastDoorbell   *DoorbellEvent       `json:"lastDoorbell,omitempty"`
	LastVehicle    *VehicleArrivalEvent `json:"lastVehicle,omitempty"`
	LastGarageOpen *GarageOpenEvent     `json:"lastGarageOpen,omitempty"`
	LastActionTime time.Time            `json:"lastActionTime"`
}

// LockdownState represents the current lockdown status
type LockdownState struct {
	Active      bool      `json:"active"`
	Reason      string    `json:"reason,omitempty"`
	ActivatedAt time.Time `json:"activatedAt,omitempty"`
	WillResetAt time.Time `json:"willResetAt,omitempty"`
}

// DoorbellEvent represents a doorbell press event
type DoorbellEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	RateLimited   bool      `json:"rateLimited"`
	TTSSent       bool      `json:"ttsSent"`
	LightsFlashed bool      `json:"lightsFlashed"`
}

// VehicleArrivalEvent represents a vehicle arrival notification
type VehicleArrivalEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	RateLimited  bool      `json:"rateLimited"`
	TTSSent      bool      `json:"ttsSent"`
	WasExpecting bool      `json:"wasExpecting"`
}

// GarageOpenEvent represents a garage auto-open event
type GarageOpenEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	Reason         string    `json:"reason"`
	GarageWasEmpty bool      `json:"garageWasEmpty"`
}

// GetCurrentInputs implements PluginShadowState
func (s *SecurityShadowState) GetCurrentInputs() map[string]interface{} {
	return s.Inputs.Current
}

// GetLastActionInputs implements PluginShadowState
func (s *SecurityShadowState) GetLastActionInputs() map[string]interface{} {
	return s.Inputs.AtLastAction
}

// GetOutputs implements PluginShadowState
func (s *SecurityShadowState) GetOutputs() interface{} {
	return s.Outputs
}

// GetMetadata implements PluginShadowState
func (s *SecurityShadowState) GetMetadata() StateMetadata {
	return s.Metadata
}

// NewSecurityShadowState creates a new security shadow state
func NewSecurityShadowState() *SecurityShadowState {
	return &SecurityShadowState{
		Plugin: "security",
		Inputs: SecurityInputs{
			Current:      make(map[string]interface{}),
			AtLastAction: make(map[string]interface{}),
		},
		Outputs: SecurityOutputs{
			Lockdown:       LockdownState{},
			LastActionTime: time.Time{},
		},
		Metadata: StateMetadata{
			LastUpdated: time.Now(),
			PluginName:  "security",
		},
	}
}
