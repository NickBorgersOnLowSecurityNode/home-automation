package state

import (
	"testing"
	"time"

	"homeautomation/internal/ha"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()

	manager := NewManager(mockClient, logger, false)
	assert.NotNil(t, manager)
	assert.Equal(t, len(AllVariables), len(manager.variables))
}

func TestManager_SyncFromHA(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()

	// Setup mock states
	mockClient.SetState("input_boolean.nick_home", "on", map[string]interface{}{})
	mockClient.SetState("input_boolean.caroline_home", "off", map[string]interface{}{})
	mockClient.SetState("input_number.alarm_time", "1668524400000", map[string]interface{}{})
	mockClient.SetState("input_text.day_phase", "morning", map[string]interface{}{})

	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	err := manager.SyncFromHA()
	require.NoError(t, err)

	// Verify boolean
	value, err := manager.GetBool("isNickHome")
	assert.NoError(t, err)
	assert.True(t, value)

	value, err = manager.GetBool("isCarolineHome")
	assert.NoError(t, err)
	assert.False(t, value)

	// Verify number
	numValue, err := manager.GetNumber("alarmTime")
	assert.NoError(t, err)
	assert.Equal(t, 1668524400000.0, numValue)

	// Verify string
	strValue, err := manager.GetString("dayPhase")
	assert.NoError(t, err)
	assert.Equal(t, "morning", strValue)
}

func TestManager_GetBool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_boolean.nick_home", "on", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	t.Run("valid key", func(t *testing.T) {
		value, err := manager.GetBool("isNickHome")
		assert.NoError(t, err)
		assert.True(t, value)
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := manager.GetBool("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := manager.GetBool("dayPhase") // This is a string
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a boolean")
	})

	t.Run("default value when not synced", func(t *testing.T) {
		freshManager := NewManager(mockClient, logger, false)
		value, err := freshManager.GetBool("isExpectingSomeone")
		assert.NoError(t, err)
		assert.False(t, value) // Should return default
	})
}

func TestManager_SetBool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_boolean.expecting_someone", "off", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	t.Run("set to true", func(t *testing.T) {
		err := manager.SetBool("isExpectingSomeone", true)
		assert.NoError(t, err)

		value, err := manager.GetBool("isExpectingSomeone")
		assert.NoError(t, err)
		assert.True(t, value)

		// Verify service call
		calls := mockClient.GetServiceCalls()
		assert.NotEmpty(t, calls)
		lastCall := calls[len(calls)-1]
		assert.Equal(t, "input_boolean", lastCall.Domain)
		assert.Equal(t, "turn_on", lastCall.Service)
	})

	t.Run("set to false", func(t *testing.T) {
		mockClient.ClearServiceCalls()
		err := manager.SetBool("isExpectingSomeone", false)
		assert.NoError(t, err)

		value, err := manager.GetBool("isExpectingSomeone")
		assert.NoError(t, err)
		assert.False(t, value)

		calls := mockClient.GetServiceCalls()
		assert.NotEmpty(t, calls)
		assert.Equal(t, "turn_off", calls[0].Service)
	})

	t.Run("invalid key", func(t *testing.T) {
		err := manager.SetBool("nonexistent", true)
		assert.Error(t, err)
	})

	t.Run("wrong type", func(t *testing.T) {
		err := manager.SetBool("dayPhase", true)
		assert.Error(t, err)
	})
}

func TestManager_GetSetString(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_text.day_phase", "morning", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	// Get
	value, err := manager.GetString("dayPhase")
	assert.NoError(t, err)
	assert.Equal(t, "morning", value)

	// Set
	err = manager.SetString("dayPhase", "evening")
	assert.NoError(t, err)

	value, err = manager.GetString("dayPhase")
	assert.NoError(t, err)
	assert.Equal(t, "evening", value)

	// Verify service call
	calls := mockClient.GetServiceCalls()
	assert.NotEmpty(t, calls)
	lastCall := calls[len(calls)-1]
	assert.Equal(t, "input_text", lastCall.Domain)
	assert.Equal(t, "set_value", lastCall.Service)
	assert.Equal(t, "evening", lastCall.Data["value"])
}

func TestManager_GetSetNumber(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_number.alarm_time", "1668524400000", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	// Get
	value, err := manager.GetNumber("alarmTime")
	assert.NoError(t, err)
	assert.Equal(t, 1668524400000.0, value)

	// Set
	err = manager.SetNumber("alarmTime", 9999.5)
	assert.NoError(t, err)

	value, err = manager.GetNumber("alarmTime")
	assert.NoError(t, err)
	assert.Equal(t, 9999.5, value)

	// Verify service call
	calls := mockClient.GetServiceCalls()
	assert.NotEmpty(t, calls)
	lastCall := calls[len(calls)-1]
	assert.Equal(t, "input_number", lastCall.Domain)
	assert.Equal(t, "set_value", lastCall.Service)
	assert.Equal(t, 9999.5, lastCall.Data["value"])
}

func TestManager_CompareAndSwapBool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_boolean.fade_out_in_progress", "off", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	t.Run("successful swap", func(t *testing.T) {
		swapped, err := manager.CompareAndSwapBool("isFadeOutInProgress", false, true)
		assert.NoError(t, err)
		assert.True(t, swapped)

		value, _ := manager.GetBool("isFadeOutInProgress")
		assert.True(t, value)
	})

	t.Run("failed swap - value changed", func(t *testing.T) {
		// Value is now true, trying to swap from false should fail
		swapped, err := manager.CompareAndSwapBool("isFadeOutInProgress", false, true)
		assert.NoError(t, err)
		assert.False(t, swapped)

		// Value should remain true
		value, _ := manager.GetBool("isFadeOutInProgress")
		assert.True(t, value)
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := manager.CompareAndSwapBool("nonexistent", false, true)
		assert.Error(t, err)
	})
}

func TestManager_Subscribe(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_boolean.nick_home", "off", map[string]interface{}{})
	mockClient.SetState("input_boolean.caroline_home", "off", map[string]interface{}{})
	mockClient.SetState("input_boolean.tori_here", "off", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	t.Run("state change notification", func(t *testing.T) {
		changeCount := 0
		var receivedOld, receivedNew interface{}

		sub, err := manager.Subscribe("isNickHome", func(key string, oldValue, newValue interface{}) {
			changeCount++
			receivedOld = oldValue
			receivedNew = newValue
		})
		require.NoError(t, err)
		defer sub.Unsubscribe()

		// Simulate state change from HA
		mockClient.SimulateStateChange("input_boolean.nick_home", "on")

		assert.Equal(t, 1, changeCount)
		assert.Equal(t, false, receivedOld)
		assert.Equal(t, true, receivedNew)
	})

	t.Run("multiple subscribers", func(t *testing.T) {
		count1, count2 := 0, 0

		sub1, _ := manager.Subscribe("isCarolineHome", func(key string, oldValue, newValue interface{}) {
			count1++
		})
		sub2, _ := manager.Subscribe("isCarolineHome", func(key string, oldValue, newValue interface{}) {
			count2++
		})

		mockClient.SimulateStateChange("input_boolean.caroline_home", "on")

		assert.Equal(t, 1, count1)
		assert.Equal(t, 1, count2)

		sub1.Unsubscribe()
		sub2.Unsubscribe()
	})

	t.Run("unsubscribe", func(t *testing.T) {
		changeCount := 0

		sub, err := manager.Subscribe("isToriHere", func(key string, oldValue, newValue interface{}) {
			changeCount++
		})
		require.NoError(t, err)

		mockClient.SimulateStateChange("input_boolean.tori_here", "on")
		assert.Equal(t, 1, changeCount)

		sub.Unsubscribe()

		mockClient.SimulateStateChange("input_boolean.tori_here", "off")
		assert.Equal(t, 1, changeCount) // Should not increment
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := manager.Subscribe("nonexistent", func(key string, oldValue, newValue interface{}) {})
		assert.Error(t, err)
	})
}

func TestManagerNotifySubscribersIsSynchronous(t *testing.T) {
	manager := &Manager{
		logger: zap.NewNop(),
		subscribers: map[string]map[uint64]StateChangeHandler{
			"test": {
				1: func(string, interface{}, interface{}) {
					time.Sleep(50 * time.Millisecond)
				},
				2: func(string, interface{}, interface{}) {},
			},
		},
	}

	start := time.Now()
	manager.notifySubscribers("test", nil, nil)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestManagerNotifySubscribersRecoversFromPanics(t *testing.T) {
	secondCalled := false
	manager := &Manager{
		logger: zap.NewNop(),
		subscribers: map[string]map[uint64]StateChangeHandler{
			"test": {
				1: func(string, interface{}, interface{}) {
					panic("boom")
				},
				2: func(string, interface{}, interface{}) {
					secondCalled = true
				},
			},
		},
	}

	assert.NotPanics(t, func() {
		manager.notifySubscribers("test", nil, nil)
	})
	assert.True(t, secondCalled)
}

func TestManager_GetAllValues(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_boolean.nick_home", "on", map[string]interface{}{})
	mockClient.SetState("input_text.day_phase", "morning", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	values := manager.GetAllValues()
	assert.NotEmpty(t, values)
	assert.True(t, values["isNickHome"].(bool))
	assert.Equal(t, "morning", values["dayPhase"].(string))
}

func TestExtractEntityName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"input_boolean.nick_home", "nick_home"},
		{"input_number.alarm_time", "alarm_time"},
		{"input_text.day_phase", "day_phase"},
		{"simple", "simple"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := extractEntityName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	mockClient.SetState("input_boolean.nick_home", "off", map[string]interface{}{})
	mockClient.Connect()

	manager := NewManager(mockClient, logger, false)
	manager.SyncFromHA()

	// Run concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				manager.SetBool("isNickHome", j%2 == 0)
				manager.GetBool("isNickHome")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should complete without race conditions
	value, err := manager.GetBool("isNickHome")
	assert.NoError(t, err)
	assert.NotNil(t, value)
}
