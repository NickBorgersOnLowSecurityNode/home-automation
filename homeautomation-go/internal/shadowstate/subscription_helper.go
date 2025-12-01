package shadowstate

import (
	"fmt"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// ShadowInputUpdater is the interface that shadow trackers must implement
// to receive automatic input updates from SubscriptionHelper.
type ShadowInputUpdater interface {
	UpdateCurrentInputs(inputs map[string]interface{})
}

// SubscriptionHelper wraps HA and state subscriptions to automatically
// capture shadow state inputs before invoking handlers. This eliminates
// the need for every handler to manually call updateShadowInputs().
type SubscriptionHelper struct {
	haClient      ha.HAClient
	stateManager  *state.Manager
	registry      *SubscriptionRegistry
	inputHelper   *InputCaptureHelper
	shadowTracker ShadowInputUpdater
	pluginName    string
	logger        *zap.Logger

	// Track subscriptions for cleanup
	haSubscriptions    []ha.Subscription
	stateSubscriptions []state.Subscription
}

// NewSubscriptionHelper creates a new subscription helper for a plugin.
// The shadowTracker receives automatic input updates before each handler runs.
func NewSubscriptionHelper(
	haClient ha.HAClient,
	stateManager *state.Manager,
	registry *SubscriptionRegistry,
	shadowTracker ShadowInputUpdater,
	pluginName string,
	logger *zap.Logger,
) *SubscriptionHelper {
	h := &SubscriptionHelper{
		haClient:           haClient,
		stateManager:       stateManager,
		registry:           registry,
		shadowTracker:      shadowTracker,
		pluginName:         pluginName,
		logger:             logger,
		haSubscriptions:    make([]ha.Subscription, 0),
		stateSubscriptions: make([]state.Subscription, 0),
	}

	// Create input helper for automatic capture
	if registry != nil {
		h.inputHelper = NewInputCaptureHelper(registry, haClient, stateManager)
	}

	return h
}

// captureInputs captures all registered inputs and updates the shadow tracker.
// This is called automatically before every handler.
func (h *SubscriptionHelper) captureInputs() {
	if h.inputHelper == nil || h.shadowTracker == nil {
		return
	}
	inputs := h.inputHelper.CaptureInputs(h.pluginName)
	h.shadowTracker.UpdateCurrentInputs(inputs)
}

// SubscribeToSensor subscribes to a Home Assistant sensor entity and parses its
// state as a float64. Shadow state inputs are automatically captured before
// the handler is called.
func (h *SubscriptionHelper) SubscribeToSensor(entityID string, handler func(value float64)) error {
	// Register the subscription for input capture
	if h.registry != nil {
		h.registry.RegisterHASubscription(h.pluginName, entityID)
	}

	sub, err := h.haClient.SubscribeStateChanges(entityID, func(entity string, oldState, newState *ha.State) {
		if newState == nil {
			return
		}

		// Capture shadow state inputs BEFORE calling the handler
		h.captureInputs()

		// Parse the state string as float64
		var val float64
		_, parseErr := fmt.Sscanf(newState.State, "%f", &val)
		if parseErr != nil {
			h.logger.Warn("Failed to parse sensor value as number",
				zap.String("entity_id", entityID),
				zap.String("value", newState.State))
			return
		}

		handler(val)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", entityID, err)
	}

	h.haSubscriptions = append(h.haSubscriptions, sub)
	return nil
}

// SubscribeToEntity subscribes to a Home Assistant entity with full state access.
// Shadow state inputs are automatically captured before the handler is called.
func (h *SubscriptionHelper) SubscribeToEntity(entityID string, handler func(entityID string, oldState, newState *ha.State)) error {
	// Register the subscription for input capture
	if h.registry != nil {
		h.registry.RegisterHASubscription(h.pluginName, entityID)
	}

	sub, err := h.haClient.SubscribeStateChanges(entityID, func(entity string, oldState, newState *ha.State) {
		// Capture shadow state inputs BEFORE calling the handler
		h.captureInputs()

		handler(entity, oldState, newState)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", entityID, err)
	}

	h.haSubscriptions = append(h.haSubscriptions, sub)
	return nil
}

// SubscribeToState subscribes to a state variable change.
// Shadow state inputs are automatically captured before the handler is called.
func (h *SubscriptionHelper) SubscribeToState(key string, handler func(key string, oldValue, newValue interface{})) error {
	// Register the subscription for input capture
	if h.registry != nil {
		h.registry.RegisterStateSubscription(h.pluginName, key)
	}

	sub, err := h.stateManager.Subscribe(key, func(k string, oldValue, newValue interface{}) {
		// Capture shadow state inputs BEFORE calling the handler
		h.captureInputs()

		handler(k, oldValue, newValue)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", key, err)
	}

	h.stateSubscriptions = append(h.stateSubscriptions, sub)
	return nil
}

// TrySubscribeToState subscribes to a state variable change, but doesn't fail if
// the variable doesn't exist. Useful for optional/dynamic variables.
// Returns true if subscription succeeded, false otherwise.
func (h *SubscriptionHelper) TrySubscribeToState(key string, handler func(key string, oldValue, newValue interface{})) bool {
	// Register the subscription for input capture even if subscription fails
	if h.registry != nil {
		h.registry.RegisterStateSubscription(h.pluginName, key)
	}

	sub, err := h.stateManager.Subscribe(key, func(k string, oldValue, newValue interface{}) {
		// Capture shadow state inputs BEFORE calling the handler
		h.captureInputs()

		handler(k, oldValue, newValue)
	})

	if err != nil {
		h.logger.Debug("Optional subscription failed (variable may not exist yet)",
			zap.String("key", key),
			zap.Error(err))
		return false
	}

	h.stateSubscriptions = append(h.stateSubscriptions, sub)
	return true
}

// GetHASubscriptions returns all HA subscriptions (for manual cleanup if needed)
func (h *SubscriptionHelper) GetHASubscriptions() []ha.Subscription {
	return h.haSubscriptions
}

// GetStateSubscriptions returns all state subscriptions (for manual cleanup if needed)
func (h *SubscriptionHelper) GetStateSubscriptions() []state.Subscription {
	return h.stateSubscriptions
}

// UnsubscribeAll cleans up all subscriptions
func (h *SubscriptionHelper) UnsubscribeAll() {
	for _, sub := range h.haSubscriptions {
		sub.Unsubscribe()
	}
	h.haSubscriptions = nil

	for _, sub := range h.stateSubscriptions {
		sub.Unsubscribe()
	}
	h.stateSubscriptions = nil
}
