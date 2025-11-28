package shadowstate

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker()
	if tracker == nil {
		t.Fatal("NewTracker returned nil")
	}
	if tracker.pluginStates == nil {
		t.Error("pluginStates map not initialized")
	}
	if tracker.stateProviders == nil {
		t.Error("stateProviders map not initialized")
	}
}

func TestTrackerRegisterPlugin(t *testing.T) {
	tracker := NewTracker()
	state := NewLightingShadowState()

	tracker.RegisterPlugin("lighting", state)

	retrieved, ok := tracker.GetPluginState("lighting")
	if !ok {
		t.Fatal("Failed to retrieve registered plugin state")
	}
	if retrieved == nil {
		t.Error("Retrieved state is nil")
	}
}

func TestTrackerRegisterPluginProvider(t *testing.T) {
	tracker := NewTracker()
	callCount := 0

	provider := func() PluginShadowState {
		callCount++
		return NewLightingShadowState()
	}

	tracker.RegisterPluginProvider("lighting", provider)

	// First call
	state1, ok := tracker.GetPluginState("lighting")
	if !ok {
		t.Fatal("Failed to retrieve state from provider")
	}
	if state1 == nil {
		t.Error("Retrieved state is nil")
	}
	if callCount != 1 {
		t.Errorf("Expected provider to be called once, was called %d times", callCount)
	}

	// Second call should call provider again
	state2, ok := tracker.GetPluginState("lighting")
	if !ok {
		t.Fatal("Failed to retrieve state from provider on second call")
	}
	if state2 == nil {
		t.Error("Retrieved state is nil on second call")
	}
	if callCount != 2 {
		t.Errorf("Expected provider to be called twice, was called %d times", callCount)
	}
}

func TestTrackerProviderTakesPrecedence(t *testing.T) {
	tracker := NewTracker()

	// Register static state
	staticState := NewLightingShadowState()
	staticState.Inputs.Current["test"] = "static"
	tracker.RegisterPlugin("lighting", staticState)

	// Register provider for same plugin
	tracker.RegisterPluginProvider("lighting", func() PluginShadowState {
		providerState := NewLightingShadowState()
		providerState.Inputs.Current["test"] = "provider"
		return providerState
	})

	// Provider should take precedence
	state, ok := tracker.GetPluginState("lighting")
	if !ok {
		t.Fatal("Failed to retrieve state")
	}

	inputs := state.GetCurrentInputs()
	if inputs["test"] != "provider" {
		t.Errorf("Expected provider state, got %v", inputs["test"])
	}
}

func TestTrackerGetPluginStateNotFound(t *testing.T) {
	tracker := NewTracker()

	_, ok := tracker.GetPluginState("nonexistent")
	if ok {
		t.Error("Expected GetPluginState to return false for nonexistent plugin")
	}
}

func TestTrackerGetAllPluginStates(t *testing.T) {
	tracker := NewTracker()

	// Register multiple plugins
	tracker.RegisterPlugin("plugin1", NewLightingShadowState())
	tracker.RegisterPlugin("plugin2", NewLightingShadowState())
	tracker.RegisterPluginProvider("plugin3", func() PluginShadowState {
		return NewLightingShadowState()
	})

	states := tracker.GetAllPluginStates()

	if len(states) != 3 {
		t.Errorf("Expected 3 plugin states, got %d", len(states))
	}

	for _, name := range []string{"plugin1", "plugin2", "plugin3"} {
		if _, ok := states[name]; !ok {
			t.Errorf("Expected to find %s in all states", name)
		}
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	tracker := NewTracker()

	// Register a provider
	tracker.RegisterPluginProvider("lighting", func() PluginShadowState {
		return NewLightingShadowState()
	})

	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, ok := tracker.GetPluginState("lighting")
				if !ok {
					t.Error("Failed to get state during concurrent access")
				}
			}
		}()
	}

	wg.Wait()
}

func TestNewLightingTracker(t *testing.T) {
	lt := NewLightingTracker()
	if lt == nil {
		t.Fatal("NewLightingTracker returned nil")
	}
	if lt.state == nil {
		t.Error("state not initialized")
	}
}

func TestLightingTrackerUpdateCurrentInputs(t *testing.T) {
	lt := NewLightingTracker()

	inputs := map[string]interface{}{
		"dayPhase":     "evening",
		"sunevent":     "sunset",
		"isAnyoneHome": true,
	}

	lt.UpdateCurrentInputs(inputs)

	state := lt.GetState()
	if state.Inputs.Current["dayPhase"] != "evening" {
		t.Errorf("Expected dayPhase to be 'evening', got %v", state.Inputs.Current["dayPhase"])
	}
	if state.Inputs.Current["sunevent"] != "sunset" {
		t.Errorf("Expected sunevent to be 'sunset', got %v", state.Inputs.Current["sunevent"])
	}
	if state.Inputs.Current["isAnyoneHome"] != true {
		t.Errorf("Expected isAnyoneHome to be true, got %v", state.Inputs.Current["isAnyoneHome"])
	}
}

func TestLightingTrackerSnapshotInputsForAction(t *testing.T) {
	lt := NewLightingTracker()

	// Set initial inputs
	inputs := map[string]interface{}{
		"dayPhase": "afternoon",
	}
	lt.UpdateCurrentInputs(inputs)

	// Snapshot
	lt.SnapshotInputsForAction()

	// Change current inputs
	newInputs := map[string]interface{}{
		"dayPhase": "evening",
	}
	lt.UpdateCurrentInputs(newInputs)

	state := lt.GetState()

	// Current should be evening
	if state.Inputs.Current["dayPhase"] != "evening" {
		t.Errorf("Expected current dayPhase to be 'evening', got %v", state.Inputs.Current["dayPhase"])
	}

	// At last action should be afternoon
	if state.Inputs.AtLastAction["dayPhase"] != "afternoon" {
		t.Errorf("Expected atLastAction dayPhase to be 'afternoon', got %v", state.Inputs.AtLastAction["dayPhase"])
	}
}

func TestLightingTrackerRecordRoomAction(t *testing.T) {
	lt := NewLightingTracker()

	lt.RecordRoomAction("Living Room", "activate_scene", "dayPhase changed", "evening", false)

	state := lt.GetState()

	room, ok := state.Outputs.Rooms["Living Room"]
	if !ok {
		t.Fatal("Room 'Living Room' not found in outputs")
	}

	if room.ActionType != "activate_scene" {
		t.Errorf("Expected action type 'activate_scene', got %s", room.ActionType)
	}
	if room.Reason != "dayPhase changed" {
		t.Errorf("Expected reason 'dayPhase changed', got %s", room.Reason)
	}
	if room.ActiveScene != "evening" {
		t.Errorf("Expected active scene 'evening', got %s", room.ActiveScene)
	}
	if room.TurnedOff {
		t.Error("Expected TurnedOff to be false")
	}
}

func TestLightingTrackerRecordTurnOff(t *testing.T) {
	lt := NewLightingTracker()

	lt.RecordRoomAction("Kitchen", "turn_off", "No one home", "", true)

	state := lt.GetState()

	room, ok := state.Outputs.Rooms["Kitchen"]
	if !ok {
		t.Fatal("Room 'Kitchen' not found in outputs")
	}

	if room.ActionType != "turn_off" {
		t.Errorf("Expected action type 'turn_off', got %s", room.ActionType)
	}
	if !room.TurnedOff {
		t.Error("Expected TurnedOff to be true")
	}
	if room.ActiveScene != "" {
		t.Errorf("Expected active scene to be empty, got %s", room.ActiveScene)
	}
}

func TestLightingTrackerGetStateReturnsDeepCopy(t *testing.T) {
	lt := NewLightingTracker()

	// Set initial state
	inputs := map[string]interface{}{
		"dayPhase": "morning",
	}
	lt.UpdateCurrentInputs(inputs)

	// Get state
	state1 := lt.GetState()

	// Modify the returned state
	state1.Inputs.Current["dayPhase"] = "modified"

	// Get state again
	state2 := lt.GetState()

	// Original should be unchanged
	if state2.Inputs.Current["dayPhase"] != "morning" {
		t.Error("Modifying returned state affected the internal state")
	}
}

func TestLightingTrackerConcurrentAccess(t *testing.T) {
	lt := NewLightingTracker()

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				inputs := map[string]interface{}{
					"dayPhase": "test",
					"count":    i*20 + j,
				}
				lt.UpdateCurrentInputs(inputs)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = lt.GetState()
			}
		}()
	}

	// Concurrent snapshots
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				lt.SnapshotInputsForAction()
			}
		}()
	}

	// Concurrent room actions
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				roomName := fmt.Sprintf("Room%d", i)
				lt.RecordRoomAction(roomName, "activate_scene", "test", "evening", false)
			}
		}(i)
	}

	wg.Wait()
}

func TestLightingTrackerMetadataUpdates(t *testing.T) {
	lt := NewLightingTracker()

	initialMetadata := lt.GetState().Metadata

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Update inputs
	lt.UpdateCurrentInputs(map[string]interface{}{"test": "value"})

	updatedMetadata := lt.GetState().Metadata

	if !updatedMetadata.LastUpdated.After(initialMetadata.LastUpdated) {
		t.Error("Expected LastUpdated to be updated after UpdateCurrentInputs")
	}

	// Wait a bit more
	time.Sleep(10 * time.Millisecond)

	// Record action
	lt.RecordRoomAction("Test Room", "test", "test", "test", false)

	actionMetadata := lt.GetState().Metadata

	if !actionMetadata.LastUpdated.After(updatedMetadata.LastUpdated) {
		t.Error("Expected LastUpdated to be updated after RecordRoomAction")
	}
}

func TestLightingShadowStateImplementsInterface(t *testing.T) {
	var _ PluginShadowState = (*LightingShadowState)(nil)
}
