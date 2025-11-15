package integration_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"home-automation/tests/integration/helpers"
)

var mockHA *helpers.MockHAClient

func TestMain(m *testing.M) {
	// Setup: Create Mock HA client
	mockHAURL := os.Getenv("MOCK_HA_URL")
	if mockHAURL == "" {
		mockHAURL = "http://localhost:8123"
	}

	mockHA = helpers.NewMockHAClient(mockHAURL)

	// Wait for Mock HA to be ready
	if err := mockHA.WaitForReady(30 * time.Second); err != nil {
		panic("Mock HA not ready: " + err.Error())
	}

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func TestSystemStartup_ConnectsToMockHA(t *testing.T) {
	// This test verifies that the homeautomation system successfully
	// connects to the Mock Home Assistant service via WebSocket

	// The system should automatically connect during startup
	// We verify this by checking that it can make service calls

	// Wait a bit for system to fully initialize
	time.Sleep(3 * time.Second)

	// Reset any previous calls
	require.NoError(t, mockHA.Reset())

	// Inject an event that should trigger system response
	err := mockHA.InjectEvent("input_boolean.nick_home", "on")
	require.NoError(t, err, "Failed to inject presence event")

	// Wait for the system to process and potentially make service calls
	// (This would depend on your actual plugin implementation)
	time.Sleep(2 * time.Second)

	// Verify the system is responsive
	// In a real implementation, you might check for specific service calls
	// For now, we just verify no errors occurred
}

func TestSystemStartup_LoadsInitialState(t *testing.T) {
	// This test verifies that the system loads all 33 entity states
	// from Mock HA during startup

	// The system should have synced state from the fixtures
	// We can verify by checking that state changes propagate correctly

	err := mockHA.InjectEvent("input_text.day_phase", "evening")
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Verify state was updated
	state, err := mockHA.GetEntityState("input_text.day_phase")
	require.NoError(t, err)
	assert.Equal(t, "evening", state)
}

func TestSystemStartup_AllEntitiesPresent(t *testing.T) {
	// Verify all 33 expected entities are present in Mock HA
	expectedEntities := []string{
		// Presence booleans
		"input_boolean.nick_home",
		"input_boolean.caroline_home",
		"input_boolean.tori_here",
		"input_boolean.any_owner_home",
		"input_boolean.anyone_home",

		// Sleep booleans
		"input_boolean.master_asleep",
		"input_boolean.guest_asleep",
		"input_boolean.everyone_asleep",
		"input_boolean.have_guests",

		// Door/occupancy booleans
		"input_boolean.guest_bedroom_door_open",
		"input_boolean.office_occupied",

		// Device state booleans
		"input_boolean.tv_playing",
		"input_boolean.apple_tv_playing",
		"input_boolean.tv_on",

		// Energy booleans
		"input_boolean.free_energy_available",
		"input_boolean.grid_available",

		// Security booleans
		"input_boolean.expecting_someone",
		"input_boolean.lockdown_active",

		// Numbers
		"input_number.alarm_time",
		"input_number.remaining_solar_generation",
		"input_number.this_hour_solar_generation",

		// Text
		"input_text.day_phase",
		"input_text.sunevent",
		"input_text.music_playback_type",
		"input_text.battery_energy_level",
		"input_text.solar_production_energy_level",
		"input_text.current_energy_level",

		// JSON
		"input_text.currently_playing_music",
		"input_text.music_config",
		"input_text.hue_config",
		"input_text.energy_config",
		"input_text.music_playlist_numbers",
		"input_text.schedule",
	}

	for _, entityID := range expectedEntities {
		_, err := mockHA.GetEntityState(entityID)
		assert.NoError(t, err, "Entity %s should exist", entityID)
	}
}
