package statetracking

import (
	"fmt"
	"time"

	"homeautomation/internal/config"
	"homeautomation/internal/dayphase"
	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Manager handles state tracking for sun events and day phase calculation
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	configLoader *config.Loader
	calculator   *dayphase.Calculator
	logger       *zap.Logger
	readOnly     bool

	// Control channels
	stopChan     chan struct{}
	stoppedChan  chan struct{}
	sunStopChan  chan struct{}

	// Subscriptions for cleanup
	subscriptions []state.Subscription
}

// NewManager creates a new State Tracking manager
func NewManager(
	haClient ha.HAClient,
	stateManager *state.Manager,
	configLoader *config.Loader,
	calculator *dayphase.Calculator,
	logger *zap.Logger,
	readOnly bool,
) *Manager {
	return &Manager{
		haClient:      haClient,
		stateManager:  stateManager,
		configLoader:  configLoader,
		calculator:    calculator,
		logger:        logger.Named("statetracking"),
		readOnly:      readOnly,
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
		subscriptions: make([]state.Subscription, 0),
	}
}

// Start begins monitoring and updating state tracking variables
func (m *Manager) Start() error {
	m.logger.Info("Starting State Tracking Manager")

	// Start periodic sun time updates (every 6 hours)
	m.sunStopChan = m.calculator.StartPeriodicUpdate()

	// Do initial calculation and update
	if err := m.updateSunEventAndDayPhase(); err != nil {
		return fmt.Errorf("failed to do initial state update: %w", err)
	}

	// Start periodic update goroutine (every 5 minutes)
	go m.periodicUpdate()

	m.logger.Info("State Tracking Manager started successfully")
	return nil
}

// Stop stops the State Tracking Manager and cleans up subscriptions
func (m *Manager) Stop() {
	m.logger.Info("Stopping State Tracking Manager")

	// Stop periodic update
	close(m.stopChan)

	// Stop sun time updates
	if m.sunStopChan != nil {
		close(m.sunStopChan)
	}

	// Wait for goroutine to finish
	<-m.stoppedChan

	// Unsubscribe from all subscriptions
	for _, sub := range m.subscriptions {
		sub.Unsubscribe()
	}
	m.subscriptions = nil

	m.logger.Info("State Tracking Manager stopped")
}

// periodicUpdate runs every 5 minutes to update sun event and day phase
func (m *Manager) periodicUpdate() {
	defer close(m.stoppedChan)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.updateSunEventAndDayPhase(); err != nil {
				m.logger.Error("Failed to update sun event and day phase", zap.Error(err))
			}

		case <-m.stopChan:
			m.logger.Info("Stopping periodic state tracking updates")
			return
		}
	}
}

// updateSunEventAndDayPhase calculates and updates sunevent and dayPhase
func (m *Manager) updateSunEventAndDayPhase() error {
	// Get current sun event
	sunEvent := m.calculator.GetSunEvent()
	sunEventStr := string(sunEvent)

	// Get current sunevent value from state
	currentSunEvent, err := m.stateManager.GetString("sunevent")
	if err != nil {
		m.logger.Warn("Failed to get current sunevent", zap.Error(err))
		currentSunEvent = ""
	}

	// Update sunevent if it changed
	if currentSunEvent != sunEventStr {
		m.logger.Info("Sun event changed",
			zap.String("old", currentSunEvent),
			zap.String("new", sunEventStr))

		if !m.readOnly {
			if err := m.stateManager.SetString("sunevent", sunEventStr); err != nil {
				return fmt.Errorf("failed to update sunevent: %w", err)
			}
		} else {
			m.logger.Info("READ-ONLY mode: Would update sunevent",
				zap.String("value", sunEventStr))
		}
	}

	// Calculate day phase based on schedule
	schedule, err := m.configLoader.GetTodaysSchedule()
	if err != nil {
		m.logger.Warn("Failed to get schedule, using defaults", zap.Error(err))
		schedule = nil
	}

	dayPhase := m.calculator.CalculateDayPhase(schedule)
	dayPhaseStr := string(dayPhase)

	// Get current dayPhase value from state
	currentDayPhase, err := m.stateManager.GetString("dayPhase")
	if err != nil {
		m.logger.Warn("Failed to get current dayPhase", zap.Error(err))
		currentDayPhase = ""
	}

	// Update dayPhase if it changed
	if currentDayPhase != dayPhaseStr {
		m.logger.Info("Day phase changed",
			zap.String("old", currentDayPhase),
			zap.String("new", dayPhaseStr),
			zap.String("sun_event", sunEventStr))

		if !m.readOnly {
			if err := m.stateManager.SetString("dayPhase", dayPhaseStr); err != nil {
				return fmt.Errorf("failed to update dayPhase: %w", err)
			}
		} else {
			m.logger.Info("READ-ONLY mode: Would update dayPhase",
				zap.String("value", dayPhaseStr))
		}
	}

	return nil
}
