package tv

import (
	"errors"
	"fmt"
	"strings"

	"homeautomation/internal/ha"
	"homeautomation/internal/shadowstate"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Manager handles TV monitoring and manipulation
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	logger       *zap.Logger
	readOnly     bool

	// Subscriptions for cleanup
	haSubscriptions    []ha.Subscription
	stateSubscriptions []state.Subscription

	// Shadow state tracking
	shadowTracker *shadowstate.TVTracker

	// Automatic shadow state input tracking
	pluginName  string
	registry    *shadowstate.SubscriptionRegistry
	inputHelper *shadowstate.InputCaptureHelper
}

// NewManager creates a new TV manager
func NewManager(haClient ha.HAClient, stateManager *state.Manager, logger *zap.Logger, readOnly bool, registry *shadowstate.SubscriptionRegistry) *Manager {
	const pluginName = "tv"
	m := &Manager{
		haClient:           haClient,
		stateManager:       stateManager,
		logger:             logger.Named("tv"),
		readOnly:           readOnly,
		haSubscriptions:    make([]ha.Subscription, 0),
		stateSubscriptions: make([]state.Subscription, 0),
		shadowTracker:      shadowstate.NewTVTracker(),
		pluginName:         pluginName,
		registry:           registry,
	}

	// Create input capture helper if registry is provided
	if registry != nil {
		m.inputHelper = shadowstate.NewInputCaptureHelper(registry, haClient, stateManager)
	}

	return m
}

// GetShadowState returns the current shadow state
func (m *Manager) GetShadowState() *shadowstate.TVShadowState {
	return m.shadowTracker.GetState()
}

// Start begins monitoring TV-related entities
func (m *Manager) Start() error {
	m.logger.Info("Starting TV Manager")

	// Register subscriptions with the registry for automatic input tracking
	if m.registry != nil {
		// HA subscriptions
		m.registry.RegisterHASubscription(m.pluginName, "media_player.big_beautiful_oled")
		m.registry.RegisterHASubscription(m.pluginName, "switch.sync_box_power")
		m.registry.RegisterHASubscription(m.pluginName, "select.sync_box_hdmi_input")

		// State subscriptions
		m.registry.RegisterStateSubscription(m.pluginName, "isAppleTVPlaying")
		m.registry.RegisterStateSubscription(m.pluginName, "isTVon")
	}

	// Subscribe to Apple TV media player state changes
	appleTVSub, err := m.haClient.SubscribeStateChanges("media_player.big_beautiful_oled", m.handleAppleTVStateChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to media_player.big_beautiful_oled: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, appleTVSub)

	// Subscribe to sync box power state changes
	syncBoxSub, err := m.haClient.SubscribeStateChanges("switch.sync_box_power", m.handleSyncBoxPowerChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to switch.sync_box_power: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, syncBoxSub)

	// Subscribe to HDMI input selector changes
	hdmiInputSub, err := m.haClient.SubscribeStateChanges("select.sync_box_hdmi_input", m.handleHDMIInputChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to select.sync_box_hdmi_input: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, hdmiInputSub)

	// Subscribe to isAppleTVPlaying state changes to recalculate isTVPlaying
	sub, err := m.stateManager.Subscribe("isAppleTVPlaying", m.handleAppleTVPlayingChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to isAppleTVPlaying: %w", err)
	}
	m.stateSubscriptions = append(m.stateSubscriptions, sub)

	// Initialize current states
	m.logger.Info("Initializing TV states from current HA entities")
	if err := m.initializeStates(); err != nil {
		m.logger.Warn("Failed to initialize some TV states", zap.Error(err))
	}

	m.logger.Info("TV Manager started successfully")
	return nil
}

// Stop stops the TV Manager and cleans up subscriptions
func (m *Manager) Stop() {
	m.logger.Info("Stopping TV Manager")

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

	m.logger.Info("TV Manager stopped")
}

// initializeStates fetches current HA entity states and initializes state variables
func (m *Manager) initializeStates() error {
	// Get Apple TV state
	appleTVState, err := m.haClient.GetState("media_player.big_beautiful_oled")
	if err == nil && appleTVState != nil {
		m.handleAppleTVStateChange("media_player.big_beautiful_oled", nil, appleTVState)
	} else if err != nil {
		m.logger.Warn("Failed to get initial Apple TV state", zap.Error(err))
	}

	// Get sync box power state
	syncBoxState, err := m.haClient.GetState("switch.sync_box_power")
	if err == nil && syncBoxState != nil {
		m.handleSyncBoxPowerChange("switch.sync_box_power", nil, syncBoxState)
	} else if err != nil {
		m.logger.Warn("Failed to get initial sync box state", zap.Error(err))
	}

	// Get HDMI input state
	hdmiInputState, err := m.haClient.GetState("select.sync_box_hdmi_input")
	if err == nil && hdmiInputState != nil {
		m.handleHDMIInputChange("select.sync_box_hdmi_input", nil, hdmiInputState)
	} else if err != nil {
		m.logger.Warn("Failed to get initial HDMI input state", zap.Error(err))
	}

	return nil
}

// handleAppleTVStateChange processes media_player.big_beautiful_oled state changes
func (m *Manager) handleAppleTVStateChange(entityID string, oldState, newState *ha.State) {
	if newState == nil {
		return
	}

	// Update shadow state inputs with raw HA entity value
	m.updateShadowInputs()

	// Check if Apple TV is playing
	isPlaying := newState.State == "playing"

	m.logger.Debug("Apple TV state changed",
		zap.String("entity_id", entityID),
		zap.String("new_state", newState.State),
		zap.Bool("is_playing", isPlaying))

	// Update isAppleTVPlaying state variable
	if err := m.stateManager.SetBool("isAppleTVPlaying", isPlaying); err != nil {
		if errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Debug("Skipping isAppleTVPlaying update in read-only mode",
				zap.Bool("is_playing", isPlaying))
		} else {
			m.logger.Error("Failed to set isAppleTVPlaying", zap.Error(err))
		}
	}

	// Update shadow state
	m.shadowTracker.UpdateAppleTVState(isPlaying, newState.State)
}

// handleSyncBoxPowerChange processes switch.sync_box_power state changes
func (m *Manager) handleSyncBoxPowerChange(entityID string, oldState, newState *ha.State) {
	if newState == nil {
		return
	}

	// Update shadow state inputs with raw HA entity value
	m.updateShadowInputs()

	// Check if sync box is on
	isTVOn := newState.State == "on"

	m.logger.Debug("Sync box power state changed",
		zap.String("entity_id", entityID),
		zap.String("new_state", newState.State),
		zap.Bool("is_tv_on", isTVOn))

	// Update isTVon state variable
	if err := m.stateManager.SetBool("isTVon", isTVOn); err != nil {
		if errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Debug("Skipping isTVon update in read-only mode",
				zap.Bool("is_tv_on", isTVOn))
		} else {
			m.logger.Error("Failed to set isTVon", zap.Error(err))
		}
	}

	// Update shadow state
	m.shadowTracker.UpdateTVPower(isTVOn)

	// If TV is off, then it's definitely not playing
	if !isTVOn {
		if err := m.stateManager.SetBool("isTVPlaying", false); err != nil {
			if errors.Is(err, state.ErrReadOnlyMode) {
				m.logger.Debug("Skipping isTVPlaying update in read-only mode",
					zap.Bool("is_playing", false))
			} else {
				m.logger.Error("Failed to set isTVPlaying to false", zap.Error(err))
			}
		}
		// Update shadow state
		m.shadowTracker.UpdateTVPlaying(false)
	}
}

// handleHDMIInputChange processes select.sync_box_hdmi_input state changes
func (m *Manager) handleHDMIInputChange(entityID string, oldState, newState *ha.State) {
	if newState == nil {
		return
	}

	// Update shadow state inputs with raw HA entity value
	m.updateShadowInputs()

	hdmiInput := newState.State

	m.logger.Debug("HDMI input changed",
		zap.String("entity_id", entityID),
		zap.String("new_input", hdmiInput))

	// Update shadow state
	m.shadowTracker.UpdateHDMIInput(hdmiInput)

	// Calculate isTVPlaying based on HDMI input
	m.calculateTVPlaying(hdmiInput)
}

// handleAppleTVPlayingChange is called when isAppleTVPlaying state variable changes
func (m *Manager) handleAppleTVPlayingChange(key string, oldValue, newValue interface{}) {
	m.logger.Debug("isAppleTVPlaying state changed",
		zap.String("key", key),
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	// Get current HDMI input to recalculate isTVPlaying
	hdmiInputState, err := m.haClient.GetState("select.sync_box_hdmi_input")
	if err != nil {
		m.logger.Warn("Failed to get HDMI input state", zap.Error(err))
		return
	}

	if hdmiInputState != nil {
		m.calculateTVPlaying(hdmiInputState.State)
	}
}

// updateShadowInputs captures the current input values from HA entities
func (m *Manager) updateShadowInputs() {
	// Use automatic input capture if available
	if m.inputHelper != nil {
		inputs := m.inputHelper.CaptureInputs(m.pluginName)
		m.shadowTracker.UpdateCurrentInputs(inputs)
		return
	}

	// Fallback to manual capture if no registry
	inputs := make(map[string]interface{})

	// Capture raw HA entity states
	if state, err := m.haClient.GetState("media_player.big_beautiful_oled"); err == nil && state != nil {
		inputs["media_player.big_beautiful_oled"] = state.State
	}
	if state, err := m.haClient.GetState("switch.sync_box_power"); err == nil && state != nil {
		inputs["switch.sync_box_power"] = state.State
	}
	if state, err := m.haClient.GetState("select.sync_box_hdmi_input"); err == nil && state != nil {
		inputs["select.sync_box_hdmi_input"] = state.State
	}

	// Also capture derived state variables
	if val, err := m.stateManager.GetBool("isAppleTVPlaying"); err == nil {
		inputs["isAppleTVPlaying"] = val
	}
	if val, err := m.stateManager.GetBool("isTVon"); err == nil {
		inputs["isTVon"] = val
	}

	m.shadowTracker.UpdateCurrentInputs(inputs)
}

// calculateTVPlaying determines isTVPlaying based on HDMI input and Apple TV state
func (m *Manager) calculateTVPlaying(hdmiInput string) {
	// Check if Apple TV is the current input
	isAppleTVInput := strings.Contains(hdmiInput, "AppleTV")

	var isTVPlaying bool

	if isAppleTVInput {
		// If Apple TV is selected, isTVPlaying = isAppleTVPlaying
		isAppleTVPlaying, err := m.stateManager.GetBool("isAppleTVPlaying")
		if err != nil {
			m.logger.Error("Failed to get isAppleTVPlaying", zap.Error(err))
			return
		}
		isTVPlaying = isAppleTVPlaying
	} else {
		// If other input (e.g., console, cable), assume TV is playing
		isTVPlaying = true
	}

	m.logger.Debug("Calculated isTVPlaying",
		zap.String("hdmi_input", hdmiInput),
		zap.Bool("is_appletv_input", isAppleTVInput),
		zap.Bool("is_tv_playing", isTVPlaying))

	// Update isTVPlaying state variable
	if err := m.stateManager.SetBool("isTVPlaying", isTVPlaying); err != nil {
		if errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Debug("Skipping isTVPlaying update in read-only mode",
				zap.Bool("is_playing", isTVPlaying))
		} else {
			m.logger.Error("Failed to set isTVPlaying", zap.Error(err))
		}
	}

	// Update shadow state
	m.shadowTracker.UpdateTVPlaying(isTVPlaying)
}
