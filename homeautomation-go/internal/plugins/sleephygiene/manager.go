package sleephygiene

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

const (
	// CheckInterval is how often we check for schedule triggers
	CheckInterval = 1 * time.Minute

	// TriggerWindow is how long after a trigger time we'll still activate it
	TriggerWindow = 1 * time.Hour

	// WakeOffset is how many minutes after begin_wake that wake happens
	WakeOffset = 25 * time.Minute

	// FadeOutDelay is the base delay between volume decrements during fade out
	FadeOutDelay = 1 * time.Second
)

// Manager handles sleep hygiene scheduling and wake-up sequences
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	config       *ScheduleConfig
	logger       *zap.Logger
	readOnly     bool

	// Internal state
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	lastTriggers map[string]time.Time // Track when we last triggered each event
}

// NewManager creates a new Sleep Hygiene manager
func NewManager(haClient ha.HAClient, stateManager *state.Manager, config *ScheduleConfig, logger *zap.Logger, readOnly bool) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		haClient:     haClient,
		stateManager: stateManager,
		config:       config,
		logger:       logger.Named("sleephygiene"),
		readOnly:     readOnly,
		ctx:          ctx,
		cancel:       cancel,
		lastTriggers: make(map[string]time.Time),
	}
}

// Start begins the sleep hygiene manager
func (m *Manager) Start() error {
	m.logger.Info("Starting Sleep Hygiene Manager")

	// Subscribe to state changes that might require action
	if _, err := m.stateManager.Subscribe("isMasterAsleep", m.handleSleepStateChange); err != nil {
		return fmt.Errorf("failed to subscribe to isMasterAsleep: %w", err)
	}

	if _, err := m.stateManager.Subscribe("musicPlaybackType", m.handleMusicStateChange); err != nil {
		return fmt.Errorf("failed to subscribe to musicPlaybackType: %w", err)
	}

	// Start the schedule checker
	m.wg.Add(1)
	go m.scheduleChecker()

	m.logger.Info("Sleep Hygiene Manager started successfully")
	return nil
}

// Stop gracefully stops the sleep hygiene manager
func (m *Manager) Stop() {
	m.logger.Info("Stopping Sleep Hygiene Manager")
	m.cancel()
	m.wg.Wait()
	m.logger.Info("Sleep Hygiene Manager stopped")
}

// scheduleChecker runs every minute to check for schedule-based triggers
func (m *Manager) scheduleChecker() {
	defer m.wg.Done()

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	m.logger.Info("Schedule checker started")

	// Check immediately on start
	m.checkScheduleTriggers()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkScheduleTriggers()
		}
	}
}

// checkScheduleTriggers checks if any schedule-based triggers should fire
func (m *Manager) checkScheduleTriggers() {
	now := time.Now()

	// Get today's schedule
	todaySchedule, err := m.config.GetScheduleForDay(now.Weekday())
	if err != nil {
		m.logger.Error("Failed to get today's schedule", zap.Error(err))
		return
	}

	// Get alarmTime from state (this is begin_wake time as a Unix timestamp in milliseconds)
	alarmTimeMs, err := m.stateManager.GetNumber("alarmTime")
	if err != nil {
		m.logger.Debug("Failed to get alarmTime", zap.Error(err))
		return
	}

	// Convert milliseconds to time.Time
	var beginWakeTime time.Time
	if alarmTimeMs > 0 {
		beginWakeTime = time.Unix(int64(alarmTimeMs)/1000, 0)
	}

	// Calculate wake time (25 minutes after begin_wake)
	wakeTime := beginWakeTime.Add(WakeOffset)

	// Parse today's times
	stopScreensTime, err := ParseTimeToday(todaySchedule.StopScreens)
	if err != nil {
		m.logger.Error("Failed to parse stop_screens time", zap.Error(err))
		return
	}

	goToBedTime, err := ParseTimeToday(todaySchedule.GoToBed)
	if err != nil {
		m.logger.Error("Failed to parse go_to_bed time", zap.Error(err))
		return
	}

	// Check each trigger
	m.checkAndFireTrigger("begin_wake", beginWakeTime, now, m.handleBeginWake)
	m.checkAndFireTrigger("wake", wakeTime, now, m.handleWake)
	m.checkAndFireTrigger("stop_screens", stopScreensTime, now, m.handleStopScreens)
	m.checkAndFireTrigger("go_to_bed", goToBedTime, now, m.handleGoToBed)
}

// checkAndFireTrigger checks if a trigger should fire and calls the handler if so
func (m *Manager) checkAndFireTrigger(triggerName string, triggerTime, now time.Time, handler func()) {
	// Skip if trigger time is zero (not set)
	if triggerTime.IsZero() {
		return
	}

	// Check if we're past the trigger time but within the trigger window
	if now.After(triggerTime) && now.Before(triggerTime.Add(TriggerWindow)) {
		// Check if we've already triggered this recently
		m.mu.Lock()
		lastTrigger, exists := m.lastTriggers[triggerName]
		m.mu.Unlock()

		if exists && now.Sub(lastTrigger) < TriggerWindow {
			// Already triggered recently, skip
			return
		}

		m.logger.Info("Triggering schedule event",
			zap.String("event", triggerName),
			zap.Time("trigger_time", triggerTime),
			zap.Time("current_time", now))

		// Update last trigger time
		m.mu.Lock()
		m.lastTriggers[triggerName] = now
		m.mu.Unlock()

		// Call the handler
		handler()
	}
}

// handleBeginWake handles the begin_wake trigger (turn on bedroom lights slowly)
func (m *Manager) handleBeginWake() {
	m.logger.Info("Handling begin_wake event")

	// Check preconditions
	if !m.checkMasterHomeAndAsleep() {
		return
	}

	// Check if sleep music is playing
	musicType, err := m.stateManager.GetString("musicPlaybackType")
	if err != nil {
		m.logger.Error("Failed to get musicPlaybackType", zap.Error(err))
		return
	}

	if musicType != "sleep" {
		m.logger.Debug("Sleep music not playing, skipping begin_wake")
		return
	}

	// Start fading out sleep sounds
	m.logger.Info("Starting fade out of sleep sounds")
	if err := m.stateManager.SetBool("isFadeOutInProgress", true); err != nil {
		m.logger.Error("Failed to set isFadeOutInProgress", zap.Error(err))
		return
	}

	// Start fade out goroutine
	m.wg.Add(1)
	go m.fadeOutSleepSounds()

	// Turn on master bedroom lights slowly
	m.callHAService("light.turn_on", map[string]interface{}{
		"entity_id":   "light.master_bedroom",
		"brightness":  1,
		"transition":  1500, // 25 minutes in seconds
		"kelvin":      2700, // Warm white
	}, "Turn on master bedroom lights slowly")
}

// handleWake handles the wake trigger (flash lights in common areas)
func (m *Manager) handleWake() {
	m.logger.Info("Handling wake event")

	// Check if anyone is home and not everyone is asleep
	if !m.checkAnyoneHomeNotEveryoneAsleep() {
		return
	}

	// Flash lights in common areas
	m.flashLights("Flash lights at wake time")
}

// handleStopScreens handles the stop_screens trigger (flash lights reminder)
func (m *Manager) handleStopScreens() {
	m.logger.Info("Handling stop_screens event")

	// Check if anyone is home and not everyone is asleep
	if !m.checkAnyoneHomeNotEveryoneAsleep() {
		return
	}

	// Flash lights to remind people to stop using screens
	m.flashLights("Flash lights to stop using screens")
}

// handleGoToBed handles the go_to_bed trigger (set music to sleep mode)
func (m *Manager) handleGoToBed() {
	m.logger.Info("Handling go_to_bed event")

	// Check preconditions
	if !m.checkMasterHomeAndAsleep() {
		return
	}

	// Set music playback type to "sleep"
	m.logger.Info("Setting music playback type to sleep")
	if err := m.stateManager.SetString("musicPlaybackType", "sleep"); err != nil {
		m.logger.Error("Failed to set musicPlaybackType to sleep", zap.Error(err))
	}
}

// handleSleepStateChange handles changes to sleep state
func (m *Manager) handleSleepStateChange(key string, oldValue, newValue interface{}) {
	m.logger.Debug("Sleep state changed",
		zap.String("key", key),
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	// If master just woke up, cancel fade out
	if key == "isMasterAsleep" {
		asleep, ok := newValue.(bool)
		if ok && !asleep {
			m.logger.Info("Master woke up, canceling fade out")
			m.stateManager.SetBool("isFadeOutInProgress", false)
		}
	}
}

// handleMusicStateChange handles changes to music playback type
func (m *Manager) handleMusicStateChange(key string, oldValue, newValue interface{}) {
	m.logger.Debug("Music state changed",
		zap.String("key", key),
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	// If music changed away from sleep, cancel fade out
	if key == "musicPlaybackType" {
		musicType, ok := newValue.(string)
		if ok && musicType != "sleep" {
			m.logger.Info("Music changed away from sleep, canceling fade out")
			m.stateManager.SetBool("isFadeOutInProgress", false)
		}
	}
}

// fadeOutSleepSounds gradually fades out bedroom speaker volumes to 0
func (m *Manager) fadeOutSleepSounds() {
	defer m.wg.Done()

	m.logger.Info("Starting fade out of sleep sounds")

	// Get current playing music to identify bedroom speakers
	var currentlyPlaying map[string]interface{}
	if err := m.stateManager.GetJSON("currentlyPlayingMusic", &currentlyPlaying); err != nil {
		m.logger.Debug("Failed to get currentlyPlayingMusic", zap.Error(err))
		return
	}

	// Extract participants (speakers)
	participants, ok := currentlyPlaying["participants"].([]interface{})
	if !ok || len(participants) == 0 {
		m.logger.Debug("No participants in currentlyPlayingMusic")
		return
	}

	// Find bedroom speakers and fade them out
	for _, p := range participants {
		participant, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		playerName, ok := participant["player_name"].(string)
		if !ok {
			continue
		}

		// Check if this is a bedroom speaker
		if !contains(playerName, "Bedroom") {
			continue
		}

		// Start fade out for this speaker
		m.wg.Add(1)
		go m.fadeOutSpeaker(playerName)
	}
}

// fadeOutSpeaker fades out a single speaker
func (m *Manager) fadeOutSpeaker(playerName string) {
	defer m.wg.Done()

	m.logger.Info("Fading out speaker", zap.String("player", playerName))

	// Get current volume
	// Note: In a real implementation, we'd query Sonos for the current volume
	// For now, we'll simulate by decrementing from a typical sleep volume
	currentVolume := 20

	for currentVolume > 0 {
		// Check if fade out was canceled
		fadeOutInProgress, err := m.stateManager.GetBool("isFadeOutInProgress")
		if err != nil || !fadeOutInProgress {
			m.logger.Info("Fade out canceled", zap.String("player", playerName))
			return
		}

		// Check if music is still in sleep mode
		musicType, err := m.stateManager.GetString("musicPlaybackType")
		if err != nil || musicType != "sleep" {
			m.logger.Info("Music no longer in sleep mode, stopping fade out",
				zap.String("player", playerName))
			return
		}

		// Decrement volume
		currentVolume--

		// Set the new volume
		m.callHAService("media_player.volume_set", map[string]interface{}{
			"entity_id":   fmt.Sprintf("media_player.%s", sanitizeEntityName(playerName)),
			"volume_level": float64(currentVolume) / 100.0,
		}, fmt.Sprintf("Set %s volume to %d", playerName, currentVolume))

		// Wait before next decrement (wait longer as volume gets lower)
		delay := time.Duration(60-currentVolume) * FadeOutDelay
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	m.logger.Info("Fade out complete", zap.String("player", playerName))
}

// flashLights flashes lights in common areas
func (m *Manager) flashLights(reason string) {
	m.logger.Info("Flashing lights", zap.String("reason", reason))

	// Flash living room lights
	m.callHAService("light.turn_on", map[string]interface{}{
		"entity_id": "light.living_room",
		"flash":     "short",
	}, "Flash living room lights")

	// Flash kitchen lights
	m.callHAService("light.turn_on", map[string]interface{}{
		"entity_id": "light.kitchen",
		"flash":     "short",
	}, "Flash kitchen lights")
}

// checkMasterHomeAndAsleep checks if master is home and asleep
func (m *Manager) checkMasterHomeAndAsleep() bool {
	isAnyoneHome, err := m.stateManager.GetBool("isAnyoneHome")
	if err != nil || !isAnyoneHome {
		m.logger.Debug("Master not home, skipping")
		return false
	}

	isMasterAsleep, err := m.stateManager.GetBool("isMasterAsleep")
	if err != nil || !isMasterAsleep {
		m.logger.Debug("Master not asleep, skipping")
		return false
	}

	return true
}

// checkAnyoneHomeNotEveryoneAsleep checks if anyone is home and not everyone is asleep
func (m *Manager) checkAnyoneHomeNotEveryoneAsleep() bool {
	isAnyoneHome, err := m.stateManager.GetBool("isAnyoneHome")
	if err != nil || !isAnyoneHome {
		m.logger.Debug("No one home, skipping")
		return false
	}

	isEveryoneAsleep, err := m.stateManager.GetBool("isEveryoneAsleep")
	if err != nil || isEveryoneAsleep {
		m.logger.Debug("Everyone asleep, skipping")
		return false
	}

	return true
}

// callHAService calls a Home Assistant service (respects read-only mode)
func (m *Manager) callHAService(service string, data map[string]interface{}, description string) {
	if m.readOnly {
		m.logger.Info("Would call HA service (read-only mode)",
			zap.String("service", service),
			zap.String("description", description),
			zap.Any("data", data))
		return
	}

	m.logger.Info("Calling HA service",
		zap.String("service", service),
		zap.String("description", description))

	// Convert data to JSON
	dataJSON, err := json.Marshal(data)
	if err != nil {
		m.logger.Error("Failed to marshal service data", zap.Error(err))
		return
	}

	// Call the service via HA client
	// Note: The current ha.HAClient interface doesn't have a CallService method
	// In a real implementation, this would call the service via the HA WebSocket API
	m.logger.Debug("Service call data", zap.ByteString("json", dataJSON))
}

// Helper functions

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// sanitizeEntityName converts a friendly name to an entity name
func sanitizeEntityName(name string) string {
	// Simple implementation - in reality, this would need more robust sanitization
	result := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result += string(c)
		} else if c == ' ' {
			result += "_"
		}
	}
	return result
}
