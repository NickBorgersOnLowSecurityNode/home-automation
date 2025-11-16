package tv

import (
	"testing"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

func TestTVManager_AppleTVStateChange(t *testing.T) {
	tests := []struct {
		name              string
		appleTVState      string
		expectedIsPlaying bool
		description       string
	}{
		{
			name:              "Apple TV playing",
			appleTVState:      "playing",
			expectedIsPlaying: true,
			description:       "When Apple TV state is 'playing', isAppleTVPlaying should be true",
		},
		{
			name:              "Apple TV paused",
			appleTVState:      "paused",
			expectedIsPlaying: false,
			description:       "When Apple TV state is 'paused', isAppleTVPlaying should be false",
		},
		{
			name:              "Apple TV idle",
			appleTVState:      "idle",
			expectedIsPlaying: false,
			description:       "When Apple TV state is 'idle', isAppleTVPlaying should be false",
		},
		{
			name:              "Apple TV off",
			appleTVState:      "off",
			expectedIsPlaying: false,
			description:       "When Apple TV state is 'off', isAppleTVPlaying should be false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Create TV manager
			manager := NewManager(mockHA, stateMgr, logger, false)

			// Simulate Apple TV state change
			newState := &ha.State{
				EntityID: "media_player.big_beautiful_oled",
				State:    tt.appleTVState,
			}
			manager.handleAppleTVStateChange("media_player.big_beautiful_oled", nil, newState)

			// Verify isAppleTVPlaying state
			isPlaying, err := stateMgr.GetBool("isAppleTVPlaying")
			if err != nil {
				t.Fatalf("Failed to get isAppleTVPlaying: %v", err)
			}

			if isPlaying != tt.expectedIsPlaying {
				t.Errorf("Expected isAppleTVPlaying=%v, got %v", tt.expectedIsPlaying, isPlaying)
			}
		})
	}
}

func TestTVManager_SyncBoxPowerChange(t *testing.T) {
	tests := []struct {
		name           string
		syncBoxState   string
		expectedIsTVOn bool
		description    string
	}{
		{
			name:           "Sync box on",
			syncBoxState:   "on",
			expectedIsTVOn: true,
			description:    "When sync box is on, isTVon should be true",
		},
		{
			name:           "Sync box off",
			syncBoxState:   "off",
			expectedIsTVOn: false,
			description:    "When sync box is off, isTVon should be false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Create TV manager
			manager := NewManager(mockHA, stateMgr, logger, false)

			// Simulate sync box state change
			newState := &ha.State{
				EntityID: "switch.sync_box_power",
				State:    tt.syncBoxState,
			}
			manager.handleSyncBoxPowerChange("switch.sync_box_power", nil, newState)

			// Verify isTVon state
			isTVOn, err := stateMgr.GetBool("isTVon")
			if err != nil {
				t.Fatalf("Failed to get isTVon: %v", err)
			}

			if isTVOn != tt.expectedIsTVOn {
				t.Errorf("Expected isTVon=%v, got %v", tt.expectedIsTVOn, isTVOn)
			}
		})
	}
}

func TestTVManager_SyncBoxOff_SetsTVPlayingFalse(t *testing.T) {
	// Create mock HA client and state manager
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Create TV manager
	manager := NewManager(mockHA, stateMgr, logger, false)

	// Initially set isTVPlaying to true
	if err := stateMgr.SetBool("isTVPlaying", true); err != nil {
		t.Fatalf("Failed to set initial isTVPlaying: %v", err)
	}

	// Simulate sync box turning off
	newState := &ha.State{
		EntityID: "switch.sync_box_power",
		State:    "off",
	}
	manager.handleSyncBoxPowerChange("switch.sync_box_power", nil, newState)

	// Verify isTVPlaying is now false
	isTVPlaying, err := stateMgr.GetBool("isTVPlaying")
	if err != nil {
		t.Fatalf("Failed to get isTVPlaying: %v", err)
	}

	if isTVPlaying != false {
		t.Errorf("Expected isTVPlaying=false when TV turns off, got %v", isTVPlaying)
	}
}

func TestTVManager_HDMIInputChange(t *testing.T) {
	tests := []struct {
		name                string
		hdmiInput           string
		isAppleTVPlaying    bool
		expectedIsTVPlaying bool
		description         string
	}{
		{
			name:                "Apple TV input - Apple TV playing",
			hdmiInput:           "AppleTV",
			isAppleTVPlaying:    true,
			expectedIsTVPlaying: true,
			description:         "When AppleTV input is selected and Apple TV is playing, isTVPlaying=true",
		},
		{
			name:                "Apple TV input - Apple TV not playing",
			hdmiInput:           "AppleTV",
			isAppleTVPlaying:    false,
			expectedIsTVPlaying: false,
			description:         "When AppleTV input is selected and Apple TV is not playing, isTVPlaying=false",
		},
		{
			name:                "HDMI 1 input - assume playing",
			hdmiInput:           "HDMI 1",
			isAppleTVPlaying:    false,
			expectedIsTVPlaying: true,
			description:         "When non-AppleTV input is selected, assume TV is playing",
		},
		{
			name:                "HDMI 2 input - assume playing",
			hdmiInput:           "HDMI 2",
			isAppleTVPlaying:    true,
			expectedIsTVPlaying: true,
			description:         "When non-AppleTV input is selected, assume TV is playing regardless of Apple TV state",
		},
		{
			name:                "Console input - assume playing",
			hdmiInput:           "Console",
			isAppleTVPlaying:    false,
			expectedIsTVPlaying: true,
			description:         "When Console input is selected, assume TV is playing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Create TV manager
			manager := NewManager(mockHA, stateMgr, logger, false)

			// Set isAppleTVPlaying state
			if err := stateMgr.SetBool("isAppleTVPlaying", tt.isAppleTVPlaying); err != nil {
				t.Fatalf("Failed to set isAppleTVPlaying: %v", err)
			}

			// Simulate HDMI input change
			newState := &ha.State{
				EntityID: "select.sync_box_hdmi_input",
				State:    tt.hdmiInput,
			}
			manager.handleHDMIInputChange("select.sync_box_hdmi_input", nil, newState)

			// Verify isTVPlaying state
			isTVPlaying, err := stateMgr.GetBool("isTVPlaying")
			if err != nil {
				t.Fatalf("Failed to get isTVPlaying: %v", err)
			}

			if isTVPlaying != tt.expectedIsTVPlaying {
				t.Errorf("Expected isTVPlaying=%v, got %v (hdmiInput=%s, isAppleTVPlaying=%v)",
					tt.expectedIsTVPlaying, isTVPlaying, tt.hdmiInput, tt.isAppleTVPlaying)
			}
		})
	}
}

func TestTVManager_AppleTVPlayingChange_RecalculatesTVPlaying(t *testing.T) {
	tests := []struct {
		name                string
		hdmiInput           string
		oldAppleTVPlaying   bool
		newAppleTVPlaying   bool
		expectedIsTVPlaying bool
		description         string
	}{
		{
			name:                "AppleTV input - changes from playing to not playing",
			hdmiInput:           "AppleTV",
			oldAppleTVPlaying:   true,
			newAppleTVPlaying:   false,
			expectedIsTVPlaying: false,
			description:         "When Apple TV stops playing on AppleTV input, isTVPlaying should become false",
		},
		{
			name:                "AppleTV input - changes from not playing to playing",
			hdmiInput:           "AppleTV",
			oldAppleTVPlaying:   false,
			newAppleTVPlaying:   true,
			expectedIsTVPlaying: true,
			description:         "When Apple TV starts playing on AppleTV input, isTVPlaying should become true",
		},
		{
			name:                "Non-AppleTV input - Apple TV state doesn't affect isTVPlaying",
			hdmiInput:           "HDMI 1",
			oldAppleTVPlaying:   true,
			newAppleTVPlaying:   false,
			expectedIsTVPlaying: true,
			description:         "When on non-AppleTV input, Apple TV state changes don't affect isTVPlaying",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Create TV manager
			manager := NewManager(mockHA, stateMgr, logger, false)

			// Set initial HDMI input in mock HA client
			mockHA.SetState("select.sync_box_hdmi_input", tt.hdmiInput, nil)

			// Set initial isAppleTVPlaying state
			if err := stateMgr.SetBool("isAppleTVPlaying", tt.oldAppleTVPlaying); err != nil {
				t.Fatalf("Failed to set initial isAppleTVPlaying: %v", err)
			}

			// Update to new isAppleTVPlaying state
			if err := stateMgr.SetBool("isAppleTVPlaying", tt.newAppleTVPlaying); err != nil {
				t.Fatalf("Failed to set new isAppleTVPlaying: %v", err)
			}

			// Simulate the state change handler
			manager.handleAppleTVPlayingChange("isAppleTVPlaying", tt.oldAppleTVPlaying, tt.newAppleTVPlaying)

			// Small delay to allow state propagation
			time.Sleep(10 * time.Millisecond)

			// Verify isTVPlaying state
			isTVPlaying, err := stateMgr.GetBool("isTVPlaying")
			if err != nil {
				t.Fatalf("Failed to get isTVPlaying: %v", err)
			}

			if isTVPlaying != tt.expectedIsTVPlaying {
				t.Errorf("Expected isTVPlaying=%v, got %v (hdmiInput=%s, isAppleTVPlaying=%v->%v)",
					tt.expectedIsTVPlaying, isTVPlaying, tt.hdmiInput, tt.oldAppleTVPlaying, tt.newAppleTVPlaying)
			}
		})
	}
}

func TestTVManager_Start_InitializesStates(t *testing.T) {
	// Create mock HA client
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Set up initial entity states in mock HA
	mockHA.SetState("media_player.big_beautiful_oled", "playing", nil)
	mockHA.SetState("switch.sync_box_power", "on", nil)
	mockHA.SetState("select.sync_box_hdmi_input", "AppleTV", nil)

	// Create TV manager
	manager := NewManager(mockHA, stateMgr, logger, false)

	// Start the manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start TV manager: %v", err)
	}

	// Small delay to allow initialization
	time.Sleep(50 * time.Millisecond)

	// Verify states were initialized correctly
	isAppleTVPlaying, err := stateMgr.GetBool("isAppleTVPlaying")
	if err != nil {
		t.Fatalf("Failed to get isAppleTVPlaying: %v", err)
	}
	if !isAppleTVPlaying {
		t.Errorf("Expected isAppleTVPlaying=true after initialization, got false")
	}

	isTVOn, err := stateMgr.GetBool("isTVon")
	if err != nil {
		t.Fatalf("Failed to get isTVon: %v", err)
	}
	if !isTVOn {
		t.Errorf("Expected isTVon=true after initialization, got false")
	}

	isTVPlaying, err := stateMgr.GetBool("isTVPlaying")
	if err != nil {
		t.Fatalf("Failed to get isTVPlaying: %v", err)
	}
	if !isTVPlaying {
		t.Errorf("Expected isTVPlaying=true after initialization (AppleTV input + playing), got false")
	}

	// Clean up
	if err := manager.Stop(); err != nil {
		t.Fatalf("Failed to stop TV manager: %v", err)
	}
}

func TestTVManager_Stop_CleansUpSubscriptions(t *testing.T) {
	// Create mock HA client and state manager
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Create TV manager
	manager := NewManager(mockHA, stateMgr, logger, false)

	// Start the manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start TV manager: %v", err)
	}

	// Verify subscriptions exist
	if manager.appleTVSub == nil {
		t.Error("Expected appleTVSub to be set after Start()")
	}
	if manager.syncBoxSub == nil {
		t.Error("Expected syncBoxSub to be set after Start()")
	}
	if manager.hdmiInputSub == nil {
		t.Error("Expected hdmiInputSub to be set after Start()")
	}

	// Stop the manager
	if err := manager.Stop(); err != nil {
		t.Fatalf("Failed to stop TV manager: %v", err)
	}

	// Test passes if Stop() doesn't error (cleanup is successful)
}

func TestTVManager_ReadOnlyMode(t *testing.T) {
	// Create mock HA client
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()

	// Create state manager in read-only mode
	stateMgr := state.NewManager(mockHA, logger, true)

	// Initialize state from HA (this populates the cache)
	mockHA.SetState("input_boolean.apple_tv_playing", "off", nil)
	if err := stateMgr.SyncFromHA(); err != nil {
		t.Fatalf("Failed to sync from HA: %v", err)
	}

	// Create TV manager in read-only mode
	_ = NewManager(mockHA, stateMgr, logger, true)

	// Simulate HA state change (this should update local cache)
	mockHA.SimulateStateChange("input_boolean.apple_tv_playing", "on")

	// Small delay to allow state propagation
	time.Sleep(10 * time.Millisecond)

	// In read-only mode, local cache should still be updated when HA sends changes
	isPlaying, err := stateMgr.GetBool("isAppleTVPlaying")
	if err != nil {
		t.Fatalf("Failed to get isAppleTVPlaying: %v", err)
	}
	if !isPlaying {
		t.Error("Expected isAppleTVPlaying=true (local cache should update from HA changes even in read-only mode)")
	}

	// Verify that the manager doesn't try to write back to HA (this is implicit -
	// if it tried, it would error, but the state manager only prevents writes,
	// not reads or cache updates from HA)
}
