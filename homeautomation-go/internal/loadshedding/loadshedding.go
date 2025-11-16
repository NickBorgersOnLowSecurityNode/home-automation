package loadshedding

import (
	"fmt"
	"sync"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

const (
	// Rate limiting: minimum time between actions
	minActionInterval = 1 * time.Hour

	// Energy states
	energyStateRed   = "red"
	energyStateBlack = "black"
	energyStateGreen = "green"
	energyStateWhite = "white"

	// Thermostat entities
	thermostatHoldHouse = "switch.most_of_house_thermostat_hold"
	thermostatHoldSuite = "switch.primary_suite_thermostat_hold"
	climateHouse        = "climate.most_of_house_thermostat"
	climateSuite        = "climate.primary_suite_thermostat"

	// Temperature ranges
	tempLowRestricted  = 65.0
	tempHighRestricted = 80.0
)

// LoadShedding manages thermostat control based on energy state
type LoadShedding struct {
	state        *state.Manager
	client       ha.HAClient
	logger       *zap.Logger
	lastAction   time.Time
	lastActionMu sync.Mutex
	subscription state.Subscription
	enabled      bool
}

// NewManager creates a new LoadShedding controller
func NewManager(stateManager *state.Manager, client ha.HAClient, logger *zap.Logger) *LoadShedding {
	return &LoadShedding{
		state:   stateManager,
		client:  client,
		logger:  logger.Named("loadshedding"),
		enabled: false,
	}
}

// Start begins monitoring energy state and controlling thermostats
func (ls *LoadShedding) Start() error {
	if ls.enabled {
		return fmt.Errorf("load shedding already started")
	}

	ls.logger.Info("Starting Load Shedding controller")

	// Subscribe to energy level changes
	sub, err := ls.state.Subscribe("currentEnergyLevel", ls.handleEnergyChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to energy level: %w", err)
	}
	ls.subscription = sub

	// Process initial state
	currentLevel, err := ls.state.GetString("currentEnergyLevel")
	if err != nil {
		ls.logger.Warn("Failed to get initial energy level", zap.Error(err))
	} else {
		ls.logger.Info("Initial energy level", zap.String("level", currentLevel))
		ls.handleEnergyChange("currentEnergyLevel", "", currentLevel)
	}

	ls.enabled = true
	return nil
}

// Stop stops the Load Shedding controller
func (ls *LoadShedding) Stop() {
	if !ls.enabled {
		return
	}

	ls.logger.Info("Stopping Load Shedding controller")
	if ls.subscription != nil {
		ls.subscription.Unsubscribe()
		ls.subscription = nil
	}
	ls.enabled = false
}

// handleEnergyChange is called when currentEnergyLevel changes
func (ls *LoadShedding) handleEnergyChange(key string, oldValue, newValue interface{}) {
	// Convert values to strings
	oldLevel := ""
	if oldValue != nil {
		if s, ok := oldValue.(string); ok {
			oldLevel = s
		}
	}

	newLevel := ""
	if newValue != nil {
		if s, ok := newValue.(string); ok {
			newLevel = s
		}
	}

	ls.logger.Info("Energy level changed",
		zap.String("old_level", oldLevel),
		zap.String("new_level", newLevel))

	// Determine action based on new state
	switch newLevel {
	case energyStateRed, energyStateBlack:
		ls.enableLoadShedding(newLevel)
	case energyStateGreen, energyStateWhite:
		ls.disableLoadShedding(newLevel)
	default:
		ls.logger.Warn("Unknown energy state",
			zap.String("state", newLevel))
	}
}

// enableLoadShedding activates load shedding (energy state red/black)
func (ls *LoadShedding) enableLoadShedding(energyLevel string) {
	ls.logger.Info("=== LOAD SHEDDING DECISION: ENABLE ===",
		zap.String("energy_level", energyLevel),
		zap.String("reason", "Energy state is "+energyLevel+" (low battery)"))

	// Check rate limiting
	if !ls.checkRateLimit() {
		return
	}

	// Turn on thermostat hold mode
	ls.logger.Info("Executing: Enable thermostat hold mode",
		zap.Strings("entities", []string{thermostatHoldHouse, thermostatHoldSuite}))

	if err := ls.client.CallService("switch", "turn_on", map[string]interface{}{
		"entity_id": []string{thermostatHoldHouse, thermostatHoldSuite},
	}); err != nil {
		ls.logger.Error("Failed to enable thermostat hold mode",
			zap.Error(err))
		return
	}

	ls.logger.Info("✓ Successfully enabled thermostat hold mode")

	// Set wider temperature range
	ls.logger.Info("Executing: Set wider temperature range",
		zap.Float64("temp_low", tempLowRestricted),
		zap.Float64("temp_high", tempHighRestricted),
		zap.Strings("entities", []string{climateHouse, climateSuite}))

	if err := ls.client.CallService("climate", "set_temperature", map[string]interface{}{
		"entity_id":       []string{climateHouse, climateSuite},
		"target_temp_low":  tempLowRestricted,
		"target_temp_high": tempHighRestricted,
	}); err != nil {
		ls.logger.Error("Failed to set thermostat temperature range",
			zap.Error(err))
		return
	}

	ls.logger.Info("✓ Successfully set wider temperature range")
	ls.logger.Info("=== LOAD SHEDDING ACTIVATED ===",
		zap.String("action", "HVAC restricted to conserve battery"))

	// Update last action time
	ls.lastActionMu.Lock()
	ls.lastAction = time.Now()
	ls.lastActionMu.Unlock()
}

// disableLoadShedding deactivates load shedding (energy state green/white)
func (ls *LoadShedding) disableLoadShedding(energyLevel string) {
	ls.logger.Info("=== LOAD SHEDDING DECISION: DISABLE ===",
		zap.String("energy_level", energyLevel),
		zap.String("reason", "Energy state is "+energyLevel+" (battery restored)"))

	// Check rate limiting
	if !ls.checkRateLimit() {
		return
	}

	// Turn off thermostat hold mode (return to schedule)
	ls.logger.Info("Executing: Disable thermostat hold mode (restore schedule)",
		zap.Strings("entities", []string{thermostatHoldHouse, thermostatHoldSuite}))

	if err := ls.client.CallService("switch", "turn_off", map[string]interface{}{
		"entity_id": []string{thermostatHoldHouse, thermostatHoldSuite},
	}); err != nil {
		ls.logger.Error("Failed to disable thermostat hold mode",
			zap.Error(err))
		return
	}

	ls.logger.Info("✓ Successfully disabled thermostat hold mode")
	ls.logger.Info("=== LOAD SHEDDING DEACTIVATED ===",
		zap.String("action", "HVAC returned to normal schedule"))

	// Update last action time
	ls.lastActionMu.Lock()
	ls.lastAction = time.Now()
	ls.lastActionMu.Unlock()
}

// checkRateLimit ensures we don't take actions too frequently
func (ls *LoadShedding) checkRateLimit() bool {
	ls.lastActionMu.Lock()
	defer ls.lastActionMu.Unlock()

	now := time.Now()
	timeSinceLastAction := now.Sub(ls.lastAction)

	if !ls.lastAction.IsZero() && timeSinceLastAction < minActionInterval {
		timeRemaining := minActionInterval - timeSinceLastAction
		ls.logger.Info("⏱  RATE LIMIT: Action skipped",
			zap.Duration("time_since_last_action", timeSinceLastAction),
			zap.Duration("min_interval", minActionInterval),
			zap.Duration("time_remaining", timeRemaining),
			zap.String("reason", "Preventing rapid thermostat toggling"))
		return false
	}

	ls.logger.Info("✓ Rate limit check passed",
		zap.Duration("time_since_last_action", timeSinceLastAction))
	return true
}
