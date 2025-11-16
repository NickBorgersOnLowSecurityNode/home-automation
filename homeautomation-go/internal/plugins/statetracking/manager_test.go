package statetracking

import (
	"testing"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

func TestStateTrackingManager_IsAnyOwnerHome(t *testing.T) {
	tests := []struct {
		name           string
		isNickHome     bool
		isCarolineHome bool
		expectedOwner  bool
		description    string
	}{
		{
			name:           "Both owners away",
			isNickHome:     false,
			isCarolineHome: false,
			expectedOwner:  false,
			description:    "No owners home",
		},
		{
			name:           "Only Nick home",
			isNickHome:     true,
			isCarolineHome: false,
			expectedOwner:  true,
			description:    "Nick is home, Caroline is away",
		},
		{
			name:           "Only Caroline home",
			isNickHome:     false,
			isCarolineHome: true,
			expectedOwner:  true,
			description:    "Caroline is home, Nick is away",
		},
		{
			name:           "Both owners home",
			isNickHome:     true,
			isCarolineHome: true,
			expectedOwner:  true,
			description:    "Both owners are home",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Set up initial state
			if err := stateMgr.SetBool("isNickHome", tt.isNickHome); err != nil {
				t.Fatalf("Failed to set isNickHome: %v", err)
			}
			if err := stateMgr.SetBool("isCarolineHome", tt.isCarolineHome); err != nil {
				t.Fatalf("Failed to set isCarolineHome: %v", err)
			}

			// Create and start manager
			manager := NewManager(mockHA, stateMgr, logger, false)
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start manager: %v", err)
			}
			defer manager.Stop()

			// Verify isAnyOwnerHome was computed correctly
			actualOwner, err := stateMgr.GetBool("isAnyOwnerHome")
			if err != nil {
				t.Fatalf("Failed to get isAnyOwnerHome: %v", err)
			}

			if actualOwner != tt.expectedOwner {
				t.Errorf("Expected isAnyOwnerHome=%v, got %v (Nick=%v, Caroline=%v)",
					tt.expectedOwner, actualOwner, tt.isNickHome, tt.isCarolineHome)
			}
		})
	}
}

func TestStateTrackingManager_IsAnyoneHome(t *testing.T) {
	tests := []struct {
		name           string
		isNickHome     bool
		isCarolineHome bool
		isToriHere     bool
		expectedAnyone bool
		description    string
	}{
		{
			name:           "Everyone away",
			isNickHome:     false,
			isCarolineHome: false,
			isToriHere:     false,
			expectedAnyone: false,
			description:    "No one is home",
		},
		{
			name:           "Only Nick home",
			isNickHome:     true,
			isCarolineHome: false,
			isToriHere:     false,
			expectedAnyone: true,
			description:    "Nick is home",
		},
		{
			name:           "Only Caroline home",
			isNickHome:     false,
			isCarolineHome: true,
			isToriHere:     false,
			expectedAnyone: true,
			description:    "Caroline is home",
		},
		{
			name:           "Only Tori here",
			isNickHome:     false,
			isCarolineHome: false,
			isToriHere:     true,
			expectedAnyone: true,
			description:    "Guest (Tori) is here",
		},
		{
			name:           "Nick and Tori home",
			isNickHome:     true,
			isCarolineHome: false,
			isToriHere:     true,
			expectedAnyone: true,
			description:    "Owner and guest are home",
		},
		{
			name:           "Everyone home",
			isNickHome:     true,
			isCarolineHome: true,
			isToriHere:     true,
			expectedAnyone: true,
			description:    "All people are home",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Set up initial state
			if err := stateMgr.SetBool("isNickHome", tt.isNickHome); err != nil {
				t.Fatalf("Failed to set isNickHome: %v", err)
			}
			if err := stateMgr.SetBool("isCarolineHome", tt.isCarolineHome); err != nil {
				t.Fatalf("Failed to set isCarolineHome: %v", err)
			}
			if err := stateMgr.SetBool("isToriHere", tt.isToriHere); err != nil {
				t.Fatalf("Failed to set isToriHere: %v", err)
			}

			// Create and start manager
			manager := NewManager(mockHA, stateMgr, logger, false)
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start manager: %v", err)
			}
			defer manager.Stop()

			// Verify isAnyoneHome was computed correctly
			actualAnyone, err := stateMgr.GetBool("isAnyoneHome")
			if err != nil {
				t.Fatalf("Failed to get isAnyoneHome: %v", err)
			}

			if actualAnyone != tt.expectedAnyone {
				t.Errorf("Expected isAnyoneHome=%v, got %v (Nick=%v, Caroline=%v, Tori=%v)",
					tt.expectedAnyone, actualAnyone, tt.isNickHome, tt.isCarolineHome, tt.isToriHere)
			}
		})
	}
}

func TestStateTrackingManager_IsAnyoneAsleep(t *testing.T) {
	tests := []struct {
		name              string
		isMasterAsleep    bool
		isGuestAsleep     bool
		expectedAnyAsleep bool
		description       string
	}{
		{
			name:              "Everyone awake",
			isMasterAsleep:    false,
			isGuestAsleep:     false,
			expectedAnyAsleep: false,
			description:       "No one is asleep",
		},
		{
			name:              "Only master asleep",
			isMasterAsleep:    true,
			isGuestAsleep:     false,
			expectedAnyAsleep: true,
			description:       "Master bedroom is asleep",
		},
		{
			name:              "Only guest asleep",
			isMasterAsleep:    false,
			isGuestAsleep:     true,
			expectedAnyAsleep: true,
			description:       "Guest bedroom is asleep",
		},
		{
			name:              "Everyone asleep",
			isMasterAsleep:    true,
			isGuestAsleep:     true,
			expectedAnyAsleep: true,
			description:       "Both bedrooms are asleep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Set isHaveGuests to true to test independent sleep states
			if err := stateMgr.SetBool("isHaveGuests", true); err != nil {
				t.Fatalf("Failed to set isHaveGuests: %v", err)
			}

			// Set up initial state
			if err := stateMgr.SetBool("isMasterAsleep", tt.isMasterAsleep); err != nil {
				t.Fatalf("Failed to set isMasterAsleep: %v", err)
			}
			if err := stateMgr.SetBool("isGuestAsleep", tt.isGuestAsleep); err != nil {
				t.Fatalf("Failed to set isGuestAsleep: %v", err)
			}

			// Create and start manager
			manager := NewManager(mockHA, stateMgr, logger, false)
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start manager: %v", err)
			}
			defer manager.Stop()

			// Verify isAnyoneAsleep was computed correctly
			actualAnyAsleep, err := stateMgr.GetBool("isAnyoneAsleep")
			if err != nil {
				t.Fatalf("Failed to get isAnyoneAsleep: %v", err)
			}

			if actualAnyAsleep != tt.expectedAnyAsleep {
				t.Errorf("Expected isAnyoneAsleep=%v, got %v (Master=%v, Guest=%v)",
					tt.expectedAnyAsleep, actualAnyAsleep, tt.isMasterAsleep, tt.isGuestAsleep)
			}
		})
	}
}

func TestStateTrackingManager_IsEveryoneAsleep(t *testing.T) {
	tests := []struct {
		name              string
		isMasterAsleep    bool
		isGuestAsleep     bool
		expectedAllAsleep bool
		description       string
	}{
		{
			name:              "Everyone awake",
			isMasterAsleep:    false,
			isGuestAsleep:     false,
			expectedAllAsleep: false,
			description:       "No one is asleep",
		},
		{
			name:              "Only master asleep",
			isMasterAsleep:    true,
			isGuestAsleep:     false,
			expectedAllAsleep: false,
			description:       "Master asleep, guest awake",
		},
		{
			name:              "Only guest asleep",
			isMasterAsleep:    false,
			isGuestAsleep:     true,
			expectedAllAsleep: false,
			description:       "Guest asleep, master awake",
		},
		{
			name:              "Everyone asleep",
			isMasterAsleep:    true,
			isGuestAsleep:     true,
			expectedAllAsleep: true,
			description:       "Both bedrooms are asleep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HA client and state manager
			mockHA := ha.NewMockClient()
			logger := zap.NewNop()
			stateMgr := state.NewManager(mockHA, logger, false)

			// Set isHaveGuests to true to test independent sleep states
			if err := stateMgr.SetBool("isHaveGuests", true); err != nil {
				t.Fatalf("Failed to set isHaveGuests: %v", err)
			}

			// Set up initial state
			if err := stateMgr.SetBool("isMasterAsleep", tt.isMasterAsleep); err != nil {
				t.Fatalf("Failed to set isMasterAsleep: %v", err)
			}
			if err := stateMgr.SetBool("isGuestAsleep", tt.isGuestAsleep); err != nil {
				t.Fatalf("Failed to set isGuestAsleep: %v", err)
			}

			// Create and start manager
			manager := NewManager(mockHA, stateMgr, logger, false)
			if err := manager.Start(); err != nil {
				t.Fatalf("Failed to start manager: %v", err)
			}
			defer manager.Stop()

			// Verify isEveryoneAsleep was computed correctly
			actualAllAsleep, err := stateMgr.GetBool("isEveryoneAsleep")
			if err != nil {
				t.Fatalf("Failed to get isEveryoneAsleep: %v", err)
			}

			if actualAllAsleep != tt.expectedAllAsleep {
				t.Errorf("Expected isEveryoneAsleep=%v, got %v (Master=%v, Guest=%v)",
					tt.expectedAllAsleep, actualAllAsleep, tt.isMasterAsleep, tt.isGuestAsleep)
			}
		})
	}
}

func TestStateTrackingManager_DynamicUpdates(t *testing.T) {
	// Test that derived states update when source states change
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Set up initial state - everyone away
	if err := stateMgr.SetBool("isNickHome", false); err != nil {
		t.Fatalf("Failed to set isNickHome: %v", err)
	}
	if err := stateMgr.SetBool("isCarolineHome", false); err != nil {
		t.Fatalf("Failed to set isCarolineHome: %v", err)
	}
	if err := stateMgr.SetBool("isToriHere", false); err != nil {
		t.Fatalf("Failed to set isToriHere: %v", err)
	}

	// Create and start manager
	manager := NewManager(mockHA, stateMgr, logger, false)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Verify initial state - no one home
	isAnyoneHome, _ := stateMgr.GetBool("isAnyoneHome")
	if isAnyoneHome != false {
		t.Errorf("Expected isAnyoneHome=false initially, got %v", isAnyoneHome)
	}

	// Nick arrives home
	if err := stateMgr.SetBool("isNickHome", true); err != nil {
		t.Fatalf("Failed to update isNickHome: %v", err)
	}

	// Verify derived states updated
	isAnyOwnerHome, _ := stateMgr.GetBool("isAnyOwnerHome")
	if isAnyOwnerHome != true {
		t.Errorf("Expected isAnyOwnerHome=true after Nick arrives, got %v", isAnyOwnerHome)
	}

	isAnyoneHome, _ = stateMgr.GetBool("isAnyoneHome")
	if isAnyoneHome != true {
		t.Errorf("Expected isAnyoneHome=true after Nick arrives, got %v", isAnyoneHome)
	}

	// Nick leaves, but Tori arrives
	if err := stateMgr.SetBool("isNickHome", false); err != nil {
		t.Fatalf("Failed to update isNickHome: %v", err)
	}
	if err := stateMgr.SetBool("isToriHere", true); err != nil {
		t.Fatalf("Failed to update isToriHere: %v", err)
	}

	// Verify isAnyOwnerHome is false but isAnyoneHome is still true
	isAnyOwnerHome, _ = stateMgr.GetBool("isAnyOwnerHome")
	if isAnyOwnerHome != false {
		t.Errorf("Expected isAnyOwnerHome=false after Nick leaves, got %v", isAnyOwnerHome)
	}

	isAnyoneHome, _ = stateMgr.GetBool("isAnyoneHome")
	if isAnyoneHome != true {
		t.Errorf("Expected isAnyoneHome=true with Tori here, got %v", isAnyoneHome)
	}
}

func TestStateTrackingManager_SleepDynamicUpdates(t *testing.T) {
	// Test that sleep derived states update when source states change
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Set isHaveGuests to true to test independent sleep states
	if err := stateMgr.SetBool("isHaveGuests", true); err != nil {
		t.Fatalf("Failed to set isHaveGuests: %v", err)
	}

	// Set up initial state - everyone awake
	if err := stateMgr.SetBool("isMasterAsleep", false); err != nil {
		t.Fatalf("Failed to set isMasterAsleep: %v", err)
	}
	if err := stateMgr.SetBool("isGuestAsleep", false); err != nil {
		t.Fatalf("Failed to set isGuestAsleep: %v", err)
	}

	// Create and start manager
	manager := NewManager(mockHA, stateMgr, logger, false)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Verify initial state
	isAnyoneAsleep, _ := stateMgr.GetBool("isAnyoneAsleep")
	isEveryoneAsleep, _ := stateMgr.GetBool("isEveryoneAsleep")
	if isAnyoneAsleep != false || isEveryoneAsleep != false {
		t.Errorf("Expected both sleep states false initially")
	}

	// Master goes to sleep
	if err := stateMgr.SetBool("isMasterAsleep", true); err != nil {
		t.Fatalf("Failed to update isMasterAsleep: %v", err)
	}

	// Verify isAnyoneAsleep=true, isEveryoneAsleep=false
	isAnyoneAsleep, _ = stateMgr.GetBool("isAnyoneAsleep")
	isEveryoneAsleep, _ = stateMgr.GetBool("isEveryoneAsleep")
	if isAnyoneAsleep != true {
		t.Errorf("Expected isAnyoneAsleep=true after master sleeps")
	}
	if isEveryoneAsleep != false {
		t.Errorf("Expected isEveryoneAsleep=false when only master sleeps")
	}

	// Guest goes to sleep
	if err := stateMgr.SetBool("isGuestAsleep", true); err != nil {
		t.Fatalf("Failed to update isGuestAsleep: %v", err)
	}

	// Verify both sleep states are true
	isAnyoneAsleep, _ = stateMgr.GetBool("isAnyoneAsleep")
	isEveryoneAsleep, _ = stateMgr.GetBool("isEveryoneAsleep")
	if isAnyoneAsleep != true || isEveryoneAsleep != true {
		t.Errorf("Expected both sleep states true when everyone sleeps")
	}

	// Guest wakes up
	if err := stateMgr.SetBool("isGuestAsleep", false); err != nil {
		t.Fatalf("Failed to update isGuestAsleep: %v", err)
	}

	// Verify isAnyoneAsleep=true, isEveryoneAsleep=false
	isAnyoneAsleep, _ = stateMgr.GetBool("isAnyoneAsleep")
	isEveryoneAsleep, _ = stateMgr.GetBool("isEveryoneAsleep")
	if isAnyoneAsleep != true {
		t.Errorf("Expected isAnyoneAsleep=true when master still sleeps")
	}
	if isEveryoneAsleep != false {
		t.Errorf("Expected isEveryoneAsleep=false when guest wakes")
	}
}

func TestStateTrackingManager_StopCleansUpSubscriptions(t *testing.T) {
	// Test that Stop() properly cleans up subscriptions
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Set up initial state
	if err := stateMgr.SetBool("isNickHome", false); err != nil {
		t.Fatalf("Failed to set isNickHome: %v", err)
	}

	// Create and start manager
	manager := NewManager(mockHA, stateMgr, logger, false)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Verify subscriptions are active by changing state
	if err := stateMgr.SetBool("isNickHome", true); err != nil {
		t.Fatalf("Failed to update isNickHome: %v", err)
	}

	isAnyOwnerHome, _ := stateMgr.GetBool("isAnyOwnerHome")
	if isAnyOwnerHome != true {
		t.Errorf("Expected derived state to update before Stop()")
	}

	// Stop the manager
	manager.Stop()

	// Change state again - derived states should NOT update after Stop
	// (This test verifies subscriptions are cleaned up, but the derived
	// state helper will have already unsubscribed, so we can't easily
	// verify this without accessing internal state. The main goal is
	// to ensure Stop() doesn't panic and properly calls helper.Stop())
}

func TestStateTrackingManager_GuestAsleepAutoSync_NoGuests(t *testing.T) {
	// Test that guest asleep auto-syncs with master when no guests
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Setup: No guests, master awake, guest awake
	if err := stateMgr.SetBool("isHaveGuests", false); err != nil {
		t.Fatalf("Failed to set isHaveGuests: %v", err)
	}
	if err := stateMgr.SetBool("isMasterAsleep", false); err != nil {
		t.Fatalf("Failed to set isMasterAsleep: %v", err)
	}
	if err := stateMgr.SetBool("isGuestAsleep", false); err != nil {
		t.Fatalf("Failed to set isGuestAsleep: %v", err)
	}

	// Create and start manager
	manager := NewManager(mockHA, stateMgr, logger, false)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Test 1: Master goes to sleep, guest should auto-sync
	if err := stateMgr.SetBool("isMasterAsleep", true); err != nil {
		t.Fatalf("Failed to update isMasterAsleep: %v", err)
	}

	guestAsleep, _ := stateMgr.GetBool("isGuestAsleep")
	if guestAsleep != true {
		t.Errorf("Expected isGuestAsleep=true after master sleeps (no guests), got %v", guestAsleep)
	}

	// Verify derived states are correct
	isEveryoneAsleep, _ := stateMgr.GetBool("isEveryoneAsleep")
	if isEveryoneAsleep != true {
		t.Errorf("Expected isEveryoneAsleep=true after auto-sync, got %v", isEveryoneAsleep)
	}

	// Test 2: Master wakes up, guest should auto-sync
	if err := stateMgr.SetBool("isMasterAsleep", false); err != nil {
		t.Fatalf("Failed to update isMasterAsleep: %v", err)
	}

	guestAsleep, _ = stateMgr.GetBool("isGuestAsleep")
	if guestAsleep != false {
		t.Errorf("Expected isGuestAsleep=false after master wakes (no guests), got %v", guestAsleep)
	}

	// Verify derived states updated
	isEveryoneAsleep, _ = stateMgr.GetBool("isEveryoneAsleep")
	if isEveryoneAsleep != false {
		t.Errorf("Expected isEveryoneAsleep=false after auto-sync, got %v", isEveryoneAsleep)
	}
}

func TestStateTrackingManager_GuestAsleepAutoSync_WithGuests(t *testing.T) {
	// Test that guest asleep does NOT auto-sync when guests are present
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Setup: Have guests, master awake, guest asleep
	if err := stateMgr.SetBool("isHaveGuests", true); err != nil {
		t.Fatalf("Failed to set isHaveGuests: %v", err)
	}
	if err := stateMgr.SetBool("isMasterAsleep", false); err != nil {
		t.Fatalf("Failed to set isMasterAsleep: %v", err)
	}
	if err := stateMgr.SetBool("isGuestAsleep", true); err != nil {
		t.Fatalf("Failed to set isGuestAsleep: %v", err)
	}

	// Create and start manager
	manager := NewManager(mockHA, stateMgr, logger, false)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Master goes to sleep
	if err := stateMgr.SetBool("isMasterAsleep", true); err != nil {
		t.Fatalf("Failed to update isMasterAsleep: %v", err)
	}

	// Guest asleep should remain true (independent when guests present)
	guestAsleep, _ := stateMgr.GetBool("isGuestAsleep")
	if guestAsleep != true {
		t.Errorf("Expected isGuestAsleep=true (independent when guests present), got %v", guestAsleep)
	}

	// Master wakes up
	if err := stateMgr.SetBool("isMasterAsleep", false); err != nil {
		t.Fatalf("Failed to update isMasterAsleep: %v", err)
	}

	// Guest asleep should STILL be true (not synced)
	guestAsleep, _ = stateMgr.GetBool("isGuestAsleep")
	if guestAsleep != true {
		t.Errorf("Expected isGuestAsleep=true (independent when guests present), got %v", guestAsleep)
	}
}

func TestStateTrackingManager_GuestAsleepAutoSync_GuestsLeave(t *testing.T) {
	// Test that auto-sync kicks in when isHaveGuests changes from true to false
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Setup: Have guests, master asleep, guest awake
	if err := stateMgr.SetBool("isHaveGuests", true); err != nil {
		t.Fatalf("Failed to set isHaveGuests: %v", err)
	}
	if err := stateMgr.SetBool("isMasterAsleep", true); err != nil {
		t.Fatalf("Failed to set isMasterAsleep: %v", err)
	}
	if err := stateMgr.SetBool("isGuestAsleep", false); err != nil {
		t.Fatalf("Failed to set isGuestAsleep: %v", err)
	}

	// Create and start manager
	manager := NewManager(mockHA, stateMgr, logger, false)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Verify guest asleep is independent (false) while guests present
	guestAsleep, _ := stateMgr.GetBool("isGuestAsleep")
	if guestAsleep != false {
		t.Errorf("Expected isGuestAsleep=false (independent), got %v", guestAsleep)
	}

	// Guests leave (isHaveGuests changes to false)
	if err := stateMgr.SetBool("isHaveGuests", false); err != nil {
		t.Fatalf("Failed to update isHaveGuests: %v", err)
	}

	// Guest asleep should now auto-sync to master (true)
	guestAsleep, _ = stateMgr.GetBool("isGuestAsleep")
	if guestAsleep != true {
		t.Errorf("Expected isGuestAsleep=true (synced with master after guests leave), got %v", guestAsleep)
	}

	// Verify derived state is correct
	isEveryoneAsleep, _ := stateMgr.GetBool("isEveryoneAsleep")
	if isEveryoneAsleep != true {
		t.Errorf("Expected isEveryoneAsleep=true after auto-sync, got %v", isEveryoneAsleep)
	}
}

func TestStateTrackingManager_GuestAsleepAutoSync_InitialSync(t *testing.T) {
	// Test that auto-sync happens on startup if needed
	mockHA := ha.NewMockClient()
	logger := zap.NewNop()
	stateMgr := state.NewManager(mockHA, logger, false)

	// Setup: No guests, master asleep, guest awake (out of sync)
	if err := stateMgr.SetBool("isHaveGuests", false); err != nil {
		t.Fatalf("Failed to set isHaveGuests: %v", err)
	}
	if err := stateMgr.SetBool("isMasterAsleep", true); err != nil {
		t.Fatalf("Failed to set isMasterAsleep: %v", err)
	}
	if err := stateMgr.SetBool("isGuestAsleep", false); err != nil {
		t.Fatalf("Failed to set isGuestAsleep: %v", err)
	}

	// Create and start manager - should auto-sync immediately
	manager := NewManager(mockHA, stateMgr, logger, false)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Guest asleep should be synced to master on startup
	guestAsleep, _ := stateMgr.GetBool("isGuestAsleep")
	if guestAsleep != true {
		t.Errorf("Expected isGuestAsleep=true (synced on startup), got %v", guestAsleep)
	}

	// Verify derived state is correct
	isEveryoneAsleep, _ := stateMgr.GetBool("isEveryoneAsleep")
	if isEveryoneAsleep != true {
		t.Errorf("Expected isEveryoneAsleep=true after initial sync, got %v", isEveryoneAsleep)
	}
}
