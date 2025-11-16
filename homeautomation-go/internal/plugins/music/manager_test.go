package music

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

func TestMusicManager_SelectAppropriateMusicMode(t *testing.T) {
	tests := []struct {
		name              string
		isAnyoneHome      bool
		isAnyoneAsleep    bool
		dayPhase          string
		currentMusicType  string
		expectedMusicType string
		description       string
	}{
		{
			name:              "No one home - stop music",
			isAnyoneHome:      false,
			isAnyoneAsleep:    false,
			dayPhase:          "day",
			currentMusicType:  "day",
			expectedMusicType: "",
			description:       "When no one is home, music should stop",
		},
		{
			name:              "Someone asleep - sleep mode",
			isAnyoneHome:      true,
			isAnyoneAsleep:    true,
			dayPhase:          "day",
			currentMusicType:  "day",
			expectedMusicType: "sleep",
			description:       "Sleep mode has highest priority",
		},
		{
			name:              "Morning - day mode (no wake-up event)",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "morning",
			currentMusicType:  "",
			expectedMusicType: "day",
			description:       "Morning phase without wake-up event triggers day music",
		},
		{
			name:              "Day - day mode",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "day",
			currentMusicType:  "",
			expectedMusicType: "day",
			description:       "Day phase triggers day music",
		},
		{
			name:              "Sunset - evening mode",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "sunset",
			currentMusicType:  "",
			expectedMusicType: "evening",
			description:       "Sunset phase triggers evening music",
		},
		{
			name:              "Dusk - evening mode",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "dusk",
			currentMusicType:  "",
			expectedMusicType: "evening",
			description:       "Dusk phase triggers evening music",
		},
		{
			name:              "Winddown - winddown mode",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "winddown",
			currentMusicType:  "",
			expectedMusicType: "winddown",
			description:       "Winddown phase triggers winddown music",
		},
		{
			name:              "Night - winddown mode",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "night",
			currentMusicType:  "",
			expectedMusicType: "winddown",
			description:       "Night phase triggers winddown music",
		},
		{
			name:              "Winddown but sleep playing - keep sleep",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "winddown",
			currentMusicType:  "sleep",
			expectedMusicType: "sleep",
			description:       "Don't override sleep music with winddown",
		},
		{
			name:              "Unknown phase - default to day",
			isAnyoneHome:      true,
			isAnyoneAsleep:    false,
			dayPhase:          "unknown",
			currentMusicType:  "",
			expectedMusicType: "day",
			description:       "Unknown phases default to day mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager (NOT read-only for tests)
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Create music config (minimal for testing)
			config := &MusicConfig{
				Music: map[string]MusicMode{
					"morning":  {},
					"day":      {},
					"evening":  {},
					"winddown": {},
					"sleep":    {},
					"sex":      {},
					"wakeup":   {},
				},
			}

			// Use a fixed time provider with a Monday (not Sunday) for testing
			// This ensures tests are independent of what day they run on
			fixedTime := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC) // Monday, January 6, 2025
			timeProvider := FixedTimeProvider{FixedTime: fixedTime}

			// Create manager
			manager := NewManager(mockHA, stateMgr, config, logger, true, timeProvider)

			// Set up initial state
			if err := stateMgr.SetBool("isAnyoneHome", tt.isAnyoneHome); err != nil {
				t.Fatalf("Failed to set isAnyoneHome: %v", err)
			}
			if err := stateMgr.SetBool("isAnyoneAsleep", tt.isAnyoneAsleep); err != nil {
				t.Fatalf("Failed to set isAnyoneAsleep: %v", err)
			}
			if err := stateMgr.SetString("dayPhase", tt.dayPhase); err != nil {
				t.Fatalf("Failed to set dayPhase: %v", err)
			}
			if err := stateMgr.SetString("musicPlaybackType", tt.currentMusicType); err != nil {
				t.Fatalf("Failed to set musicPlaybackType: %v", err)
			}

			// Execute music mode selection
			manager.selectAppropriateMusicMode()

			// Verify result
			actualMusicType, err := stateMgr.GetString("musicPlaybackType")
			if err != nil {
				t.Fatalf("Failed to get musicPlaybackType: %v", err)
			}

			if actualMusicType != tt.expectedMusicType {
				t.Errorf("Expected music type %q, got %q. Description: %s",
					tt.expectedMusicType, actualMusicType, tt.description)
			}
		})
	}
}

func TestMusicManager_DetermineMusicModeFromDayPhase(t *testing.T) {
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)
	config := &MusicConfig{}

	// Use a fixed time provider with a Monday (not Sunday) for testing
	fixedTime := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC) // Monday, January 6, 2025
	timeProvider := FixedTimeProvider{FixedTime: fixedTime}

	manager := NewManager(mockHA, stateMgr, config, logger, true, timeProvider)

	tests := []struct {
		dayPhase          string
		currentMusicType  string
		expectedMusicMode string
	}{
		{"morning", "", "day"}, // Morning without wake-up event = day music
		{"day", "", "day"},
		{"sunset", "", "evening"},
		{"dusk", "", "evening"},
		{"winddown", "", "winddown"},
		{"night", "", "winddown"},
		{"winddown", "sleep", "sleep"}, // Don't override sleep
		{"unknown", "", "day"},         // Default to day
	}

	for _, tt := range tests {
		t.Run(tt.dayPhase+"_"+tt.currentMusicType, func(t *testing.T) {
			result := manager.determineMusicModeFromDayPhase(tt.dayPhase, tt.currentMusicType, "", false)
			if result != tt.expectedMusicMode {
				t.Errorf("For dayPhase=%s, currentMusicType=%s: expected %s, got %s",
					tt.dayPhase, tt.currentMusicType, tt.expectedMusicMode, result)
			}
		})
	}
}

func TestMusicManager_StateChangeHandling(t *testing.T) {
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	config := &MusicConfig{
		Music: map[string]MusicMode{
			"morning":  {},
			"day":      {},
			"evening":  {},
			"winddown": {},
			"sleep":    {},
			"sex":      {},
			"wakeup":   {},
		},
	}

	// Use a fixed time provider with a Monday (not Sunday) for testing
	fixedTime := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC) // Monday, January 6, 2025
	timeProvider := FixedTimeProvider{FixedTime: fixedTime}

	manager := NewManager(mockHA, stateMgr, config, logger, true, timeProvider)

	// Set initial state
	if err := stateMgr.SetBool("isAnyoneHome", true); err != nil {
		t.Fatalf("Failed to set isAnyoneHome: %v", err)
	}
	if err := stateMgr.SetBool("isAnyoneAsleep", false); err != nil {
		t.Fatalf("Failed to set isAnyoneAsleep: %v", err)
	}
	if err := stateMgr.SetString("dayPhase", "day"); err != nil {
		t.Fatalf("Failed to set dayPhase: %v", err)
	}
	if err := stateMgr.SetString("musicPlaybackType", ""); err != nil {
		t.Fatalf("Failed to set musicPlaybackType: %v", err)
	}

	// Start manager (which subscribes to state changes)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Initial selection should set day mode
	musicType, err := stateMgr.GetString("musicPlaybackType")
	if err != nil {
		t.Fatalf("Failed to get musicPlaybackType: %v", err)
	}
	if musicType != "day" {
		t.Errorf("Expected initial music type 'day', got %q", musicType)
	}

	// Change to evening phase - should trigger music mode change
	if err := stateMgr.SetString("dayPhase", "sunset"); err != nil {
		t.Fatalf("Failed to set dayPhase: %v", err)
	}

	// Give the subscription callback time to execute
	time.Sleep(100 * time.Millisecond)

	musicType, err = stateMgr.GetString("musicPlaybackType")
	if err != nil {
		t.Fatalf("Failed to get musicPlaybackType: %v", err)
	}
	if musicType != "evening" {
		t.Errorf("Expected music type 'evening' after sunset, got %q", musicType)
	}

	// Someone goes to sleep - should trigger sleep mode
	if err := stateMgr.SetBool("isAnyoneAsleep", true); err != nil {
		t.Fatalf("Failed to set isAnyoneAsleep: %v", err)
	}

	// Give the subscription callback time to execute
	time.Sleep(100 * time.Millisecond)

	musicType, err = stateMgr.GetString("musicPlaybackType")
	if err != nil {
		t.Fatalf("Failed to get musicPlaybackType: %v", err)
	}
	if musicType != "sleep" {
		t.Errorf("Expected music type 'sleep' when someone is asleep, got %q", musicType)
	}
}

func TestMusicManager_Stop(t *testing.T) {
	// Create mock HA client and state manager
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Create music config
	config := &MusicConfig{
		Music: map[string]MusicMode{
			"morning":  {},
			"day":      {},
			"evening":  {},
			"winddown": {},
			"sleep":    {},
			"sex":      {},
			"wakeup":   {},
		},
	}

	// Create manager
	manager := NewManager(mockHA, stateMgr, config, logger, true, nil)

	// Set initial state
	if err := stateMgr.SetBool("isAnyoneHome", true); err != nil {
		t.Fatalf("Failed to set isAnyoneHome: %v", err)
	}
	if err := stateMgr.SetBool("isAnyoneAsleep", false); err != nil {
		t.Fatalf("Failed to set isAnyoneAsleep: %v", err)
	}
	if err := stateMgr.SetString("dayPhase", "day"); err != nil {
		t.Fatalf("Failed to set dayPhase: %v", err)
	}

	// Start manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Verify subscriptions were created
	if len(manager.subscriptions) != 3 {
		t.Errorf("Expected 3 subscriptions, got %d", len(manager.subscriptions))
	}

	// Stop manager
	manager.Stop()

	// Verify subscriptions were cleaned up
	if manager.subscriptions != nil {
		t.Errorf("Expected subscriptions to be nil after Stop(), got %v", manager.subscriptions)
	}
}

// findRepoRoot finds the repository root by looking for go.mod
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up the directory tree until we find the parent of homeautomation-go
	for {
		// Check if we're at or can find the homeautomation-go directory
		if filepath.Base(dir) == "homeautomation-go" {
			return filepath.Dir(dir) // Return parent directory
		}

		// Check if configs directory exists here
		configsDir := filepath.Join(dir, "configs")
		if _, err := os.Stat(configsDir); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find repository root")
		}
		dir = parent
	}
}

func TestLoadMusicConfig(t *testing.T) {
	// Find the repository root and construct path to config file
	repoRoot := findRepoRoot(t)
	configPath := filepath.Join(repoRoot, "configs", "music_config.yaml")

	// Test with the actual config file
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load music config: %v", err)
	}

	// Verify all expected modes are present
	expectedModes := []string{"morning", "day", "evening", "winddown", "sleep", "sex", "wakeup"}
	for _, mode := range expectedModes {
		if _, ok := config.Music[mode]; !ok {
			t.Errorf("Missing expected music mode: %s", mode)
		}
	}

	// Verify morning mode has expected structure
	morningMode, ok := config.Music["morning"]
	if !ok {
		t.Fatal("Morning mode not found")
	}

	if len(morningMode.Participants) == 0 {
		t.Error("Morning mode should have participants")
	}

	if len(morningMode.PlaybackOptions) == 0 {
		t.Error("Morning mode should have playback options")
	}

	// Verify a participant has expected fields
	if len(morningMode.Participants) > 0 {
		participant := morningMode.Participants[0]
		if participant.PlayerName == "" {
			t.Error("Participant should have player_name")
		}
		if participant.BaseVolume == 0 {
			t.Error("Participant should have base_volume")
		}
	}

	// Verify a playback option has expected fields
	if len(morningMode.PlaybackOptions) > 0 {
		option := morningMode.PlaybackOptions[0]
		if option.URI == "" {
			t.Error("Playback option should have uri")
		}
		if option.MediaType == "" {
			t.Error("Playback option should have media_type")
		}
		if option.VolumeMultiplier == 0 {
			t.Error("Playback option should have volume_multiplier")
		}
	}
}

func TestMusicManager_ReadOnlyMode(t *testing.T) {
	logger := zap.NewNop()
	mockClient := ha.NewMockClient()
	// Create state manager in read-only mode
	stateManager := state.NewManager(mockClient, logger, true)

	// Initialize required state variables (can set because they're LocalOnly or initial sync)
	_ = stateManager.SetBool("isAnyoneHome", true)
	_ = stateManager.SetBool("isAnyoneAsleep", false)
	_ = stateManager.SetString("dayPhase", "day")
	_ = stateManager.SetString("musicPlaybackType", "")

	config := &MusicConfig{
		Music: map[string]MusicMode{
			"day":   {},
			"sleep": {},
		},
	}

	manager := NewManager(mockClient, stateManager, config, logger, true, nil)

	// Test selecting music mode in read-only mode - should handle gracefully
	manager.selectAppropriateMusicMode()

	// Test with sleep scenario
	_ = stateManager.SetBool("isAnyoneAsleep", true)
	manager.selectAppropriateMusicMode()

	// Test with no one home
	_ = stateManager.SetBool("isAnyoneHome", false)
	manager.selectAppropriateMusicMode()

	// If we get here without panicking, the read-only mode handling worked correctly
	// The actual verification is that no errors are thrown, just debug logs
}
