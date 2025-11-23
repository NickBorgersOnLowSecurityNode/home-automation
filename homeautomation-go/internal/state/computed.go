package state

import "go.uber.org/zap"

// SetupComputedState initializes computed state variables and sets up
// subscriptions to automatically recompute them when dependencies change.
//
// Computed state variables are derived from other state variables:
// - isAnyoneHomeAndAwake = isAnyoneHome && !isAnyoneAsleep
func (m *Manager) SetupComputedState() error {
	// Compute initial value
	if err := m.recomputeAnyoneHomeAndAwake(); err != nil {
		return err
	}

	// Subscribe to dependency changes
	_, err := m.Subscribe("isAnyoneHome", func(key string, oldValue, newValue interface{}) {
		if err := m.recomputeAnyoneHomeAndAwake(); err != nil {
			m.logger.Error("Failed to recompute isAnyoneHomeAndAwake",
				zap.String("trigger", key),
				zap.Error(err))
		}
	})
	if err != nil {
		return err
	}

	_, err = m.Subscribe("isAnyoneAsleep", func(key string, oldValue, newValue interface{}) {
		if err := m.recomputeAnyoneHomeAndAwake(); err != nil {
			m.logger.Error("Failed to recompute isAnyoneHomeAndAwake",
				zap.String("trigger", key),
				zap.Error(err))
		}
	})
	if err != nil {
		return err
	}

	m.logger.Info("Computed state initialized",
		zap.Strings("variables", []string{"isAnyoneHomeAndAwake"}))

	return nil
}

// recomputeAnyoneHomeAndAwake computes isAnyoneHomeAndAwake from its dependencies.
// Formula: isAnyoneHomeAndAwake = isAnyoneHome && !isAnyoneAsleep
func (m *Manager) recomputeAnyoneHomeAndAwake() error {
	isAnyoneHome, err := m.GetBool("isAnyoneHome")
	if err != nil {
		return err
	}

	isAnyoneAsleep, err := m.GetBool("isAnyoneAsleep")
	if err != nil {
		return err
	}

	newValue := isAnyoneHome && !isAnyoneAsleep

	// Get current value to check if it changed
	currentValue, _ := m.GetBool("isAnyoneHomeAndAwake")
	if currentValue != newValue {
		m.logger.Debug("Recomputing isAnyoneHomeAndAwake",
			zap.Bool("isAnyoneHome", isAnyoneHome),
			zap.Bool("isAnyoneAsleep", isAnyoneAsleep),
			zap.Bool("result", newValue))
	}

	return m.SetBool("isAnyoneHomeAndAwake", newValue)
}
