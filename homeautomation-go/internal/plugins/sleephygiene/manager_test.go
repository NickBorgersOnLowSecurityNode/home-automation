package sleephygiene

import (
	"testing"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, true)

	config := &ScheduleConfig{
		Schedule: []DaySchedule{
			{Name: "Sunday", BeginWake: "09:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Monday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Tuesday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Wednesday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Thursday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Friday", BeginWake: "08:50", StopScreens: "23:00", GoToBed: "23:59"},
			{Name: "Saturday", BeginWake: "09:50", StopScreens: "23:00", GoToBed: "23:59"},
		},
	}

	manager := NewManager(mockClient, stateManager, config, logger, true)
	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.readOnly != true {
		t.Errorf("Expected readOnly=true, got %v", manager.readOnly)
	}
}

func TestManagerStart(t *testing.T) {
	logger := zap.NewNop()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, true)

	// Sync state from mock HA
	if err := stateManager.SyncFromHA(); err != nil {
		t.Fatalf("Failed to sync state: %v", err)
	}

	config := &ScheduleConfig{
		Schedule: []DaySchedule{
			{Name: "Sunday", BeginWake: "09:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Monday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Tuesday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Wednesday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Thursday", BeginWake: "08:50", StopScreens: "22:30", GoToBed: "23:30"},
			{Name: "Friday", BeginWake: "08:50", StopScreens: "23:00", GoToBed: "23:59"},
			{Name: "Saturday", BeginWake: "09:50", StopScreens: "23:00", GoToBed: "23:59"},
		},
	}

	manager := NewManager(mockClient, stateManager, config, logger, true)

	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the manager
	manager.Stop()
}

func TestCheckMasterHomeAndAsleep(t *testing.T) {
	logger := zap.NewNop()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)

	// Sync state from mock HA
	if err := stateManager.SyncFromHA(); err != nil {
		t.Fatalf("Failed to sync state: %v", err)
	}

	config := &ScheduleConfig{Schedule: []DaySchedule{}}
	manager := NewManager(mockClient, stateManager, config, logger, true)

	// Test when no one is home
	stateManager.SetBool("isAnyoneHome", false)
	stateManager.SetBool("isMasterAsleep", false)

	if manager.checkMasterHomeAndAsleep() {
		t.Error("Expected false when no one is home")
	}

	// Test when home but not asleep
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", false)

	if manager.checkMasterHomeAndAsleep() {
		t.Error("Expected false when master not asleep")
	}

	// Test when home and asleep
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isMasterAsleep", true)

	if !manager.checkMasterHomeAndAsleep() {
		t.Error("Expected true when master is home and asleep")
	}
}

func TestCheckAnyoneHomeNotEveryoneAsleep(t *testing.T) {
	logger := zap.NewNop()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)

	// Sync state from mock HA
	if err := stateManager.SyncFromHA(); err != nil {
		t.Fatalf("Failed to sync state: %v", err)
	}

	config := &ScheduleConfig{Schedule: []DaySchedule{}}
	manager := NewManager(mockClient, stateManager, config, logger, true)

	// Test when no one is home
	stateManager.SetBool("isAnyoneHome", false)
	stateManager.SetBool("isEveryoneAsleep", false)

	if manager.checkAnyoneHomeNotEveryoneAsleep() {
		t.Error("Expected false when no one is home")
	}

	// Test when everyone is asleep
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isEveryoneAsleep", true)

	if manager.checkAnyoneHomeNotEveryoneAsleep() {
		t.Error("Expected false when everyone is asleep")
	}

	// Test when someone is home and not everyone is asleep
	stateManager.SetBool("isAnyoneHome", true)
	stateManager.SetBool("isEveryoneAsleep", false)

	if !manager.checkAnyoneHomeNotEveryoneAsleep() {
		t.Error("Expected true when someone is home and not everyone is asleep")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"Master Bedroom", "Bedroom", true},
		{"Guest Bedroom", "Bedroom", true},
		{"Living Room", "Bedroom", false},
		{"Kitchen", "Bedroom", false},
		{"", "", true},
		{"test", "", true},
		{"", "test", false},
	}

	for _, tt := range tests {
		got := contains(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestSanitizeEntityName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Master Bedroom", "Master_Bedroom"},
		{"Living Room", "Living_Room"},
		{"kitchen", "kitchen"},
		{"Test123", "Test123"},
	}

	for _, tt := range tests {
		got := sanitizeEntityName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeEntityName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
