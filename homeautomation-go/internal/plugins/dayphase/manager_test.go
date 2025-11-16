package dayphase

import (
	"testing"
	"time"

	"homeautomation/internal/config"
	dayphaselib "homeautomation/internal/dayphase"
	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)
	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	assert.NotNil(t, manager)
	assert.Equal(t, mockClient, manager.haClient)
	assert.Equal(t, stateManager, manager.stateManager)
	assert.Equal(t, configLoader, manager.configLoader)
	assert.Equal(t, calculator, manager.calculator)
	assert.False(t, manager.readOnly)
}

func TestManagerStartStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)
	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	// Start the manager
	err := manager.Start()
	assert.NoError(t, err)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the manager
	manager.Stop()
}

func TestUpdateSunEventAndDayPhase(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)

	// Initialize state variables
	err := stateManager.SyncFromHA()
	assert.NoError(t, err)

	configLoader := config.NewLoader("../../../configs", logger)

	// Load schedule config for day phase calculation
	err = configLoader.LoadScheduleConfig()
	if err != nil {
		// If config file doesn't exist in test environment, skip this part
		t.Logf("Warning: Could not load schedule config: %v", err)
	}

	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	// Update sun event and day phase
	err = manager.updateSunEventAndDayPhase()
	assert.NoError(t, err)

	// Verify that sunevent was set
	sunEvent, err := stateManager.GetString("sunevent")
	assert.NoError(t, err)
	assert.NotEmpty(t, sunEvent)
	assert.Contains(t, []string{"morning", "day", "sunset", "dusk", "night"}, sunEvent)

	// Verify that dayPhase was set
	dayPhase, err := stateManager.GetString("dayPhase")
	assert.NoError(t, err)
	assert.NotEmpty(t, dayPhase)
	assert.Contains(t, []string{"morning", "day", "sunset", "dusk", "winddown", "night"}, dayPhase)
}

func TestUpdateSunEventAndDayPhaseReadOnly(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, true) // READ ONLY

	// Initialize state variables
	err := stateManager.SyncFromHA()
	assert.NoError(t, err)

	// Set initial values
	err = stateManager.SetString("sunevent", "night")
	assert.Error(t, err) // Should error in read-only mode

	configLoader := config.NewLoader("../../../configs", logger)
	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, true)

	// Update should succeed even in read-only mode (just won't write to HA)
	err = manager.updateSunEventAndDayPhase()
	assert.NoError(t, err)
}

func TestManagerPeriodicUpdate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)
	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	// Start the manager
	err := manager.Start()
	assert.NoError(t, err)

	// Let it run for a short time
	time.Sleep(200 * time.Millisecond)

	// Verify initial state was set
	sunEvent, err := stateManager.GetString("sunevent")
	assert.NoError(t, err)
	assert.NotEmpty(t, sunEvent)

	dayPhase, err := stateManager.GetString("dayPhase")
	assert.NoError(t, err)
	assert.NotEmpty(t, dayPhase)

	// Stop the manager
	manager.Stop()
}

func TestManagerWithDifferentCoordinates(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)

	// Test with different coordinates (San Francisco)
	calculator := dayphaselib.NewCalculator(37.7749, -122.4194, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	err := manager.Start()
	assert.NoError(t, err)

	// Give it time to calculate
	time.Sleep(100 * time.Millisecond)

	// Should still work with different coordinates
	sunEvent, err := stateManager.GetString("sunevent")
	assert.NoError(t, err)
	assert.NotEmpty(t, sunEvent)

	manager.Stop()
}

func TestUpdateSunEventNoChange(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)
	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	// Initialize state
	err := stateManager.SyncFromHA()
	assert.NoError(t, err)

	// First update
	err = manager.updateSunEventAndDayPhase()
	assert.NoError(t, err)

	sunEvent1, _ := stateManager.GetString("sunevent")

	// Second update (should be same value, no change)
	err = manager.updateSunEventAndDayPhase()
	assert.NoError(t, err)

	sunEvent2, _ := stateManager.GetString("sunevent")
	assert.Equal(t, sunEvent1, sunEvent2)
}

func TestUpdateDayPhaseNoChange(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)
	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	// Initialize state
	err := stateManager.SyncFromHA()
	assert.NoError(t, err)

	// First update
	err = manager.updateSunEventAndDayPhase()
	assert.NoError(t, err)

	dayPhase1, _ := stateManager.GetString("dayPhase")

	// Second update (should be same value, no change)
	err = manager.updateSunEventAndDayPhase()
	assert.NoError(t, err)

	dayPhase2, _ := stateManager.GetString("dayPhase")
	assert.Equal(t, dayPhase1, dayPhase2)
}

func TestManagerStopBeforeStart(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	configLoader := config.NewLoader("../../../configs", logger)
	calculator := dayphaselib.NewCalculator(32.85486, -97.50515, logger)

	manager := NewManager(mockClient, stateManager, configLoader, calculator, logger, false)

	// Should not panic if Stop is called before Start
	assert.NotPanics(t, func() {
		manager.Stop()
	})
}
