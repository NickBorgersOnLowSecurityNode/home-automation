package energy

import (
	"fmt"
	"math"
	"sort"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Manager handles energy state calculations and updates
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	config       *EnergyConfig
	logger       *zap.Logger
	readOnly     bool
	timezone     *time.Location

	// Subscriptions for cleanup
	haSubscriptions    []ha.Subscription
	stateSubscriptions []state.Subscription

	// Control for free energy checker
	stopChecker chan struct{}
}

// NewManager creates a new Energy State manager
func NewManager(haClient ha.HAClient, stateManager *state.Manager, config *EnergyConfig, logger *zap.Logger, readOnly bool, timezone *time.Location) *Manager {
	// Default to UTC if no timezone provided
	if timezone == nil {
		timezone = time.UTC
	}

	return &Manager{
		haClient:           haClient,
		stateManager:       stateManager,
		config:             config,
		logger:             logger.Named("energy"),
		readOnly:           readOnly,
		timezone:           timezone,
		haSubscriptions:    make([]ha.Subscription, 0),
		stateSubscriptions: make([]state.Subscription, 0),
		stopChecker:        make(chan struct{}),
	}
}

// Start begins monitoring energy state
func (m *Manager) Start() error {
	m.logger.Info("Starting Energy State Manager")

	// Subscribe to battery level changes
	if err := m.subscribeToSensor("sensor.span_panel_span_storage_battery_percentage_2", m.handleBatteryChange); err != nil {
		return fmt.Errorf("failed to subscribe to battery sensor: %w", err)
	}

	// Subscribe to this hour solar generation
	if err := m.subscribeToSensor("sensor.energy_next_hour", m.handleThisHourSolarChange); err != nil {
		return fmt.Errorf("failed to subscribe to this hour solar sensor: %w", err)
	}

	// Subscribe to remaining solar generation
	if err := m.subscribeToSensor("sensor.energy_production_today_remaining", m.handleRemainingSolarChange); err != nil {
		return fmt.Errorf("failed to subscribe to remaining solar sensor: %w", err)
	}

	// Subscribe to grid availability changes
	sub, err := m.stateManager.Subscribe("isGridAvailable", m.handleGridAvailabilityChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to grid availability: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	// Subscribe to battery and solar energy level changes to recalculate overall level
	sub, err = m.stateManager.Subscribe("batteryEnergyLevel", m.handleIntermediateLevelChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to battery energy level: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	sub, err = m.stateManager.Subscribe("solarProductionEnergyLevel", m.handleIntermediateLevelChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to solar production energy level: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	sub, err = m.stateManager.Subscribe("isFreeEnergyAvailable", m.handleIntermediateLevelChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to free energy available: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	// Start free energy check timer (check every minute)
	go m.runFreeEnergyChecker()

	m.logger.Info("Energy State Manager started successfully")
	return nil
}

// Stop stops the Energy State Manager and cleans up subscriptions
func (m *Manager) Stop() {
	m.logger.Info("Stopping Energy State Manager")

	// Stop the free energy checker goroutine
	close(m.stopChecker)

	// Unsubscribe from all HA subscriptions
	for _, sub := range m.haSubscriptions {
		sub.Unsubscribe()
	}
	m.haSubscriptions = nil

	// Unsubscribe from all state subscriptions
	for _, sub := range m.stateSubscriptions {
		sub.Unsubscribe()
	}
	m.stateSubscriptions = nil

	m.logger.Info("Energy State Manager stopped")
}

// subscribeToSensor subscribes to a Home Assistant sensor
func (m *Manager) subscribeToSensor(entityID string, callback func(state float64)) error {
	sub, err := m.haClient.SubscribeStateChanges(entityID, func(entity string, oldState, newState *ha.State) {
		// Try to convert to float64
		var val float64

		// Parse the state string
		_, parseErr := fmt.Sscanf(newState.State, "%f", &val)
		if parseErr != nil {
			m.logger.Warn("Failed to parse sensor value as number",
				zap.String("entity_id", entityID),
				zap.String("value", newState.State))
			return
		}

		callback(val)
	})
	if err == nil {
		m.haSubscriptions = append(m.haSubscriptions, sub)
	}
	return err
}

// handleBatteryChange processes battery percentage changes
func (m *Manager) handleBatteryChange(percentage float64) {
	m.logger.Info("Battery level changed",
		zap.Float64("percentage", percentage))

	// Validate percentage is finite
	if math.IsNaN(percentage) || math.IsInf(percentage, 0) {
		m.logger.Warn("Battery percentage is not finite, ignoring",
			zap.Float64("percentage", percentage))
		return
	}

	// Determine battery energy level
	level := m.determineBatteryEnergyLevel(percentage)
	if level == "" {
		m.logger.Warn("No battery energy level determined",
			zap.Float64("percentage", percentage))
		return
	}

	m.logger.Info("Determined battery energy level",
		zap.Float64("percentage", percentage),
		zap.String("level", level))

	// Update state variable
	if err := m.stateManager.SetString("batteryEnergyLevel", level); err != nil {
		m.logger.Error("Failed to set batteryEnergyLevel",
			zap.Error(err))
	}
}

// handleThisHourSolarChange processes this hour solar generation changes
func (m *Manager) handleThisHourSolarChange(kw float64) {
	m.logger.Info("This hour solar generation changed",
		zap.Float64("kw", kw))

	// Validate kw is finite
	if math.IsNaN(kw) || math.IsInf(kw, 0) {
		m.logger.Warn("This hour solar generation is not finite, ignoring",
			zap.Float64("kw", kw))
		return
	}

	// Update state variable
	if err := m.stateManager.SetNumber("thisHourSolarGeneration", kw); err != nil {
		m.logger.Error("Failed to set thisHourSolarGeneration",
			zap.Error(err))
	}

	// Trigger recalculation
	m.recalculateSolarProductionLevel()
}

// handleRemainingSolarChange processes remaining solar generation changes
func (m *Manager) handleRemainingSolarChange(kwh float64) {
	m.logger.Info("Remaining solar generation changed",
		zap.Float64("kwh", kwh))

	// Validate kwh is finite
	if math.IsNaN(kwh) || math.IsInf(kwh, 0) {
		m.logger.Warn("Remaining solar generation is not finite, ignoring",
			zap.Float64("kwh", kwh))
		return
	}

	// Update state variable
	if err := m.stateManager.SetNumber("remainingSolarGeneration", kwh); err != nil {
		m.logger.Error("Failed to set remainingSolarGeneration",
			zap.Error(err))
	}

	// Trigger recalculation
	m.recalculateSolarProductionLevel()
}

// handleGridAvailabilityChange processes grid availability changes
func (m *Manager) handleGridAvailabilityChange(key string, oldValue, newValue interface{}) {
	m.logger.Info("Grid availability changed",
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	// Trigger free energy recalculation
	m.checkFreeEnergy()
}

// handleIntermediateLevelChange recalculates overall energy level when intermediate levels change
func (m *Manager) handleIntermediateLevelChange(key string, oldValue, newValue interface{}) {
	m.logger.Debug("Intermediate energy level changed",
		zap.String("key", key),
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	// Recalculate overall energy level
	m.recalculateOverallEnergyLevel()
}

// determineBatteryEnergyLevel determines the battery energy level based on percentage
func (m *Manager) determineBatteryEnergyLevel(percentage float64) string {
	// Build sorted list of levels by battery threshold
	type levelThreshold struct {
		name      string
		threshold float64
	}

	var levels []levelThreshold
	for _, state := range m.config.Energy.EnergyStates {
		if !math.IsNaN(state.BatteryMinimumPercentage) && !math.IsInf(state.BatteryMinimumPercentage, 0) {
			levels = append(levels, levelThreshold{
				name:      state.ConditionName,
				threshold: state.BatteryMinimumPercentage,
			})
		}
	}

	// Sort by threshold ascending
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].threshold < levels[j].threshold
	})

	// Find highest level where percentage >= threshold
	var chosen string
	for _, level := range levels {
		if percentage >= level.threshold {
			chosen = level.name
		}
	}

	m.logger.Debug("Determined battery energy level",
		zap.Float64("percentage", percentage),
		zap.String("level", chosen))

	return chosen
}

// recalculateSolarProductionLevel recalculates the solar production energy level
func (m *Manager) recalculateSolarProductionLevel() {
	thisHourKW, err := m.stateManager.GetNumber("thisHourSolarGeneration")
	if err != nil {
		m.logger.Error("Failed to get thisHourSolarGeneration", zap.Error(err))
		return
	}

	remainingKWH, err := m.stateManager.GetNumber("remainingSolarGeneration")
	if err != nil {
		m.logger.Error("Failed to get remainingSolarGeneration", zap.Error(err))
		return
	}

	level := m.determineSolarEnergyLevel(thisHourKW, remainingKWH)

	m.logger.Info("Determined solar production energy level",
		zap.Float64("this_hour_kw", thisHourKW),
		zap.Float64("remaining_kwh", remainingKWH),
		zap.String("level", level))

	// Update state variable
	if err := m.stateManager.SetString("solarProductionEnergyLevel", level); err != nil {
		m.logger.Error("Failed to set solarProductionEnergyLevel",
			zap.Error(err))
	}
}

// determineSolarEnergyLevel determines the solar energy level
func (m *Manager) determineSolarEnergyLevel(thisHourKW, remainingKWH float64) string {
	// Default to black
	level := "black"

	// Check each energy state in order (they should already be ordered in config)
	for _, state := range m.config.Energy.EnergyStates {
		// Both conditions must be met
		if thisHourKW >= state.EnergyProductionMinimumKW &&
			remainingKWH >= state.RemainingEnergyProductionMinimumKWH {
			level = state.ConditionName
		}
	}

	m.logger.Debug("Determined solar energy level",
		zap.Float64("this_hour_kw", thisHourKW),
		zap.Float64("remaining_kwh", remainingKWH),
		zap.String("level", level))

	return level
}

// recalculateOverallEnergyLevel calculates the overall energy level
func (m *Manager) recalculateOverallEnergyLevel() {
	// Check for free energy override
	isFreeEnergy, err := m.stateManager.GetBool("isFreeEnergyAvailable")
	if err != nil {
		m.logger.Error("Failed to get isFreeEnergyAvailable", zap.Error(err))
		return
	}

	if isFreeEnergy {
		m.logger.Info("Free energy is available, setting current energy level to white")
		if err := m.stateManager.SetString("currentEnergyLevel", "white"); err != nil {
			m.logger.Error("Failed to set currentEnergyLevel", zap.Error(err))
		}
		return
	}

	// Get battery and solar levels
	batteryLevel, err := m.stateManager.GetString("batteryEnergyLevel")
	if err != nil {
		m.logger.Error("Failed to get batteryEnergyLevel", zap.Error(err))
		return
	}

	solarLevel, err := m.stateManager.GetString("solarProductionEnergyLevel")
	if err != nil {
		m.logger.Error("Failed to get solarProductionEnergyLevel", zap.Error(err))
		return
	}

	overallLevel := m.determineOverallEnergyLevel(batteryLevel, solarLevel)

	m.logger.Info("Determined overall energy level",
		zap.String("battery_level", batteryLevel),
		zap.String("solar_level", solarLevel),
		zap.String("overall_level", overallLevel))

	// Update state variable
	if err := m.stateManager.SetString("currentEnergyLevel", overallLevel); err != nil {
		m.logger.Error("Failed to set currentEnergyLevel", zap.Error(err))
	}
}

// determineOverallEnergyLevel combines battery and solar levels
func (m *Manager) determineOverallEnergyLevel(batteryLevel, solarLevel string) string {
	// Extract ordered list of level names
	var levelNames []string
	for _, state := range m.config.Energy.EnergyStates {
		levelNames = append(levelNames, state.ConditionName)
	}

	// Find indexes of battery and solar levels
	batteryIndex := -1
	solarIndex := -1

	for i, name := range levelNames {
		if name == batteryLevel {
			batteryIndex = i
		}
		if name == solarLevel {
			solarIndex = i
		}
	}

	if batteryIndex == -1 || solarIndex == -1 {
		m.logger.Warn("Invalid battery or solar level",
			zap.String("battery_level", batteryLevel),
			zap.String("solar_level", solarLevel))
		return "black"
	}

	// Find min and max indexes
	minIndex := batteryIndex
	if solarIndex < minIndex {
		minIndex = solarIndex
	}

	maxIndex := batteryIndex
	if solarIndex > maxIndex {
		maxIndex = solarIndex
	}

	// Maximum allowed output is at most one level higher than the lowest input
	maxAllowedIndex := minIndex + 1
	if maxAllowedIndex >= len(levelNames) {
		maxAllowedIndex = len(levelNames) - 1
	}

	// Final output is the lesser of maxIndex and maxAllowedIndex
	outputIndex := maxIndex
	if maxAllowedIndex < outputIndex {
		outputIndex = maxAllowedIndex
	}

	result := levelNames[outputIndex]

	m.logger.Debug("Calculated overall energy level",
		zap.String("battery_level", batteryLevel),
		zap.Int("battery_index", batteryIndex),
		zap.String("solar_level", solarLevel),
		zap.Int("solar_index", solarIndex),
		zap.Int("min_index", minIndex),
		zap.Int("max_index", maxIndex),
		zap.Int("max_allowed_index", maxAllowedIndex),
		zap.Int("output_index", outputIndex),
		zap.String("result", result))

	return result
}

// runFreeEnergyChecker runs the free energy checker every minute
func (m *Manager) runFreeEnergyChecker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Check immediately on start
	m.checkFreeEnergy()

	for {
		select {
		case <-ticker.C:
			m.checkFreeEnergy()
		case <-m.stopChecker:
			m.logger.Info("Stopping free energy checker")
			return
		}
	}
}

// checkFreeEnergy checks if free energy is currently available
func (m *Manager) checkFreeEnergy() {
	isGridAvailable, err := m.stateManager.GetBool("isGridAvailable")
	if err != nil {
		m.logger.Error("Failed to get isGridAvailable", zap.Error(err))
		return
	}

	isFreeEnergy := m.isFreeEnergyTime(isGridAvailable)

	// Get current state
	currentFreeEnergy, err := m.stateManager.GetBool("isFreeEnergyAvailable")
	if err != nil {
		m.logger.Error("Failed to get isFreeEnergyAvailable", zap.Error(err))
		return
	}

	// Only log changes
	if isFreeEnergy != currentFreeEnergy {
		m.logger.Info("Free energy availability changed",
			zap.Bool("is_free_energy", isFreeEnergy),
			zap.Bool("is_grid_available", isGridAvailable))
	}

	// Update state
	if err := m.stateManager.SetBool("isFreeEnergyAvailable", isFreeEnergy); err != nil {
		m.logger.Error("Failed to set isFreeEnergyAvailable", zap.Error(err))
	}
}

// isFreeEnergyTime checks if current time is within free energy window
func (m *Manager) isFreeEnergyTime(isGridAvailable bool) bool {
	if !isGridAvailable {
		m.logger.Debug("Grid is not available, no free energy")
		return false
	}

	// Get current time in configured timezone
	now := time.Now().In(m.timezone)

	// Parse times (format: "21:00")
	startTime, err := time.Parse("15:04", m.config.Energy.FreeEnergyTime.Start)
	if err != nil {
		m.logger.Error("Failed to parse free energy start time", zap.Error(err))
		return false
	}

	endTime, err := time.Parse("15:04", m.config.Energy.FreeEnergyTime.End)
	if err != nil {
		m.logger.Error("Failed to parse free energy end time", zap.Error(err))
		return false
	}

	// Set the times to today in configured timezone
	todayStart := time.Date(now.Year(), now.Month(), now.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, m.timezone)

	todayEnd := time.Date(now.Year(), now.Month(), now.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, m.timezone)

	// If end time is before start time, it spans midnight
	if todayEnd.Before(todayStart) {
		// Free energy is from start time yesterday to end time today
		// OR from start time today to end time tomorrow
		if now.After(todayStart) || now.Before(todayEnd) {
			m.logger.Debug("Within free energy time (spans midnight)",
				zap.Time("now", now),
				zap.Time("start", todayStart),
				zap.Time("end", todayEnd))
			return true
		}
	} else {
		// Normal case: start and end on same day
		if now.After(todayStart) && now.Before(todayEnd) {
			m.logger.Debug("Within free energy time",
				zap.Time("now", now),
				zap.Time("start", todayStart),
				zap.Time("end", todayEnd))
			return true
		}
	}

	return false
}
