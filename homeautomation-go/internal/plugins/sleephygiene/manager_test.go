package sleephygiene

import (
	"testing"
	"time"

	"homeautomation/internal/config"
	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// setupTest creates a test environment with mock HA client, state manager, and config loader
func setupTest(t *testing.T, currentTime time.Time) (*Manager, *ha.MockClient, *state.Manager, *config.Loader) {
	logger := zap.NewNop()
	mockHA := ha.NewMockClient()
	stateManager := state.NewManager(mockHA, logger, false)

	// Initialize state with default values
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetBool("isGuestAsleep", false)
	stateManager.SetBool("isEveryoneAsleep", true)
	stateManager.SetBool("isFadeOutInProgress", false)
	stateManager.SetBool("isNickHome", true)
	stateManager.SetBool("isCarolineHome", true)
	stateManager.SetString("musicPlaybackType", "sleep")

	// Set alarm time to 9:00 AM today
	year, month, day := currentTime.Date()
	alarmTime := time.Date(year, month, day, 9, 0, 0, 0, currentTime.Location())
	stateManager.SetNumber("alarmTime", float64(alarmTime.UnixMilli()))

	// Create a config loader
	configLoader := config.NewLoader("../../../configs", logger)

	// Create manager with fixed time provider
	timeProvider := FixedTimeProvider{FixedTime: currentTime}
	manager := NewManager(mockHA, stateManager, configLoader, logger, false, timeProvider)

	return manager, mockHA, stateManager, configLoader
}

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	mockHA := ha.NewMockClient()
	stateManager := state.NewManager(mockHA, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)

	manager := NewManager(mockHA, stateManager, configLoader, logger, false, nil)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.timeProvider == nil {
		t.Error("timeProvider should default to RealTimeProvider")
	}

	if manager.haClient != mockHA {
		t.Error("haClient not set correctly")
	}

	if manager.stateManager != stateManager {
		t.Error("stateManager not set correctly")
	}
}

func TestStartStop(t *testing.T) {
	now := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	manager, _, _, _ := setupTest(t, now)

	// Start manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Check that ticker is running
	if manager.ticker == nil {
		t.Error("Ticker should be initialized after Start()")
	}

	// Check that subscriptions exist
	if len(manager.subscriptions) == 0 {
		t.Error("Should have subscriptions after Start()")
	}

	// Stop manager
	manager.Stop()

	// Verify cleanup
	if manager.subscriptions != nil {
		t.Error("Subscriptions should be nil after Stop()")
	}
}

func TestBeginWake_AllConditionsMet(t *testing.T) {
	// Set time to 9:05 AM (5 minutes after alarm time)
	now := time.Date(2024, 1, 15, 9, 5, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Ensure all conditions are met
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetString("musicPlaybackType", "sleep")
	stateManager.SetBool("isFadeOutInProgress", false)

	// Trigger begin_wake
	manager.handleBeginWake()

	// Verify that isFadeOutInProgress was set to true
	fadeOut, _ := stateManager.GetBool("isFadeOutInProgress")
	if !fadeOut {
		t.Error("isFadeOutInProgress should be set to true after begin_wake")
	}

	// Verify at least one call to SetState (for isFadeOutInProgress)
	calls := mockHA.GetServiceCalls()
	if len(calls) < 1 {
		t.Error("Expected at least one service call to set isFadeOutInProgress")
	}
}

func TestBeginWake_NoOneHome(t *testing.T) {
	now := time.Date(2024, 1, 15, 9, 5, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Set no one home
	stateManager.SetBool("isAnyoneHome", false)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetString("musicPlaybackType", "sleep")

	// Clear mock calls
	mockHA.ClearServiceCalls()

	// Trigger begin_wake
	manager.handleBeginWake()

	// Verify that isFadeOutInProgress was NOT changed
	fadeOut, _ := stateManager.GetBool("isFadeOutInProgress")
	if fadeOut {
		t.Error("isFadeOutInProgress should not be set when no one is home")
	}
}

func TestBeginWake_MasterNotAsleep(t *testing.T) {
	now := time.Date(2024, 1, 15, 9, 5, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Set master not asleep
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", false)
	stateManager.SetString("musicPlaybackType", "sleep")

	// Clear mock calls
	mockHA.ClearServiceCalls()

	// Trigger begin_wake
	manager.handleBeginWake()

	// Verify that isFadeOutInProgress was NOT changed
	fadeOut, _ := stateManager.GetBool("isFadeOutInProgress")
	if fadeOut {
		t.Error("isFadeOutInProgress should not be set when master is not asleep")
	}
}

func TestBeginWake_NotPlayingSleepMusic(t *testing.T) {
	now := time.Date(2024, 1, 15, 9, 5, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Set music playback to something other than "sleep"
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetString("musicPlaybackType", "day")

	// Clear mock calls
	mockHA.ClearServiceCalls()

	// Trigger begin_wake
	manager.handleBeginWake()

	// Verify that isFadeOutInProgress was NOT changed
	fadeOut, _ := stateManager.GetBool("isFadeOutInProgress")
	if fadeOut {
		t.Error("isFadeOutInProgress should not be set when not playing sleep music")
	}
}

func TestWake_AllConditionsMet(t *testing.T) {
	// Set time to 9:30 AM (25 minutes after alarm time)
	now := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Ensure all conditions are met
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetBool("isFadeOutInProgress", true)
	stateManager.SetBool("isNickHome", true)
	stateManager.SetBool("isCarolineHome", true)

	// Clear previous calls
	mockHA.ClearServiceCalls()

	// Trigger wake
	manager.handleWake()

	// Verify service calls were made
	calls := mockHA.GetServiceCalls()

	// Should have:
	// - 2 calls for master bedroom lights (initial + transition)
	// - 2 calls for common area lights flash
	// - 1 call for TTS cuddle announcement
	if len(calls) < 5 {
		t.Errorf("Expected at least 5 service calls, got %d", len(calls))
	}

	// Check that light service was called for master bedroom
	foundMasterBedroom := false
	foundTTS := false

	for _, call := range calls {
		if call.Domain == "light" && call.Service == "turn_on" {
			if entityID, ok := call.Data["entity_id"].(string); ok && entityID == "light.master_bedroom" {
				foundMasterBedroom = true
			}
		}
		if call.Domain == "tts" && call.Service == "speak" {
			foundTTS = true
		}
	}

	if !foundMasterBedroom {
		t.Error("Expected light.turn_on call for master bedroom")
	}

	if !foundTTS {
		t.Error("Expected TTS call for cuddle announcement")
	}
}

func TestWake_OnlyOneOwnerHome(t *testing.T) {
	now := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Set only Nick home
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetBool("isFadeOutInProgress", true)
	stateManager.SetBool("isNickHome", true)
	stateManager.SetBool("isCarolineHome", false)

	// Clear previous calls
	mockHA.ClearServiceCalls()

	// Trigger wake
	manager.handleWake()

	// Verify service calls were made
	calls := mockHA.GetServiceCalls()

	// Should have light calls but NO TTS call
	foundTTS := false
	for _, call := range calls {
		if call.Domain == "tts" && call.Service == "speak" {
			foundTTS = true
		}
	}

	if foundTTS {
		t.Error("Should not announce cuddle when only one owner is home")
	}
}

func TestStopScreens_AllConditionsMet(t *testing.T) {
	now := time.Date(2024, 1, 15, 22, 30, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Set conditions: someone home, not everyone asleep
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isEveryoneAsleep", false)

	// Clear previous calls
	mockHA.ClearServiceCalls()

	// Trigger stop_screens
	manager.handleStopScreens()

	// Verify light flash calls were made
	calls := mockHA.GetServiceCalls()

	foundFlash := false
	for _, call := range calls {
		if call.Domain == "light" && call.Service == "turn_on" {
			if flash, ok := call.Data["flash"].(string); ok && flash == "short" {
				foundFlash = true
				break
			}
		}
	}

	if !foundFlash {
		t.Error("Expected light flash calls for stop_screens")
	}
}

func TestStopScreens_EveryoneAsleep(t *testing.T) {
	now := time.Date(2024, 1, 15, 22, 30, 0, 0, time.UTC)
	manager, mockHA, stateManager, _ := setupTest(t, now)

	// Set everyone asleep
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isEveryoneAsleep", true)

	// Clear previous calls
	mockHA.ClearServiceCalls()

	// Trigger stop_screens
	manager.handleStopScreens()

	// Verify NO calls were made
	calls := mockHA.GetServiceCalls()
	if len(calls) > 0 {
		t.Error("Should not flash lights when everyone is asleep")
	}
}

func TestReadOnlyMode(t *testing.T) {
	now := time.Date(2024, 1, 15, 9, 5, 0, 0, time.UTC)
	logger := zap.NewNop()
	mockHA := ha.NewMockClient()
	stateManager := state.NewManager(mockHA, logger, true) // READ-ONLY

	configLoader := config.NewLoader("../../../configs", logger)

	// Set alarm time
	year, month, day := now.Date()
	alarmTime := time.Date(year, month, day, 9, 0, 0, 0, now.Location())
	stateManager.SetNumber("alarmTime", float64(alarmTime.UnixMilli()))

	// Set conditions
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetString("musicPlaybackType", "sleep")

	// Create manager in READ-ONLY mode
	timeProvider := FixedTimeProvider{FixedTime: now}
	manager := NewManager(mockHA, stateManager, configLoader, logger, true, timeProvider)

	// Clear previous calls
	mockHA.ClearServiceCalls()

	// Trigger begin_wake
	manager.handleBeginWake()

	// In read-only mode, no state changes should be made
	// The state manager is in read-only mode, so SetBool will not actually update HA
	// But we can verify the manager respects read-only flag
	if !manager.readOnly {
		t.Error("Manager should be in read-only mode")
	}
}

func TestAlarmTimeChange_ResetsTriggers(t *testing.T) {
	now := time.Date(2024, 1, 15, 9, 5, 0, 0, time.UTC)
	manager, _, stateManager, _ := setupTest(t, now)

	// Start manager so subscription is active
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Mark begin_wake as triggered
	manager.triggeredToday["begin_wake"] = now

	// Change alarm time
	newAlarmTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	stateManager.SetNumber("alarmTime", float64(newAlarmTime.UnixMilli()))

	// Give subscription callback time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify that begin_wake trigger was reset
	if _, exists := manager.triggeredToday["begin_wake"]; exists {
		t.Error("begin_wake trigger should be reset when alarm time changes")
	}
}

func TestIsSameDay(t *testing.T) {
	t1 := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 15, 23, 59, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 16, 0, 1, 0, 0, time.UTC)

	if !isSameDay(t1, t2) {
		t.Error("t1 and t2 should be on the same day")
	}

	if isSameDay(t1, t3) {
		t.Error("t1 and t3 should not be on the same day")
	}
}

func TestCheckTimeTriggers_Integration(t *testing.T) {
	// This test verifies the complete flow of checking time triggers

	// Test at begin_wake time (9:00 AM)
	now := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	manager, mockHA, stateManager, configLoader := setupTest(t, now)

	// Load schedule config for today
	if err := configLoader.LoadScheduleConfig(); err != nil {
		t.Skipf("Skipping test: schedule config not available: %v", err)
	}

	// Set all conditions for begin_wake
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetString("musicPlaybackType", "sleep")

	// Clear calls
	mockHA.ClearServiceCalls()

	// Check triggers
	manager.checkTimeTriggers()

	// Should trigger begin_wake
	if _, triggered := manager.triggeredToday["begin_wake"]; !triggered {
		t.Error("begin_wake should be triggered at 9:00 AM")
	}

	// Verify fade out was started
	fadeOut, _ := stateManager.GetBool("isFadeOutInProgress")
	if !fadeOut {
		t.Error("isFadeOutInProgress should be true after begin_wake")
	}
}

func TestCheckTimeTriggers_WakeTime(t *testing.T) {
	// Test at wake time (9:25 AM - 25 minutes after alarm)
	now := time.Date(2024, 1, 15, 9, 25, 0, 0, time.UTC)
	manager, mockHA, stateManager, configLoader := setupTest(t, now)

	// Load schedule config
	if err := configLoader.LoadScheduleConfig(); err != nil {
		t.Skipf("Skipping test: schedule config not available: %v", err)
	}

	// Set all conditions for wake
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetBool("isFadeOutInProgress", true)
	stateManager.SetBool("isNickHome", true)
	stateManager.SetBool("isCarolineHome", true)

	// Clear calls
	mockHA.ClearServiceCalls()

	// Check triggers
	manager.checkTimeTriggers()

	// Should trigger wake
	if _, triggered := manager.triggeredToday["wake"]; !triggered {
		t.Error("wake should be triggered at 9:25 AM")
	}

	// Verify light calls were made
	calls := mockHA.GetServiceCalls()
	if len(calls) == 0 {
		t.Error("Expected service calls for wake sequence")
	}
}

func TestSleepHygieneManager_ReadOnlyMode(t *testing.T) {
	logger := zap.NewNop()
	mockHA := ha.NewMockClient()
	stateManager := state.NewManager(mockHA, logger, true)
	configLoader := config.NewLoader("../../../configs", logger)

	// Set up initial state
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)
	stateManager.SetString("musicPlaybackType", "sleep")
	stateManager.SetBool("isFadeOutInProgress", false)
	stateManager.SetBool("isNickHome", true)
	stateManager.SetBool("isCarolineHome", true)

	// Set alarm time to 1 hour from now
	alarmTime := time.Now().Add(1 * time.Hour)
	stateManager.SetNumber("alarmTime", float64(alarmTime.UnixMilli()))

	manager := NewManager(mockHA, stateManager, configLoader, logger, true, nil)
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Test that read-only mode prevents state updates but doesn't error
	// The manager should handle read-only mode gracefully
	manager.checkTimeTriggers()

	// In read-only mode, no service calls should be made
	calls := mockHA.GetServiceCalls()
	if len(calls) != 0 {
		t.Errorf("Expected no service calls in read-only mode, got %d", len(calls))
	}
}

func TestSleepHygieneManager_HandleGoToBed(t *testing.T) {
	logger := zap.NewNop()
	mockHA := ha.NewMockClient()
	stateManager := state.NewManager(mockHA, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)

	manager := NewManager(mockHA, stateManager, configLoader, logger, false, nil)

	// Test the placeholder function - should not error
	manager.handleGoToBed()

	// No assertions needed - just testing that it runs without errors
}
