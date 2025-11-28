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

// SleepHygieneTracker manages shadow state specifically for the sleep hygiene plugin
type SleepHygieneTracker struct {
	mu    sync.RWMutex
	state *SleepHygieneShadowState
}

// NewSleepHygieneTracker creates a new sleep hygiene shadow state tracker
func NewSleepHygieneTracker() *SleepHygieneTracker {
	return &SleepHygieneTracker{
		state: NewSleepHygieneShadowState(),
	}
}

// UpdateCurrentInputs updates the current input values
func (st *SleepHygieneTracker) UpdateCurrentInputs(inputs map[string]interface{}) {
	st.mu.Lock()
	defer st.mu.Unlock()

	for key, value := range inputs {
		st.state.Inputs.Current[key] = value
	}
	st.state.Metadata.LastUpdated = time.Now()
}

// SnapshotInputsForAction captures current inputs as the at-last-action snapshot
func (st *SleepHygieneTracker) SnapshotInputsForAction() {
	st.mu.Lock()
	defer st.mu.Unlock()

	// Deep copy current inputs to at-last-action
	st.state.Inputs.AtLastAction = make(map[string]interface{})
	for key, value := range st.state.Inputs.Current {
		st.state.Inputs.AtLastAction[key] = value
	}
}

// RecordAction records a sleep hygiene action
func (st *SleepHygieneTracker) RecordAction(actionType string, reason string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	st.state.Outputs.LastActionTime = now
	st.state.Outputs.LastActionType = actionType
	st.state.Outputs.LastActionReason = reason
	st.state.Metadata.LastUpdated = now
}

// UpdateWakeSequenceStatus updates the wake sequence status
func (st *SleepHygieneTracker) UpdateWakeSequenceStatus(status string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.state.Outputs.WakeSequenceStatus = status
	st.state.Metadata.LastUpdated = time.Now()
}

// RecordFadeOutStart records the start of a speaker fade-out
func (st *SleepHygieneTracker) RecordFadeOutStart(speakerEntityID string, startVolume int) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	st.state.Outputs.FadeOutProgress[speakerEntityID] = SpeakerFadeOut{
		SpeakerEntityID: speakerEntityID,
		CurrentVolume:   startVolume,
		StartVolume:     startVolume,
		IsActive:        true,
		StartTime:       now,
		LastUpdate:      now,
	}
	st.state.Metadata.LastUpdated = now
}

// UpdateFadeOutProgress updates the fade-out progress for a speaker
func (st *SleepHygieneTracker) UpdateFadeOutProgress(speakerEntityID string, currentVolume int) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if fadeOut, exists := st.state.Outputs.FadeOutProgress[speakerEntityID]; exists {
		fadeOut.CurrentVolume = currentVolume
		fadeOut.LastUpdate = time.Now()
		if currentVolume == 0 {
			fadeOut.IsActive = false
		}
		st.state.Outputs.FadeOutProgress[speakerEntityID] = fadeOut
	}
	st.state.Metadata.LastUpdated = time.Now()
}

// ClearFadeOutProgress clears all fade-out progress
func (st *SleepHygieneTracker) ClearFadeOutProgress() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.state.Outputs.FadeOutProgress = make(map[string]SpeakerFadeOut)
	st.state.Metadata.LastUpdated = time.Now()
}

// RecordTTSAnnouncement records a TTS announcement
func (st *SleepHygieneTracker) RecordTTSAnnouncement(message string, speaker string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.state.Outputs.LastTTSAnnouncement = &TTSAnnouncement{
		Message:   message,
		Speaker:   speaker,
		Timestamp: time.Now(),
	}
	st.state.Metadata.LastUpdated = time.Now()
}

// RecordStopScreensReminder records a stop screens reminder trigger
func (st *SleepHygieneTracker) RecordStopScreensReminder() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.state.Outputs.StopScreensReminder = &ReminderTrigger{
		Triggered: true,
		Timestamp: time.Now(),
	}
	st.state.Metadata.LastUpdated = time.Now()
}

// RecordGoToBedReminder records a go to bed reminder trigger
func (st *SleepHygieneTracker) RecordGoToBedReminder() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.state.Outputs.GoToBedReminder = &ReminderTrigger{
		Triggered: true,
		Timestamp: time.Now(),
	}
	st.state.Metadata.LastUpdated = time.Now()
}

// GetState returns the current shadow state (thread-safe copy)
func (st *SleepHygieneTracker) GetState() *SleepHygieneShadowState {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	stateCopy := &SleepHygieneShadowState{
		Plugin: st.state.Plugin,
		Inputs: SleepHygieneInputs{
			Current:      make(map[string]interface{}),
			AtLastAction: make(map[string]interface{}),
		},
		Outputs: SleepHygieneOutputs{
			WakeSequenceStatus: st.state.Outputs.WakeSequenceStatus,
			FadeOutProgress:    make(map[string]SpeakerFadeOut),
			LastActionTime:     st.state.Outputs.LastActionTime,
			LastActionType:     st.state.Outputs.LastActionType,
			LastActionReason:   st.state.Outputs.LastActionReason,
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

	// Copy fade out progress
	for k, v := range st.state.Outputs.FadeOutProgress {
		stateCopy.Outputs.FadeOutProgress[k] = v
	}

	// Copy TTS announcement if it exists
	if st.state.Outputs.LastTTSAnnouncement != nil {
		announcement := *st.state.Outputs.LastTTSAnnouncement
		stateCopy.Outputs.LastTTSAnnouncement = &announcement
	}

	// Copy stop screens reminder if it exists
	if st.state.Outputs.StopScreensReminder != nil {
		reminder := *st.state.Outputs.StopScreensReminder
		stateCopy.Outputs.StopScreensReminder = &reminder
	}

	// Copy go to bed reminder if it exists
	if st.state.Outputs.GoToBedReminder != nil {
		reminder := *st.state.Outputs.GoToBedReminder
		stateCopy.Outputs.GoToBedReminder = &reminder
	}

	return stateCopy
}
