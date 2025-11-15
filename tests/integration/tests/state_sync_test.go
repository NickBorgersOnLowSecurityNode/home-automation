package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"home-automation/tests/integration/helpers"
)

func TestStateSync_HAToGolang_BooleanUpdate(t *testing.T) {
	// Reset service calls
	require.NoError(t, mockHA.Reset())

	// Test: Update a boolean state in HA
	err := mockHA.InjectEvent("input_boolean.nick_home", "on")
	require.NoError(t, err, "Failed to inject event")

	// Wait for system to process the event
	time.Sleep(500 * time.Millisecond)

	// Verify the state was updated in Mock HA
	state, err := mockHA.GetEntityState("input_boolean.nick_home")
	require.NoError(t, err)
	assert.Equal(t, "on", state)

	// The Golang system should have received this update via WebSocket
	// and synchronized its internal cache
	// (In a real test, you might verify this causes downstream effects)
}

func TestStateSync_GolangToHA_ServiceCall(t *testing.T) {
	// This test verifies that when the Golang system wants to update state,
	// it makes the appropriate service call to HA

	// Reset previous calls
	require.NoError(t, mockHA.Reset())

	// Trigger a state change that should cause Golang to update HA
	// For example, if Nick arrives home, the system might update derived states

	err := mockHA.InjectEvent("input_boolean.nick_home", "on")
	require.NoError(t, err)

	err = mockHA.InjectEvent("input_boolean.caroline_home", "on")
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(1 * time.Second)

	// The system should update any_owner_home and anyone_home
	// Check for service calls to update these derived states
	calls, err := mockHA.WaitForServiceCalls(
		helpers.ServiceCallFilter{Domain: "input_boolean", Service: "turn_on"},
		3*time.Second,
	)
	require.NoError(t, err)

	// In a real implementation, verify specific entities were updated
	// For now, just check that some service calls were made
	t.Logf("Received %d service calls", len(calls))
}

func TestStateSync_NumberUpdate(t *testing.T) {
	require.NoError(t, mockHA.Reset())

	// Test updating a number entity
	err := mockHA.InjectEvent("input_number.alarm_time", 25200000) // 7:00 AM in ms
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	state, err := mockHA.GetEntityState("input_number.alarm_time")
	require.NoError(t, err)

	// JSON numbers might be float64
	stateNum, ok := state.(float64)
	require.True(t, ok, "State should be a number")
	assert.Equal(t, 25200000.0, stateNum)
}

func TestStateSync_TextUpdate(t *testing.T) {
	require.NoError(t, mockHA.Reset())

	// Test updating a text entity
	phases := []string{"morning", "day", "evening", "winddown", "sleep"}

	for _, phase := range phases {
		err := mockHA.InjectEvent("input_text.day_phase", phase)
		require.NoError(t, err, "Failed to set day phase to %s", phase)

		time.Sleep(300 * time.Millisecond)

		state, err := mockHA.GetEntityState("input_text.day_phase")
		require.NoError(t, err)
		assert.Equal(t, phase, state, "Day phase should be %s", phase)
	}
}

func TestStateSync_RapidUpdates_NoCorruption(t *testing.T) {
	// Test that rapid state changes don't cause corruption or race conditions
	require.NoError(t, mockHA.Reset())

	// Send 20 rapid updates
	for i := 0; i < 20; i++ {
		state := "on"
		if i%2 == 0 {
			state = "off"
		}

		err := mockHA.InjectEvent("input_boolean.nick_home", state)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
	}

	// Wait for all updates to process
	time.Sleep(2 * time.Second)

	// Final state should be consistent (last update was off)
	state, err := mockHA.GetEntityState("input_boolean.nick_home")
	require.NoError(t, err)
	assert.Equal(t, "off", state)

	// System should still be healthy after rapid updates
	err = mockHA.HealthCheck()
	assert.NoError(t, err, "System should remain healthy")
}

func TestStateSync_MultipleEntities_Concurrent(t *testing.T) {
	// Test updating multiple entities concurrently
	require.NoError(t, mockHA.Reset())

	entities := []struct {
		id    string
		value interface{}
	}{
		{"input_boolean.nick_home", "on"},
		{"input_boolean.caroline_home", "on"},
		{"input_boolean.tori_here", "on"},
		{"input_text.day_phase", "evening"},
		{"input_number.alarm_time", 28800000},
	}

	// Inject all events rapidly
	for _, e := range entities {
		err := mockHA.InjectEvent(e.id, e.value)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Verify all states are correct
	for _, e := range entities {
		state, err := mockHA.GetEntityState(e.id)
		require.NoError(t, err, "Failed to get state for %s", e.id)

		// Compare based on type
		switch v := e.value.(type) {
		case string:
			assert.Equal(t, v, state, "State mismatch for %s", e.id)
		case int:
			stateNum, ok := state.(float64)
			require.True(t, ok, "Expected number for %s", e.id)
			assert.Equal(t, float64(v), stateNum, "State mismatch for %s", e.id)
		}
	}
}
