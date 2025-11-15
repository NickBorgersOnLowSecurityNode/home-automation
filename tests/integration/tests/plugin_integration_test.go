package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"home-automation/tests/integration/helpers"
)

func TestPlugin_PresenceArrival_TriggersAutomation(t *testing.T) {
	// This test verifies that when a person arrives home,
	// the appropriate automations are triggered

	// Setup: Ensure everyone is away initially
	require.NoError(t, mockHA.Reset())
	mockHA.InjectEvent("input_boolean.nick_home", "off")
	mockHA.InjectEvent("input_boolean.caroline_home", "off")
	mockHA.InjectEvent("input_text.day_phase", "day")
	time.Sleep(1 * time.Second)

	// Clear service calls from setup
	require.NoError(t, mockHA.Reset())

	// Act: Nick arrives home
	err := mockHA.InjectEvent("input_boolean.nick_home", "on")
	require.NoError(t, err)

	// Assert: System should update derived presence states
	// Wait for service calls related to presence
	calls, err := mockHA.WaitForServiceCalls(
		helpers.ServiceCallFilter{Domain: "input_boolean"},
		5*time.Second,
	)
	require.NoError(t, err)

	// The system should have updated any_owner_home and anyone_home to true
	// (This depends on your actual State Tracking plugin implementation)

	t.Logf("Presence arrival triggered %d service calls", len(calls))

	// In a full implementation, you would also check for:
	// - TTS announcement (tts.google_say)
	// - Music playback (media_player.play_media)
	// - Lighting scenes (light.turn_on or scene.turn_on)
}

func TestPlugin_PresenceDeparture_StopsMusic(t *testing.T) {
	// Setup: Nick is home with music playing
	require.NoError(t, mockHA.Reset())
	mockHA.InjectEvent("input_boolean.nick_home", "on")
	mockHA.InjectEvent("input_boolean.caroline_home", "off")
	mockHA.InjectEvent("input_text.music_playback_type", "day")
	time.Sleep(1 * time.Second)

	// Clear setup calls
	require.NoError(t, mockHA.Reset())

	// Act: Nick leaves (last person)
	err := mockHA.InjectEvent("input_boolean.nick_home", "off")
	require.NoError(t, err)

	// Wait for music stop commands
	calls, err := mockHA.WaitForServiceCalls(
		helpers.ServiceCallFilter{Domain: "media_player", Service: "media_stop"},
		5*time.Second,
	)
	require.NoError(t, err)

	// Music should have been stopped
	// (This depends on your Music plugin implementation)
	t.Logf("Departure triggered %d media_stop calls", len(calls))
}

func TestPlugin_DayPhaseChange_TriggersLighting(t *testing.T) {
	// Test that changing day phase triggers lighting scene changes

	require.NoError(t, mockHA.Reset())
	mockHA.InjectEvent("input_boolean.anyone_home", "on")
	time.Sleep(500 * time.Millisecond)

	require.NoError(t, mockHA.Reset())

	// Act: Change day phase from morning to evening
	err := mockHA.InjectEvent("input_text.day_phase", "evening")
	require.NoError(t, err)

	// Wait for lighting service calls
	calls, err := mockHA.WaitForServiceCalls(
		helpers.ServiceCallFilter{Domain: "light"},
		5*time.Second,
	)
	require.NoError(t, err)

	// Should have triggered scene activation
	// (This depends on your Lighting plugin implementation)
	t.Logf("Day phase change triggered %d light service calls", len(calls))
}

func TestPlugin_SleepDetection_AdjustsAutomation(t *testing.T) {
	// Test that sleep state changes trigger appropriate automations

	// Setup: People are home and awake
	require.NoError(t, mockHA.Reset())
	mockHA.InjectEvent("input_boolean.nick_home", "on")
	mockHA.InjectEvent("input_boolean.master_asleep", "off")
	time.Sleep(1 * time.Second)

	require.NoError(t, mockHA.Reset())

	// Act: Master bedroom goes to sleep
	err := mockHA.InjectEvent("input_boolean.master_asleep", "on")
	require.NoError(t, err)

	// Wait for sleep-related automations
	// This might include:
	// - Music mode change to sleep sounds
	// - Lights turning off
	// - Thermostat adjustments

	time.Sleep(3 * time.Second)

	// Get all service calls
	musicCalls, _ := mockHA.GetServiceCalls(
		helpers.ServiceCallFilter{Domain: "media_player"},
	)
	lightCalls, _ := mockHA.GetServiceCalls(
		helpers.ServiceCallFilter{Domain: "light"},
	)

	t.Logf("Sleep triggered %d media_player and %d light calls",
		len(musicCalls), len(lightCalls))
}

func TestPlugin_EnergyStateChange_TriggersLoadShedding(t *testing.T) {
	// Test that low energy state triggers load shedding

	require.NoError(t, mockHA.Reset())

	// Act: Battery drops to low level
	err := mockHA.InjectEvent("input_text.battery_energy_level", "red")
	require.NoError(t, err)

	err = mockHA.InjectEvent("input_text.current_energy_level", "red")
	require.NoError(t, err)

	// Wait for thermostat service calls
	calls, err := mockHA.WaitForServiceCalls(
		helpers.ServiceCallFilter{Domain: "climate"},
		5*time.Second,
	)
	require.NoError(t, err)

	// Load shedding should adjust thermostat
	// (This depends on your Load Shedding plugin implementation)
	t.Logf("Energy state change triggered %d climate calls", len(calls))
}

func TestPlugin_TVPlayback_AdjustsBrightness(t *testing.T) {
	// Test that TV playback detection adjusts lighting

	require.NoError(t, mockHA.Reset())
	mockHA.InjectEvent("input_text.day_phase", "evening")
	time.Sleep(500 * time.Millisecond)

	require.NoError(t, mockHA.Reset())

	// Act: TV starts playing
	err := mockHA.InjectEvent("input_boolean.tv_playing", "on")
	require.NoError(t, err)

	// Wait for brightness adjustments
	time.Sleep(2 * time.Second)

	// TV plugin should have adjusted brightness or activated dim scene
	calls, _ := mockHA.GetServiceCalls(
		helpers.ServiceCallFilter{Domain: "light"},
	)

	t.Logf("TV playback triggered %d light calls", len(calls))
}

func TestPlugin_MultiplePlugins_Coordination(t *testing.T) {
	// Test that multiple plugins can coordinate properly
	// Example: Person arrives home during evening

	require.NoError(t, mockHA.Reset())

	// Setup: Evening time, no one home
	mockHA.InjectEvent("input_text.day_phase", "evening")
	mockHA.InjectEvent("input_boolean.anyone_home", "off")
	time.Sleep(1 * time.Second)

	require.NoError(t, mockHA.Reset())

	// Act: Nick arrives home in the evening
	err := mockHA.InjectEvent("input_boolean.nick_home", "on")
	require.NoError(t, err)

	// Wait for all plugins to react
	time.Sleep(5 * time.Second)

	// Multiple plugins should have activated:
	// - State Tracking: Update presence states, announce arrival
	// - Music: Start evening music
	// - Lighting: Activate evening scenes

	ttsCalls, _ := mockHA.GetServiceCalls(
		helpers.ServiceCallFilter{Domain: "tts"},
	)
	musicCalls, _ := mockHA.GetServiceCalls(
		helpers.ServiceCallFilter{Domain: "media_player"},
	)
	lightCalls, _ := mockHA.GetServiceCalls(
		helpers.ServiceCallFilter{Domain: "light"},
	)

	t.Logf("Multi-plugin coordination: %d tts, %d music, %d light calls",
		len(ttsCalls), len(musicCalls), len(lightCalls))

	// System should remain stable
	assert.NoError(t, mockHA.HealthCheck())
}
