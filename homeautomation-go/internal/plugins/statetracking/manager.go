package statetracking

import (
	"fmt"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Manager handles automatic computation of derived state variables.
// This plugin implements the logic from Node-RED's "State Tracking" flow.
//
// Derived states computed:
//   - isAnyOwnerHome = isNickHome OR isCarolineHome
//   - isAnyoneHome = isAnyOwnerHome OR isToriHere
//   - isAnyoneAsleep = isMasterAsleep OR isGuestAsleep
//   - isEveryoneAsleep = isMasterAsleep AND isGuestAsleep
//
// Additional features:
//   - Automatic guest sleep detection when guest bedroom door closes
type Manager struct {
	stateManager *state.Manager
	logger       *zap.Logger
	helper       *state.DerivedStateHelper
}

// NewManager creates a new State Tracking manager
func NewManager(stateManager *state.Manager, logger *zap.Logger) *Manager {
	return &Manager{
		stateManager: stateManager,
		logger:       logger.Named("statetracking"),
	}
}

// Start begins computing and maintaining derived states.
// This must be called before other plugins that depend on derived states (Music, Security).
func (m *Manager) Start() error {
	m.logger.Info("Starting State Tracking Manager")

	// Create and start the derived state helper
	m.helper = state.NewDerivedStateHelper(m.stateManager, m.logger)
	if err := m.helper.Start(); err != nil {
		return fmt.Errorf("failed to start derived state helper: %w", err)
	}

	m.logger.Info("State Tracking Manager started successfully",
		zap.Strings("derivedStates", []string{
			"isAnyOwnerHome",
			"isAnyoneHome",
			"isAnyoneAsleep",
			"isEveryoneAsleep",
		}))
	return nil
}

// Stop stops the State Tracking Manager and cleans up subscriptions
func (m *Manager) Stop() {
	m.logger.Info("Stopping State Tracking Manager")
	if m.helper != nil {
		m.helper.Stop()
	}
	m.logger.Info("State Tracking Manager stopped")
}
