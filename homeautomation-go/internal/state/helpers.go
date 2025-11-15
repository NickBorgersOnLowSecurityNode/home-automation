package state

import (
	"go.uber.org/zap"
)

// Helper functions for derived state computation
// These implement the logic from Node-RED's State Tracking tab

// DerivedStateHelper manages automatic computation of derived states
type DerivedStateHelper struct {
	manager *Manager
	logger  *zap.Logger
	subs    []Subscription
}

// NewDerivedStateHelper creates a new helper for managing derived states
func NewDerivedStateHelper(manager *Manager, logger *zap.Logger) *DerivedStateHelper {
	return &DerivedStateHelper{
		manager: manager,
		logger:  logger,
		subs:    make([]Subscription, 0),
	}
}

// Start begins monitoring and updating derived states
func (h *DerivedStateHelper) Start() error {
	h.logger.Info("Starting derived state helper")

	// Subscribe to presence changes to update isAnyoneHome
	if err := h.setupPresenceTracking(); err != nil {
		return err
	}

	// Subscribe to sleep state changes to update isEveryoneAsleep
	if err := h.setupSleepTracking(); err != nil {
		return err
	}

	// Subscribe for auto-sleep detection
	if err := h.setupAutoSleepDetection(); err != nil {
		return err
	}

	// Initialize derived states immediately
	h.updateIsAnyoneHome()
	h.updateIsEveryoneAsleep()

	h.logger.Info("Derived state helper started")
	return nil
}

// Stop unsubscribes from all state changes
func (h *DerivedStateHelper) Stop() {
	h.logger.Info("Stopping derived state helper")
	for _, sub := range h.subs {
		sub.Unsubscribe()
	}
	h.subs = nil
}

// setupPresenceTracking subscribes to presence changes and updates isAnyoneHome
func (h *DerivedStateHelper) setupPresenceTracking() error {
	// Subscribe to Nick's presence
	sub1, err := h.manager.Subscribe("isNickHome", func(key string, oldValue, newValue interface{}) {
		h.logger.Debug("Nick's presence changed", zap.Any("old", oldValue), zap.Any("new", newValue))
		h.updateIsAnyoneHome()
	})
	if err != nil {
		return err
	}
	h.subs = append(h.subs, sub1)

	// Subscribe to Caroline's presence
	sub2, err := h.manager.Subscribe("isCarolineHome", func(key string, oldValue, newValue interface{}) {
		h.logger.Debug("Caroline's presence changed", zap.Any("old", oldValue), zap.Any("new", newValue))
		h.updateIsAnyoneHome()
	})
	if err != nil {
		return err
	}
	h.subs = append(h.subs, sub2)

	return nil
}

// setupSleepTracking subscribes to sleep state changes and updates isEveryoneAsleep
func (h *DerivedStateHelper) setupSleepTracking() error {
	// Subscribe to master bedroom sleep state
	sub1, err := h.manager.Subscribe("isMasterAsleep", func(key string, oldValue, newValue interface{}) {
		h.logger.Debug("Master sleep state changed", zap.Any("old", oldValue), zap.Any("new", newValue))
		h.updateIsEveryoneAsleep()
	})
	if err != nil {
		return err
	}
	h.subs = append(h.subs, sub1)

	// Subscribe to guest bedroom sleep state
	sub2, err := h.manager.Subscribe("isGuestAsleep", func(key string, oldValue, newValue interface{}) {
		h.logger.Debug("Guest sleep state changed", zap.Any("old", oldValue), zap.Any("new", newValue))
		h.updateIsEveryoneAsleep()
	})
	if err != nil {
		return err
	}
	h.subs = append(h.subs, sub2)

	return nil
}

// setupAutoSleepDetection implements automatic guest sleep detection
// Logic: Guest falls asleep when door closes if:
// - Someone is home
// - Guest not already marked asleep
// - Have guests
// - Guest bedroom door is closed
func (h *DerivedStateHelper) setupAutoSleepDetection() error {
	// Subscribe to guest bedroom door state
	sub, err := h.manager.Subscribe("isGuestBedroomDoorOpen", func(key string, oldValue, newValue interface{}) {
		h.logger.Debug("Guest bedroom door state changed", zap.Any("old", oldValue), zap.Any("new", newValue))

		// Door just closed (was open, now closed)
		wasOpen, wasOpenOk := oldValue.(bool)
		isClosed, isClosedOk := newValue.(bool)

		if wasOpenOk && isClosedOk && wasOpen && !isClosed {
			h.checkAutoGuestSleep()
		}
	})
	if err != nil {
		return err
	}
	h.subs = append(h.subs, sub)

	return nil
}

// updateIsAnyoneHome computes: isAnyoneHome = isNickHome OR isCarolineHome
func (h *DerivedStateHelper) updateIsAnyoneHome() {
	isNickHome, err1 := h.manager.GetBool("isNickHome")
	isCarolineHome, err2 := h.manager.GetBool("isCarolineHome")

	if err1 != nil || err2 != nil {
		h.logger.Error("Failed to get presence states",
			zap.Error(err1),
			zap.Error(err2))
		return
	}

	isAnyoneHome := isNickHome || isCarolineHome

	// Get current value to check if it changed
	currentValue, _ := h.manager.GetBool("isAnyoneHome")
	if currentValue == isAnyoneHome {
		return // No change
	}

	if err := h.manager.SetBool("isAnyoneHome", isAnyoneHome); err != nil {
		h.logger.Error("Failed to update isAnyoneHome", zap.Error(err))
		return
	}

	h.logger.Info("Updated isAnyoneHome",
		zap.Bool("isNickHome", isNickHome),
		zap.Bool("isCarolineHome", isCarolineHome),
		zap.Bool("isAnyoneHome", isAnyoneHome))
}

// updateIsEveryoneAsleep computes: isEveryoneAsleep = isMasterAsleep AND isGuestAsleep
func (h *DerivedStateHelper) updateIsEveryoneAsleep() {
	isMasterAsleep, err1 := h.manager.GetBool("isMasterAsleep")
	isGuestAsleep, err2 := h.manager.GetBool("isGuestAsleep")

	if err1 != nil || err2 != nil {
		h.logger.Error("Failed to get sleep states",
			zap.Error(err1),
			zap.Error(err2))
		return
	}

	isEveryoneAsleep := isMasterAsleep && isGuestAsleep

	// Get current value to check if it changed
	currentValue, _ := h.manager.GetBool("isEveryoneAsleep")
	if currentValue == isEveryoneAsleep {
		return // No change
	}

	if err := h.manager.SetBool("isEveryoneAsleep", isEveryoneAsleep); err != nil {
		h.logger.Error("Failed to update isEveryoneAsleep", zap.Error(err))
		return
	}

	h.logger.Info("Updated isEveryoneAsleep",
		zap.Bool("isMasterAsleep", isMasterAsleep),
		zap.Bool("isGuestAsleep", isGuestAsleep),
		zap.Bool("isEveryoneAsleep", isEveryoneAsleep))
}

// checkAutoGuestSleep checks if guest should be automatically marked as asleep
func (h *DerivedStateHelper) checkAutoGuestSleep() {
	// Applicability checks
	isAnyoneHome, _ := h.manager.GetBool("isAnyoneHome")
	if !isAnyoneHome {
		h.logger.Debug("Auto-sleep: No one home")
		return
	}

	isGuestAsleep, _ := h.manager.GetBool("isGuestAsleep")
	if isGuestAsleep {
		h.logger.Debug("Auto-sleep: Guest already marked asleep")
		return
	}

	isHaveGuests, _ := h.manager.GetBool("isHaveGuests")
	if !isHaveGuests {
		h.logger.Debug("Auto-sleep: No guests present")
		return
	}

	isGuestBedroomDoorOpen, _ := h.manager.GetBool("isGuestBedroomDoorOpen")
	if isGuestBedroomDoorOpen {
		h.logger.Debug("Auto-sleep: Guest bedroom door is open")
		return
	}

	// All conditions met - mark guest as asleep
	h.logger.Info("Auto-detecting guest sleep (door closed)")
	if err := h.manager.SetBool("isGuestAsleep", true); err != nil {
		h.logger.Error("Failed to auto-set guest asleep", zap.Error(err))
	}
}
