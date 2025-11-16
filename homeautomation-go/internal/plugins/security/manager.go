package security

import (
	"fmt"
	"sync"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Manager handles security-related automation
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	logger       *zap.Logger
	readOnly     bool

	// Subscriptions for cleanup
	haSubscriptions    []ha.Subscription
	stateSubscriptions []state.Subscription

	// Rate limiting for notifications
	lastDoorbellNotification       time.Time
	lastVehicleArrivalNotification time.Time
	mu                             sync.Mutex
}

// NewManager creates a new Security manager
func NewManager(haClient ha.HAClient, stateManager *state.Manager, logger *zap.Logger, readOnly bool) *Manager {
	return &Manager{
		haClient:           haClient,
		stateManager:       stateManager,
		logger:             logger.Named("security"),
		readOnly:           readOnly,
		haSubscriptions:    make([]ha.Subscription, 0),
		stateSubscriptions: make([]state.Subscription, 0),
	}
}

// Start begins monitoring security-related events
func (m *Manager) Start() error {
	m.logger.Info("Starting Security Manager")

	// 1. Subscribe to sleep/home states for lockdown activation
	sub, err := m.stateManager.Subscribe("isEveryoneAsleep", m.handleEveryoneAsleepChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to isEveryoneAsleep: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	sub, err = m.stateManager.Subscribe("isAnyoneHome", m.handleAnyoneHomeChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to isAnyoneHome: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	// 2. Subscribe to didOwnerJustReturnHome for garage auto-open
	sub, err = m.stateManager.Subscribe("didOwnerJustReturnHome", m.handleOwnerReturnHome)
	if err != nil {
		return fmt.Errorf("failed to subscribe to didOwnerJustReturnHome: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	// 3. Subscribe to doorbell button
	haSub, err := m.haClient.SubscribeStateChanges("input_button.doorbell", m.handleDoorbellPressed)
	if err != nil {
		return fmt.Errorf("failed to subscribe to doorbell: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, haSub)

	// 4. Subscribe to vehicle arriving button
	haSub, err = m.haClient.SubscribeStateChanges("input_button.vehicle_arriving", m.handleVehicleArriving)
	if err != nil {
		return fmt.Errorf("failed to subscribe to vehicle_arriving: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, haSub)

	// 5. Subscribe to lockdown activation for auto-reset
	haSub, err = m.haClient.SubscribeStateChanges("input_boolean.lockdown", m.handleLockdownActivated)
	if err != nil {
		return fmt.Errorf("failed to subscribe to lockdown: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, haSub)

	m.logger.Info("Security Manager started successfully")
	return nil
}

// Stop stops the Security Manager and cleans up subscriptions
func (m *Manager) Stop() {
	m.logger.Info("Stopping Security Manager")

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

	m.logger.Info("Security Manager stopped")
}

// handleEveryoneAsleepChange activates lockdown when everyone is asleep
func (m *Manager) handleEveryoneAsleepChange(key string, oldValue, newValue interface{}) {
	asleep, ok := newValue.(bool)
	if !ok {
		m.logger.Error("Invalid type for isEveryoneAsleep", zap.Any("value", newValue))
		return
	}

	if asleep {
		m.logger.Info("Everyone is asleep, activating lockdown")
		m.activateLockdown()
	}
}

// handleAnyoneHomeChange activates lockdown when no one is home
func (m *Manager) handleAnyoneHomeChange(key string, oldValue, newValue interface{}) {
	anyoneHome, ok := newValue.(bool)
	if !ok {
		m.logger.Error("Invalid type for isAnyoneHome", zap.Any("value", newValue))
		return
	}

	if !anyoneHome {
		m.logger.Info("No one is home, activating lockdown")
		m.activateLockdown()
	}
}

// activateLockdown turns on the lockdown input_boolean
func (m *Manager) activateLockdown() {
	if m.readOnly {
		m.logger.Info("READ-ONLY: Would activate lockdown")
		return
	}

	if err := m.haClient.CallService("input_boolean", "turn_on", map[string]interface{}{
		"entity_id": "input_boolean.lockdown",
	}); err != nil {
		m.logger.Error("Failed to activate lockdown", zap.Error(err))
	} else {
		m.logger.Info("Lockdown activated")
	}
}

// handleLockdownActivated auto-resets lockdown after 5 seconds
func (m *Manager) handleLockdownActivated(entity string, oldState, newState *ha.State) {
	if newState.State == "on" {
		m.logger.Info("Lockdown activated, will auto-reset in 5 seconds")

		// Wait 5 seconds, then reset
		go func() {
			time.Sleep(5 * time.Second)

			if m.readOnly {
				m.logger.Info("READ-ONLY: Would reset lockdown")
				return
			}

			if err := m.haClient.CallService("input_boolean", "turn_off", map[string]interface{}{
				"entity_id": "input_boolean.lockdown",
			}); err != nil {
				m.logger.Error("Failed to reset lockdown", zap.Error(err))
			} else {
				m.logger.Info("Lockdown reset")
			}
		}()
	}
}

// handleOwnerReturnHome opens garage door if owner just returned home
func (m *Manager) handleOwnerReturnHome(key string, oldValue, newValue interface{}) {
	returned, ok := newValue.(bool)
	if !ok {
		m.logger.Error("Invalid type for didOwnerJustReturnHome", zap.Any("value", newValue))
		return
	}

	if !returned {
		return
	}

	m.logger.Info("Owner just returned home, checking garage status")

	// Check if garage is empty (no vehicle detected)
	currentState, err := m.haClient.GetState("binary_sensor.garage_door_vehicle_detected")
	if err != nil {
		m.logger.Error("Failed to get garage sensor state", zap.Error(err))
		return
	}

	// If sensor is "off", garage is empty
	if currentState.State == "off" {
		m.logger.Info("Garage is empty, opening door")
		m.openGarageDoor()
	} else {
		m.logger.Info("Garage is occupied, not opening door")
	}
}

// openGarageDoor opens the garage door
func (m *Manager) openGarageDoor() {
	if m.readOnly {
		m.logger.Info("READ-ONLY: Would open garage door")
		return
	}

	if err := m.haClient.CallService("cover", "open_cover", map[string]interface{}{
		"entity_id": "cover.garage_door_door",
	}); err != nil {
		m.logger.Error("Failed to open garage door", zap.Error(err))
	} else {
		m.logger.Info("Garage door opened")
	}
}

// handleDoorbellPressed sends notifications when doorbell is pressed
func (m *Manager) handleDoorbellPressed(entity string, oldState, newState *ha.State) {
	// Rate limit: max 1 notification per 20 seconds
	m.mu.Lock()
	if time.Since(m.lastDoorbellNotification) < 20*time.Second {
		m.logger.Info("Doorbell notification rate limited")
		m.mu.Unlock()
		return
	}
	m.lastDoorbellNotification = time.Now()
	m.mu.Unlock()

	m.logger.Info("Doorbell pressed, sending notifications")

	// Send TTS notification
	m.sendTTSNotification("Doorbell ringing")

	// Flash lights twice
	go m.flashLightsForDoorbell()
}

// flashLightsForDoorbell flashes lights twice with 2-second delay
func (m *Manager) flashLightsForDoorbell() {
	lights := []string{
		"light.primary_suite",
		"light.living_room",
		"light.independent",
	}

	// First flash
	m.flashLights(lights)

	// Wait 2 seconds
	time.Sleep(2 * time.Second)

	// Second flash
	m.flashLights(lights)
}

// flashLights flashes the specified lights
func (m *Manager) flashLights(lights []string) {
	if m.readOnly {
		m.logger.Info("READ-ONLY: Would flash lights", zap.Strings("lights", lights))
		return
	}

	if err := m.haClient.CallService("light", "turn_on", map[string]interface{}{
		"entity_id": lights,
		"flash":     "short",
	}); err != nil {
		m.logger.Error("Failed to flash lights", zap.Error(err))
	}
}

// handleVehicleArriving announces when expected vehicle arrives
func (m *Manager) handleVehicleArriving(entity string, oldState, newState *ha.State) {
	// Check if we're expecting someone
	expectingSomeone, err := m.stateManager.GetBool("isExpectingSomeone")
	if err != nil {
		m.logger.Error("Failed to get isExpectingSomeone state", zap.Error(err))
		return
	}

	if !expectingSomeone {
		m.logger.Info("Vehicle arriving but not expecting anyone")
		return
	}

	// Rate limit: max 1 notification per 20 seconds
	m.mu.Lock()
	if time.Since(m.lastVehicleArrivalNotification) < 20*time.Second {
		m.logger.Info("Vehicle arrival notification rate limited")
		m.mu.Unlock()
		return
	}
	m.lastVehicleArrivalNotification = time.Now()
	m.mu.Unlock()

	m.logger.Info("Expected vehicle has arrived, sending notification")

	// Send TTS notification
	m.sendTTSNotification("They have arrived")

	// Reset expecting someone flag
	if !m.readOnly {
		if err := m.haClient.CallService("input_boolean", "turn_off", map[string]interface{}{
			"entity_id": "input_boolean.expecting_someone",
		}); err != nil {
			m.logger.Error("Failed to reset expecting_someone", zap.Error(err))
		}
	}
}

// sendTTSNotification sends a TTS message to all Sonos speakers
func (m *Manager) sendTTSNotification(message string) {
	if m.readOnly {
		m.logger.Info("READ-ONLY: Would send TTS notification", zap.String("message", message))
		return
	}

	speakers := []string{
		"media_player.bedroom",
		"media_player.kitchen",
		"media_player.dining_room",
		"media_player.soundbar",
		"media_player.kids_bathroom",
	}

	if err := m.haClient.CallService("tts", "speak", map[string]interface{}{
		"entity_id":              "tts.google_translate_en_com",
		"media_player_entity_id": speakers,
		"message":                message,
		"cache":                  true,
	}); err != nil {
		m.logger.Error("Failed to send TTS notification", zap.Error(err), zap.String("message", message))
	} else {
		m.logger.Info("TTS notification sent", zap.String("message", message))
	}
}
