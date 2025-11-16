package lighting

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "hue_config.yaml")

	configContent := `---
rooms:
  - hue_group: Living Room
    hass_area_id: living_room_2
    on_if_true: isAnyoneHomeAndAwake
    on_if_false: isTVPlaying
    off_if_true: isEveryoneAsleep
    off_if_false: isAnyoneHome
    increase_brightness_if_true: isHaveGuests
    transition_seconds: 30
  - hue_group: Primary Suite
    hass_area_id: master_bedroom
    on_if_true: ~
    on_if_false: isMasterAsleep
    off_if_true: isMasterAsleep
    off_if_false: isAnyoneHome
    increase_brightness_if_true: ~
    transition_seconds: 180
  - hue_group: Front of House
    hass_area_id: front_of_house
    on_if_true:
      - isHaveGuests
      - didOwnerJustReturnHome
    on_if_false: ~
    off_if_true: isEveryoneAsleep
    off_if_false:
      - isHaveGuests
      - didOwnerJustReturnHome
    increase_brightness_if_true: ~
    transition_seconds: 120
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the config
	if len(config.Rooms) != 3 {
		t.Errorf("Expected 3 rooms, got %d", len(config.Rooms))
	}

	// Check Living Room config
	livingRoom := config.Rooms[0]
	if livingRoom.HueGroup != "Living Room" {
		t.Errorf("Expected HueGroup 'Living Room', got '%s'", livingRoom.HueGroup)
	}
	if livingRoom.HASSAreaID != "living_room_2" {
		t.Errorf("Expected HASSAreaID 'living_room_2', got '%s'", livingRoom.HASSAreaID)
	}
	if *livingRoom.TransitionSeconds != 30 {
		t.Errorf("Expected TransitionSeconds 30, got %d", *livingRoom.TransitionSeconds)
	}

	// Check Primary Suite config
	primarySuite := config.Rooms[1]
	if primarySuite.HueGroup != "Primary Suite" {
		t.Errorf("Expected HueGroup 'Primary Suite', got '%s'", primarySuite.HueGroup)
	}
	if *primarySuite.TransitionSeconds != 180 {
		t.Errorf("Expected TransitionSeconds 180, got %d", *primarySuite.TransitionSeconds)
	}

	// Check Front of House config (with array conditions)
	frontOfHouse := config.Rooms[2]
	if frontOfHouse.HueGroup != "Front of House" {
		t.Errorf("Expected HueGroup 'Front of House', got '%s'", frontOfHouse.HueGroup)
	}
	if *frontOfHouse.TransitionSeconds != 120 {
		t.Errorf("Expected TransitionSeconds 120, got %d", *frontOfHouse.TransitionSeconds)
	}
}

func TestRoomConfigGetters(t *testing.T) {
	tests := []struct {
		name     string
		room     RoomConfig
		expected struct {
			onIfTrue                 []string
			onIfFalse                []string
			offIfTrue                []string
			offIfFalse               []string
			increaseBrightnessIfTrue []string
		}
	}{
		{
			name: "Single string conditions",
			room: RoomConfig{
				OnIfTrue:                 "isAnyoneHomeAndAwake",
				OnIfFalse:                "isTVPlaying",
				OffIfTrue:                "isEveryoneAsleep",
				OffIfFalse:               "isAnyoneHome",
				IncreaseBrightnessIfTrue: "isHaveGuests",
			},
			expected: struct {
				onIfTrue                 []string
				onIfFalse                []string
				offIfTrue                []string
				offIfFalse               []string
				increaseBrightnessIfTrue []string
			}{
				onIfTrue:                 []string{"isAnyoneHomeAndAwake"},
				onIfFalse:                []string{"isTVPlaying"},
				offIfTrue:                []string{"isEveryoneAsleep"},
				offIfFalse:               []string{"isAnyoneHome"},
				increaseBrightnessIfTrue: []string{"isHaveGuests"},
			},
		},
		{
			name: "Array conditions",
			room: RoomConfig{
				OnIfTrue:   []interface{}{"isHaveGuests", "didOwnerJustReturnHome"},
				OffIfFalse: []interface{}{"isHaveGuests", "didOwnerJustReturnHome"},
			},
			expected: struct {
				onIfTrue                 []string
				onIfFalse                []string
				offIfTrue                []string
				offIfFalse               []string
				increaseBrightnessIfTrue []string
			}{
				onIfTrue:                 []string{"isHaveGuests", "didOwnerJustReturnHome"},
				onIfFalse:                []string{},
				offIfTrue:                []string{},
				offIfFalse:               []string{"isHaveGuests", "didOwnerJustReturnHome"},
				increaseBrightnessIfTrue: []string{},
			},
		},
		{
			name: "Nil conditions",
			room: RoomConfig{
				OnIfTrue:  nil,
				OnIfFalse: nil,
			},
			expected: struct {
				onIfTrue                 []string
				onIfFalse                []string
				offIfTrue                []string
				offIfFalse               []string
				increaseBrightnessIfTrue []string
			}{
				onIfTrue:                 []string{},
				onIfFalse:                []string{},
				offIfTrue:                []string{},
				offIfFalse:               []string{},
				increaseBrightnessIfTrue: []string{},
			},
		},
		{
			name: "Empty string conditions",
			room: RoomConfig{
				OnIfTrue:  "",
				OnIfFalse: "",
			},
			expected: struct {
				onIfTrue                 []string
				onIfFalse                []string
				offIfTrue                []string
				offIfFalse               []string
				increaseBrightnessIfTrue []string
			}{
				onIfTrue:                 []string{},
				onIfFalse:                []string{},
				offIfTrue:                []string{},
				offIfFalse:               []string{},
				increaseBrightnessIfTrue: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onIfTrue := tt.room.GetOnIfTrueConditions()
			if !stringSlicesEqual(onIfTrue, tt.expected.onIfTrue) {
				t.Errorf("GetOnIfTrueConditions() = %v, want %v", onIfTrue, tt.expected.onIfTrue)
			}

			onIfFalse := tt.room.GetOnIfFalseConditions()
			if !stringSlicesEqual(onIfFalse, tt.expected.onIfFalse) {
				t.Errorf("GetOnIfFalseConditions() = %v, want %v", onIfFalse, tt.expected.onIfFalse)
			}

			offIfTrue := tt.room.GetOffIfTrueConditions()
			if !stringSlicesEqual(offIfTrue, tt.expected.offIfTrue) {
				t.Errorf("GetOffIfTrueConditions() = %v, want %v", offIfTrue, tt.expected.offIfTrue)
			}

			offIfFalse := tt.room.GetOffIfFalseConditions()
			if !stringSlicesEqual(offIfFalse, tt.expected.offIfFalse) {
				t.Errorf("GetOffIfFalseConditions() = %v, want %v", offIfFalse, tt.expected.offIfFalse)
			}

			increaseBrightness := tt.room.GetIncreaseBrightnessIfTrueConditions()
			if !stringSlicesEqual(increaseBrightness, tt.expected.increaseBrightnessIfTrue) {
				t.Errorf("GetIncreaseBrightnessIfTrueConditions() = %v, want %v", increaseBrightness, tt.expected.increaseBrightnessIfTrue)
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLoadConfigInvalidPath(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file, got nil")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `this is not: valid: yaml: content`
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}
