package shadowstate

import "sync"

// SubscriptionRegistry tracks what each plugin subscribes to for automatic
// shadow state input capture. This eliminates the need for plugins to manually
// maintain lists of their subscriptions.
type SubscriptionRegistry struct {
	mu                 sync.RWMutex
	haSubscriptions    map[string][]string // pluginName -> []entityID
	stateSubscriptions map[string][]string // pluginName -> []stateKey
}

// NewSubscriptionRegistry creates a new subscription registry
func NewSubscriptionRegistry() *SubscriptionRegistry {
	return &SubscriptionRegistry{
		haSubscriptions:    make(map[string][]string),
		stateSubscriptions: make(map[string][]string),
	}
}

// RegisterHASubscription registers that a plugin subscribes to a Home Assistant entity
func (r *SubscriptionRegistry) RegisterHASubscription(pluginName, entityID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicates
	for _, existing := range r.haSubscriptions[pluginName] {
		if existing == entityID {
			return // Already registered
		}
	}

	r.haSubscriptions[pluginName] = append(r.haSubscriptions[pluginName], entityID)
}

// RegisterStateSubscription registers that a plugin subscribes to a state variable
func (r *SubscriptionRegistry) RegisterStateSubscription(pluginName, stateKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicates
	for _, existing := range r.stateSubscriptions[pluginName] {
		if existing == stateKey {
			return // Already registered
		}
	}

	r.stateSubscriptions[pluginName] = append(r.stateSubscriptions[pluginName], stateKey)
}

// GetHASubscriptions returns all Home Assistant entities a plugin subscribes to
func (r *SubscriptionRegistry) GetHASubscriptions(pluginName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent modification
	subs := r.haSubscriptions[pluginName]
	if subs == nil {
		return nil
	}

	result := make([]string, len(subs))
	copy(result, subs)
	return result
}

// GetStateSubscriptions returns all state variables a plugin subscribes to
func (r *SubscriptionRegistry) GetStateSubscriptions(pluginName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent modification
	subs := r.stateSubscriptions[pluginName]
	if subs == nil {
		return nil
	}

	result := make([]string, len(subs))
	copy(result, subs)
	return result
}

// UnregisterPlugin removes all subscription registrations for a plugin
func (r *SubscriptionRegistry) UnregisterPlugin(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.haSubscriptions, pluginName)
	delete(r.stateSubscriptions, pluginName)
}

// GetAllPlugins returns a list of all registered plugin names
func (r *SubscriptionRegistry) GetAllPlugins() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use a map to deduplicate
	plugins := make(map[string]struct{})
	for name := range r.haSubscriptions {
		plugins[name] = struct{}{}
	}
	for name := range r.stateSubscriptions {
		plugins[name] = struct{}{}
	}

	// Convert to slice
	result := make([]string, 0, len(plugins))
	for name := range plugins {
		result = append(result, name)
	}
	return result
}
