package integration

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"homeautomation/internal/plugins/lighting"
	"homeautomation/internal/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// Lighting Control Plugin - Trigger Scenario Tests
//
// These tests verify that the lighting manager correctly subscribes to and
// responds to changes in state variables used in room configurations.
//
// Variables tested:
// - didOwnerJustReturnHome (local-only) - Used by Front of House
// - isAnyoneHomeAndAwake (computed) - Used by Living Room and Sitting Room
//
// Reference: Node-RED Lighting Control flow (16cd74edb3f2c03d)
// ============================================================================

// setupLightingTriggersTest creates a test environment with the lighting plugin
// using the extended config that includes rooms with presence triggers
func setupLightingTriggersTest(t *testing.T) (*MockHAServer, *state.Manager, *lighting.Manager, func()) {
	server, client, manager, baseCleanup := setupTest(t)

	// Load test lighting config (includes Front of House and Sitting Room)
	configPath := filepath.Join("testdata", "hue_config_test.yaml")
	lightingConfig, err := lighting.LoadConfig(configPath)
	require.NoError(t, err, "Failed to load test lighting config")

	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create lighting plugin (NOT read-only - we want to see service calls)
	lightingMgr := lighting.NewManager(client, manager, lightingConfig, logger, false)

	// Start the lighting plugin
	err = lightingMgr.Start()
	require.NoError(t, err, "Failed to start lighting manager")

	cleanup := func() {
		lightingMgr.Stop()
		baseCleanup()
	}

	return server, manager, lightingMgr, cleanup
}

// ============================================================================
// Tests for didOwnerJustReturnHome subscription
//
// Node-RED Behavior:
// - The "Did Owner Just Return Home?" get-shared-state node triggers on change
// - When didOwnerJustReturnHome becomes true, "Front of House" lights turn on
// - When didOwnerJustReturnHome becomes false, lights may turn off
// ============================================================================

// TestScenario_OwnerJustReturnedHome_ShouldActivateFrontOfHouseScene tests that
// when didOwnerJustReturnHome becomes true, the Front of House lights should
// activate their scene.
func TestScenario_OwnerJustReturnedHome_ShouldActivateFrontOfHouseScene(t *testing.T) {
	server, stateManager, _, cleanup := setupLightingTriggersTest(t)
	defer cleanup()

	// GIVEN: Evening, owner is not home, didOwnerJustReturnHome is false
	t.Log("GIVEN: Evening, owner is not home, didOwnerJustReturnHome is false")
	server.SetState("input_text.day_phase", "evening", map[string]interface{}{})
	server.SetState("input_boolean.anyone_home", "off", map[string]interface{}{})
	server.SetState("input_boolean.have_guests", "off", map[string]interface{}{})
	server.SetState("input_boolean.everyone_asleep", "off", map[string]interface{}{})
	time.Sleep(300 * time.Millisecond)

	// Clear service calls from initialization
	server.ClearServiceCalls()

	// WHEN: Owner returns home (didOwnerJustReturnHome becomes true)
	// Since didOwnerJustReturnHome is a local-only variable, we set it directly
	// through the state manager
	t.Log("WHEN: Owner returns home (didOwnerJustReturnHome becomes true)")
	err := stateManager.SetBool("didOwnerJustReturnHome", true)
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// THEN: Front of House scene should be activated
	calls := server.GetServiceCalls()
	sceneActivations := filterServiceCalls(calls, "scene", "turn_on")
	t.Logf("Scene activations after didOwnerJustReturnHome=true: %d", len(sceneActivations))

	// Find Front of House scene activation
	foundFrontOfHouse := false
	for _, call := range sceneActivations {
		if entityID, ok := call.ServiceData["entity_id"].(string); ok {
			t.Logf("  Scene activated: %s", entityID)
			if strings.Contains(entityID, "front_of_house") {
				foundFrontOfHouse = true
			}
		}
	}

	assert.True(t, foundFrontOfHouse,
		"Front of House scene should activate when didOwnerJustReturnHome becomes true")
}

// TestScenario_OwnerJustReturnedHome_ThenLeft_ShouldTurnOffFrontOfHouse tests
// the off_if_false: didOwnerJustReturnHome behavior
func TestScenario_OwnerJustReturnedHome_ThenLeft_ShouldTurnOffFrontOfHouse(t *testing.T) {
	server, stateManager, _, cleanup := setupLightingTriggersTest(t)
	defer cleanup()

	// GIVEN: Evening, owner just returned (didOwnerJustReturnHome is true)
	t.Log("GIVEN: Evening, didOwnerJustReturnHome is true, Front of House lights are on")
	server.SetState("input_text.day_phase", "evening", map[string]interface{}{})
	server.SetState("input_boolean.have_guests", "off", map[string]interface{}{})
	server.SetState("input_boolean.everyone_asleep", "off", map[string]interface{}{})
	time.Sleep(200 * time.Millisecond)

	// Set didOwnerJustReturnHome to true first
	err := stateManager.SetBool("didOwnerJustReturnHome", true)
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)

	server.ClearServiceCalls()

	// WHEN: Owner "left" (didOwnerJustReturnHome becomes false)
	t.Log("WHEN: didOwnerJustReturnHome becomes false")
	err = stateManager.SetBool("didOwnerJustReturnHome", false)
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// THEN: Front of House should turn off (off_if_false: didOwnerJustReturnHome)
	calls := server.GetServiceCalls()
	lightOffCalls := filterServiceCalls(calls, "light", "turn_off")
	t.Logf("Light turn_off calls after didOwnerJustReturnHome=false: %d", len(lightOffCalls))

	// Find Front of House turn off
	foundFrontOfHouseOff := false
	for _, call := range lightOffCalls {
		if areaID, ok := call.ServiceData["area_id"].(string); ok {
			t.Logf("  Light turned off in area: %s", areaID)
			if areaID == "front_of_house" {
				foundFrontOfHouseOff = true
			}
		}
	}

	assert.True(t, foundFrontOfHouseOff,
		"Front of House lights should turn off when didOwnerJustReturnHome becomes false")
}

// ============================================================================
// Tests for isAnyoneHomeAndAwake subscription
//
// Node-RED Behavior:
// - The "Anyone Home and Awake" get-shared-state node triggers on change
// - When isAnyoneHomeAndAwake becomes true, Living Room and Sitting Room
//   lights should turn on
// - When it becomes false, lights may turn off
// ============================================================================

// TestScenario_AnyoneHomeAndAwake_ShouldActivateLivingRoomScene tests that
// when isAnyoneHomeAndAwake changes, rooms with on_if_true: isAnyoneHomeAndAwake
// should re-evaluate their lighting.
func TestScenario_AnyoneHomeAndAwake_ShouldActivateLivingRoomScene(t *testing.T) {
	server, _, _, cleanup := setupLightingTriggersTest(t)
	defer cleanup()

	// GIVEN: Evening, no one is home or they're asleep
	t.Log("GIVEN: Evening, isAnyoneHomeAndAwake is false")
	server.SetState("input_text.day_phase", "evening", map[string]interface{}{})
	server.SetState("input_boolean.anyone_home", "off", map[string]interface{}{})
	server.SetState("input_boolean.anyone_home_and_awake", "off", map[string]interface{}{})
	server.SetState("input_boolean.everyone_asleep", "off", map[string]interface{}{})
	time.Sleep(300 * time.Millisecond)

	// Clear service calls from initialization
	server.ClearServiceCalls()

	// WHEN: Someone comes home and is awake (isAnyoneHomeAndAwake becomes true)
	t.Log("WHEN: isAnyoneHomeAndAwake becomes true")
	server.SetState("input_boolean.anyone_home_and_awake", "on", map[string]interface{}{})
	time.Sleep(500 * time.Millisecond)

	// THEN: Living Room should activate scene (on_if_true: isAnyoneHomeAndAwake)
	calls := server.GetServiceCalls()
	sceneActivations := filterServiceCalls(calls, "scene", "turn_on")
	t.Logf("Scene activations after isAnyoneHomeAndAwake=true: %d", len(sceneActivations))

	// Check for Living Room and Sitting Room scene activation
	foundLivingRoom := false
	foundSittingRoom := false
	for _, call := range sceneActivations {
		if entityID, ok := call.ServiceData["entity_id"].(string); ok {
			t.Logf("  Scene activated: %s", entityID)
			if strings.Contains(entityID, "living_room") {
				foundLivingRoom = true
			}
			if strings.Contains(entityID, "sitting_room") {
				foundSittingRoom = true
			}
		}
	}

	assert.True(t, foundLivingRoom,
		"Living Room scene should activate when isAnyoneHomeAndAwake becomes true")
	assert.True(t, foundSittingRoom,
		"Sitting Room scene should activate when isAnyoneHomeAndAwake becomes true")
}

// TestScenario_AnyoneHomeAndAwake_BecomeFalse_ShouldEvaluateOffConditions tests
// that when isAnyoneHomeAndAwake becomes false (everyone left or went to sleep),
// rooms using this variable should re-evaluate their conditions.
func TestScenario_AnyoneHomeAndAwake_BecomeFalse_ShouldEvaluateOffConditions(t *testing.T) {
	server, _, _, cleanup := setupLightingTriggersTest(t)
	defer cleanup()

	// GIVEN: Evening, someone is home and awake, lights are on
	t.Log("GIVEN: Evening, isAnyoneHomeAndAwake is true, lights should be on")
	server.SetState("input_text.day_phase", "evening", map[string]interface{}{})
	server.SetState("input_boolean.anyone_home", "on", map[string]interface{}{})
	server.SetState("input_boolean.anyone_home_and_awake", "on", map[string]interface{}{})
	server.SetState("input_boolean.everyone_asleep", "off", map[string]interface{}{})
	time.Sleep(300 * time.Millisecond)

	// Clear service calls from initialization
	server.ClearServiceCalls()

	// WHEN: Everyone goes to sleep (isAnyoneHomeAndAwake becomes false)
	t.Log("WHEN: isAnyoneHomeAndAwake becomes false (everyone asleep)")
	server.SetState("input_boolean.anyone_home_and_awake", "off", map[string]interface{}{})
	time.Sleep(500 * time.Millisecond)

	// THEN: Rooms should re-evaluate - this should trigger evaluation
	calls := server.GetServiceCalls()
	t.Logf("Total service calls after isAnyoneHomeAndAwake=false: %d", len(calls))

	for _, call := range calls {
		t.Logf("  Service call: %s.%s", call.Domain, call.Service)
	}

	// Verify that lighting re-evaluated (any service call indicates subscription worked)
	assert.Greater(t, len(calls), 0,
		"Lighting should re-evaluate when isAnyoneHomeAndAwake changes")
}

// TestScenario_CompareSubscribedVsUnsubscribedTriggers provides a comparison
// test that verifies both isAnyoneHome and isAnyoneHomeAndAwake are now
// properly subscribed and trigger lighting re-evaluation.
func TestScenario_CompareSubscribedVsUnsubscribedTriggers(t *testing.T) {
	server, _, _, cleanup := setupLightingTriggersTest(t)
	defer cleanup()

	// Setup initial state
	t.Log("SETUP: Evening, all presence variables false")
	server.SetState("input_text.day_phase", "evening", map[string]interface{}{})
	server.SetState("input_boolean.anyone_home", "off", map[string]interface{}{})
	server.SetState("input_boolean.anyone_home_and_awake", "off", map[string]interface{}{})
	server.SetState("input_boolean.everyone_asleep", "off", map[string]interface{}{})
	time.Sleep(300 * time.Millisecond)

	// ===== TEST 1: isAnyoneHome =====
	server.ClearServiceCalls()
	t.Log("")
	t.Log("TEST 1: Changing isAnyoneHome")
	server.SetState("input_boolean.anyone_home", "on", map[string]interface{}{})
	time.Sleep(500 * time.Millisecond)

	subscribedCalls := server.GetServiceCalls()
	subscribedScenes := filterServiceCalls(subscribedCalls, "scene", "turn_on")
	t.Logf("  Scene activations from isAnyoneHome change: %d", len(subscribedScenes))
	for _, call := range subscribedScenes {
		if entityID, ok := call.ServiceData["entity_id"].(string); ok {
			t.Logf("    - %s", entityID)
		}
	}

	// Reset
	server.SetState("input_boolean.anyone_home", "off", map[string]interface{}{})
	time.Sleep(200 * time.Millisecond)

	// ===== TEST 2: isAnyoneHomeAndAwake =====
	server.ClearServiceCalls()
	t.Log("")
	t.Log("TEST 2: Changing isAnyoneHomeAndAwake")
	server.SetState("input_boolean.anyone_home_and_awake", "on", map[string]interface{}{})
	time.Sleep(500 * time.Millisecond)

	unsubscribedCalls := server.GetServiceCalls()
	unsubscribedScenes := filterServiceCalls(unsubscribedCalls, "scene", "turn_on")
	t.Logf("  Scene activations from isAnyoneHomeAndAwake change: %d", len(unsubscribedScenes))
	for _, call := range unsubscribedScenes {
		if entityID, ok := call.ServiceData["entity_id"].(string); ok {
			t.Logf("    - %s", entityID)
		}
	}

	// ===== COMPARISON =====
	t.Log("")
	t.Log("===== COMPARISON RESULTS =====")
	t.Logf("isAnyoneHome: %d scene activations", len(subscribedScenes))
	t.Logf("isAnyoneHomeAndAwake: %d scene activations", len(unsubscribedScenes))
	t.Log("==============================")

	// ASSERTIONS - Both should now trigger scene activations
	assert.Greater(t, len(subscribedScenes), 0,
		"isAnyoneHome changes should trigger scene activations")

	assert.Greater(t, len(unsubscribedScenes), 0,
		"isAnyoneHomeAndAwake changes should trigger scene activations (now subscribed)")
}
