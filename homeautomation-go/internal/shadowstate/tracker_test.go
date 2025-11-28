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

// Security Tracker Tests

func TestNewSecurityTracker(t *testing.T) {
	st := NewSecurityTracker()
	if st == nil {
		t.Fatal("NewSecurityTracker returned nil")
	}
	if st.state == nil {
		t.Error("state not initialized")
	}
}

func TestSecurityTrackerUpdateCurrentInputs(t *testing.T) {
	st := NewSecurityTracker()

	inputs := map[string]interface{}{
		"isEveryoneAsleep":       true,
		"isAnyoneHome":           false,
		"isExpectingSomeone":     false,
		"didOwnerJustReturnHome": false,
	}

	st.UpdateCurrentInputs(inputs)

	state := st.GetState()
	if state.Inputs.Current["isEveryoneAsleep"] != true {
		t.Errorf("Expected isEveryoneAsleep to be true, got %v", state.Inputs.Current["isEveryoneAsleep"])
	}
	if state.Inputs.Current["isAnyoneHome"] != false {
		t.Errorf("Expected isAnyoneHome to be false, got %v", state.Inputs.Current["isAnyoneHome"])
	}
}

func TestSecurityTrackerSnapshotInputsForAction(t *testing.T) {
	st := NewSecurityTracker()

	// Set initial inputs
	inputs := map[string]interface{}{
		"isEveryoneAsleep": false,
	}
	st.UpdateCurrentInputs(inputs)

	// Snapshot
	st.SnapshotInputsForAction()

	// Change current inputs
	newInputs := map[string]interface{}{
		"isEveryoneAsleep": true,
	}
	st.UpdateCurrentInputs(newInputs)

	state := st.GetState()

	// Current should be true
	if state.Inputs.Current["isEveryoneAsleep"] != true {
		t.Errorf("Expected current isEveryoneAsleep to be true, got %v", state.Inputs.Current["isEveryoneAsleep"])
	}

	// At last action should be false
	if state.Inputs.AtLastAction["isEveryoneAsleep"] != false {
		t.Errorf("Expected atLastAction isEveryoneAsleep to be false, got %v", state.Inputs.AtLastAction["isEveryoneAsleep"])
	}
}

func TestSecurityTrackerRecordLockdownAction(t *testing.T) {
	st := NewSecurityTracker()

	st.RecordLockdownAction(true, "Everyone is asleep")

	state := st.GetState()

	if !state.Outputs.Lockdown.Active {
		t.Error("Expected lockdown to be active")
	}
	if state.Outputs.Lockdown.Reason != "Everyone is asleep" {
		t.Errorf("Expected reason 'Everyone is asleep', got %s", state.Outputs.Lockdown.Reason)
	}
	if state.Outputs.Lockdown.ActivatedAt.IsZero() {
		t.Error("Expected ActivatedAt to be set")
	}
	if state.Outputs.Lockdown.WillResetAt.IsZero() {
		t.Error("Expected WillResetAt to be set")
	}

	// Test deactivation
	st.RecordLockdownAction(false, "Auto-reset")

	state = st.GetState()
	if state.Outputs.Lockdown.Active {
		t.Error("Expected lockdown to be inactive after reset")
	}
	if state.Outputs.Lockdown.ActivatedAt != (time.Time{}) {
		t.Error("Expected ActivatedAt to be cleared")
	}
}

func TestSecurityTrackerRecordDoorbellEvent(t *testing.T) {
	st := NewSecurityTracker()

	st.RecordDoorbellEvent(false, true, true)

	state := st.GetState()

	if state.Outputs.LastDoorbell == nil {
		t.Fatal("Expected LastDoorbell to be set")
	}
	if state.Outputs.LastDoorbell.RateLimited {
		t.Error("Expected RateLimited to be false")
	}
	if !state.Outputs.LastDoorbell.TTSSent {
		t.Error("Expected TTSSent to be true")
	}
	if !state.Outputs.LastDoorbell.LightsFlashed {
		t.Error("Expected LightsFlashed to be true")
	}
	if state.Outputs.LastDoorbell.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}

	// Test rate-limited event
	st.RecordDoorbellEvent(true, false, false)

	state = st.GetState()
	if !state.Outputs.LastDoorbell.RateLimited {
		t.Error("Expected RateLimited to be true")
	}
	if state.Outputs.LastDoorbell.TTSSent {
		t.Error("Expected TTSSent to be false for rate-limited event")
	}
}

func TestSecurityTrackerRecordVehicleArrivalEvent(t *testing.T) {
	st := NewSecurityTracker()

	st.RecordVehicleArrivalEvent(false, true, true)

	state := st.GetState()

	if state.Outputs.LastVehicle == nil {
		t.Fatal("Expected LastVehicle to be set")
	}
	if state.Outputs.LastVehicle.RateLimited {
		t.Error("Expected RateLimited to be false")
	}
	if !state.Outputs.LastVehicle.TTSSent {
		t.Error("Expected TTSSent to be true")
	}
	if !state.Outputs.LastVehicle.WasExpecting {
		t.Error("Expected WasExpecting to be true")
	}
	if state.Outputs.LastVehicle.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}

func TestSecurityTrackerRecordGarageOpenEvent(t *testing.T) {
	st := NewSecurityTracker()

	st.RecordGarageOpenEvent("Owner returned home", true)

	state := st.GetState()

	if state.Outputs.LastGarageOpen == nil {
		t.Fatal("Expected LastGarageOpen to be set")
	}
	if state.Outputs.LastGarageOpen.Reason != "Owner returned home" {
		t.Errorf("Expected reason 'Owner returned home', got %s", state.Outputs.LastGarageOpen.Reason)
	}
	if !state.Outputs.LastGarageOpen.GarageWasEmpty {
		t.Error("Expected GarageWasEmpty to be true")
	}
	if state.Outputs.LastGarageOpen.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}
}

func TestSecurityTrackerGetStateReturnsDeepCopy(t *testing.T) {
	st := NewSecurityTracker()

	// Set initial state
	inputs := map[string]interface{}{
		"isEveryoneAsleep": false,
	}
	st.UpdateCurrentInputs(inputs)

	// Get state
	state1 := st.GetState()

	// Modify the returned state
	state1.Inputs.Current["isEveryoneAsleep"] = true

	// Get state again
	state2 := st.GetState()

	// Original should be unchanged
	if state2.Inputs.Current["isEveryoneAsleep"] != false {
		t.Error("Modifying returned state affected the internal state")
	}
}

func TestSecurityTrackerConcurrentAccess(t *testing.T) {
	st := NewSecurityTracker()

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				inputs := map[string]interface{}{
					"isEveryoneAsleep": i%2 == 0,
					"count":            i*20 + j,
				}
				st.UpdateCurrentInputs(inputs)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = st.GetState()
			}
		}()
	}

	// Concurrent snapshots
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				st.SnapshotInputsForAction()
			}
		}()
	}

	// Concurrent lockdown actions
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				st.RecordLockdownAction(i%2 == 0, "test")
			}
		}(i)
	}

	// Concurrent doorbell events
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				st.RecordDoorbellEvent(false, true, true)
			}
		}()
	}

	wg.Wait()
}

func TestSecurityTrackerMetadataUpdates(t *testing.T) {
	st := NewSecurityTracker()

	initialMetadata := st.GetState().Metadata

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Update inputs
	st.UpdateCurrentInputs(map[string]interface{}{"test": "value"})

	updatedMetadata := st.GetState().Metadata

	if !updatedMetadata.LastUpdated.After(initialMetadata.LastUpdated) {
		t.Error("Expected LastUpdated to be updated after UpdateCurrentInputs")
	}

	// Wait a bit more
	time.Sleep(10 * time.Millisecond)

	// Record action
	st.RecordLockdownAction(true, "test")

	actionMetadata := st.GetState().Metadata

	if !actionMetadata.LastUpdated.After(updatedMetadata.LastUpdated) {
		t.Error("Expected LastUpdated to be updated after RecordLockdownAction")
	}
}

func TestSecurityShadowStateImplementsInterface(t *testing.T) {
	var _ PluginShadowState = (*SecurityShadowState)(nil)
}

func TestSecurityTrackerLastActionTime(t *testing.T) {
	st := NewSecurityTracker()

	// Initially should be zero
	state := st.GetState()
	if !state.Outputs.LastActionTime.IsZero() {
		t.Error("Expected LastActionTime to be zero initially")
	}

	// Record an action
	st.RecordLockdownAction(true, "test")

	state = st.GetState()
	if state.Outputs.LastActionTime.IsZero() {
		t.Error("Expected LastActionTime to be set after action")
	}

	firstTime := state.Outputs.LastActionTime

	// Wait and record another action
	time.Sleep(10 * time.Millisecond)
	st.RecordDoorbellEvent(false, true, true)

	state = st.GetState()
	if !state.Outputs.LastActionTime.After(firstTime) {
		t.Error("Expected LastActionTime to be updated after second action")
	}
}
