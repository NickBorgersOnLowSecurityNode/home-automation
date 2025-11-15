package state

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"homeautomation/internal/ha"

	"go.uber.org/zap"
)

// StateChangeHandler is called when a state variable changes
type StateChangeHandler func(key string, oldValue, newValue interface{})

// Subscription represents an active state change subscription
type Subscription interface {
	Unsubscribe()
}

type subscription struct {
	key     string
	manager *Manager
}

func (s *subscription) Unsubscribe() {
	s.manager.unsubscribe(s.key)
}

// Manager manages state synchronization with Home Assistant
type Manager struct {
	client      ha.HAClient
	logger      *zap.Logger
	cache       map[string]interface{}
	cacheMu     sync.RWMutex
	variables   map[string]StateVariable
	entityToKey map[string]string
	subscribers map[string][]StateChangeHandler
	subsMu      sync.RWMutex
	haSubs      map[string]ha.Subscription
}

// NewManager creates a new state manager
func NewManager(client ha.HAClient, logger *zap.Logger) *Manager {
	variables := VariablesByKey()
	entityToKey := make(map[string]string)

	for key, v := range variables {
		entityToKey[v.EntityID] = key
	}

	return &Manager{
		client:      client,
		logger:      logger,
		cache:       make(map[string]interface{}),
		variables:   variables,
		entityToKey: entityToKey,
		subscribers: make(map[string][]StateChangeHandler),
		haSubs:      make(map[string]ha.Subscription),
	}
}

// SyncFromHA reads all state variables from Home Assistant
func (m *Manager) SyncFromHA() error {
	m.logger.Info("Syncing state from Home Assistant...")

	states, err := m.client.GetAllStates()
	if err != nil {
		return fmt.Errorf("failed to get states: %w", err)
	}

	// Create a map for quick lookup
	stateMap := make(map[string]*ha.State)
	for _, state := range states {
		stateMap[state.EntityID] = state
	}

	// Sync each variable
	syncCount := 0
	localCount := 0
	for _, variable := range AllVariables {
		// Skip local-only variables (not synced with HA)
		if variable.LocalOnly {
			m.cacheMu.Lock()
			m.cache[variable.Key] = variable.Default
			m.cacheMu.Unlock()
			localCount++
			m.logger.Debug("Initialized local-only variable",
				zap.String("key", variable.Key))
			continue
		}

		state, ok := stateMap[variable.EntityID]
		if !ok {
			m.logger.Warn("Entity not found in HA, using default",
				zap.String("entity_id", variable.EntityID),
				zap.String("key", variable.Key))
			m.cacheMu.Lock()
			m.cache[variable.Key] = variable.Default
			m.cacheMu.Unlock()
			continue
		}

		// Parse and cache the value
		value, err := m.parseStateValue(state.State, variable.Type)
		if err != nil {
			m.logger.Error("Failed to parse state value",
				zap.String("entity_id", variable.EntityID),
				zap.String("key", variable.Key),
				zap.Error(err))
			m.cacheMu.Lock()
			m.cache[variable.Key] = variable.Default
			m.cacheMu.Unlock()
			continue
		}

		m.cacheMu.Lock()
		m.cache[variable.Key] = value
		m.cacheMu.Unlock()
		syncCount++

		// Subscribe to state changes
		if err := m.subscribeToEntity(variable.EntityID, variable.Key); err != nil {
			m.logger.Warn("Failed to subscribe to entity",
				zap.String("entity_id", variable.EntityID),
				zap.Error(err))
		}
	}

	m.logger.Info("State sync complete",
		zap.Int("synced", syncCount),
		zap.Int("local_only", localCount),
		zap.Int("total", len(AllVariables)))

	return nil
}

// parseStateValue parses a state string into the appropriate type
func (m *Manager) parseStateValue(stateStr string, varType StateType) (interface{}, error) {
	switch varType {
	case TypeBool:
		return stateStr == "on", nil
	case TypeNumber:
		return strconv.ParseFloat(stateStr, 64)
	case TypeString:
		return stateStr, nil
	case TypeJSON:
		var result interface{}
		if err := json.Unmarshal([]byte(stateStr), &result); err != nil {
			// If it's not valid JSON, return empty object
			return map[string]interface{}{}, nil
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown type: %s", varType)
	}
}

// subscribeToEntity subscribes to state changes for an entity
func (m *Manager) subscribeToEntity(entityID, key string) error {
	sub, err := m.client.SubscribeStateChanges(entityID, func(entity string, oldState, newState *ha.State) {
		if newState == nil {
			return
		}

		variable, ok := m.variables[key]
		if !ok {
			return
		}

		// Parse new value
		newValue, err := m.parseStateValue(newState.State, variable.Type)
		if err != nil {
			m.logger.Error("Failed to parse state change",
				zap.String("entity_id", entityID),
				zap.String("key", key),
				zap.Error(err))
			return
		}

		// Update cache
		m.cacheMu.Lock()
		oldValue := m.cache[key]
		m.cache[key] = newValue
		m.cacheMu.Unlock()

		m.logger.Debug("State changed",
			zap.String("key", key),
			zap.Any("old", oldValue),
			zap.Any("new", newValue))

		// Notify subscribers
		m.notifySubscribers(key, oldValue, newValue)
	})

	if err != nil {
		return err
	}

	m.haSubs[entityID] = sub
	return nil
}

// notifySubscribers notifies all subscribers of a state change
func (m *Manager) notifySubscribers(key string, oldValue, newValue interface{}) {
	m.subsMu.RLock()
	handlers := m.subscribers[key]
	m.subsMu.RUnlock()

	for _, handler := range handlers {
		go handler(key, oldValue, newValue)
	}
}

// GetBool retrieves a boolean state variable
func (m *Manager) GetBool(key string) (bool, error) {
	variable, ok := m.variables[key]
	if !ok {
		return false, fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeBool {
		return false, fmt.Errorf("variable %s is not a boolean", key)
	}

	m.cacheMu.RLock()
	value, ok := m.cache[key]
	m.cacheMu.RUnlock()

	if !ok {
		return variable.Default.(bool), nil
	}

	boolValue, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("cached value for %s is not a boolean", key)
	}

	return boolValue, nil
}

// SetBool sets a boolean state variable
func (m *Manager) SetBool(key string, value bool) error {
	variable, ok := m.variables[key]
	if !ok {
		return fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeBool {
		return fmt.Errorf("variable %s is not a boolean", key)
	}

	// Update cache
	m.cacheMu.Lock()
	oldValue := m.cache[key]
	m.cache[key] = value
	m.cacheMu.Unlock()

	// Skip HA sync for local-only variables
	if variable.LocalOnly {
		return nil
	}

	// Sync to HA
	entityName := extractEntityName(variable.EntityID)
	if err := m.client.SetInputBoolean(entityName, value); err != nil {
		// Rollback cache on error
		m.cacheMu.Lock()
		m.cache[key] = oldValue
		m.cacheMu.Unlock()
		return fmt.Errorf("failed to set HA value: %w", err)
	}

	return nil
}

// GetString retrieves a string state variable
func (m *Manager) GetString(key string) (string, error) {
	variable, ok := m.variables[key]
	if !ok {
		return "", fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeString {
		return "", fmt.Errorf("variable %s is not a string", key)
	}

	m.cacheMu.RLock()
	value, ok := m.cache[key]
	m.cacheMu.RUnlock()

	if !ok {
		return variable.Default.(string), nil
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("cached value for %s is not a string", key)
	}

	return strValue, nil
}

// SetString sets a string state variable
func (m *Manager) SetString(key string, value string) error {
	variable, ok := m.variables[key]
	if !ok {
		return fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeString {
		return fmt.Errorf("variable %s is not a string", key)
	}

	// Update cache
	m.cacheMu.Lock()
	oldValue := m.cache[key]
	m.cache[key] = value
	m.cacheMu.Unlock()

	// Skip HA sync for local-only variables
	if variable.LocalOnly {
		return nil
	}

	// Sync to HA
	entityName := extractEntityName(variable.EntityID)
	if err := m.client.SetInputText(entityName, value); err != nil {
		// Rollback cache on error
		m.cacheMu.Lock()
		m.cache[key] = oldValue
		m.cacheMu.Unlock()
		return fmt.Errorf("failed to set HA value: %w", err)
	}

	return nil
}

// GetNumber retrieves a number state variable
func (m *Manager) GetNumber(key string) (float64, error) {
	variable, ok := m.variables[key]
	if !ok {
		return 0, fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeNumber {
		return 0, fmt.Errorf("variable %s is not a number", key)
	}

	m.cacheMu.RLock()
	value, ok := m.cache[key]
	m.cacheMu.RUnlock()

	if !ok {
		return variable.Default.(float64), nil
	}

	numValue, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("cached value for %s is not a number", key)
	}

	return numValue, nil
}

// SetNumber sets a number state variable
func (m *Manager) SetNumber(key string, value float64) error {
	variable, ok := m.variables[key]
	if !ok {
		return fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeNumber {
		return fmt.Errorf("variable %s is not a number", key)
	}

	// Update cache
	m.cacheMu.Lock()
	oldValue := m.cache[key]
	m.cache[key] = value
	m.cacheMu.Unlock()

	// Skip HA sync for local-only variables
	if variable.LocalOnly {
		return nil
	}

	// Sync to HA
	entityName := extractEntityName(variable.EntityID)
	if err := m.client.SetInputNumber(entityName, value); err != nil {
		// Rollback cache on error
		m.cacheMu.Lock()
		m.cache[key] = oldValue
		m.cacheMu.Unlock()
		return fmt.Errorf("failed to set HA value: %w", err)
	}

	return nil
}

// GetJSON retrieves a JSON state variable
func (m *Manager) GetJSON(key string, target interface{}) error {
	variable, ok := m.variables[key]
	if !ok {
		return fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeJSON {
		return fmt.Errorf("variable %s is not JSON", key)
	}

	m.cacheMu.RLock()
	value, ok := m.cache[key]
	m.cacheMu.RUnlock()

	if !ok {
		// Parse default value
		return json.Unmarshal([]byte(variable.Default.(string)), target)
	}

	// Marshal and unmarshal to convert to target type
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cached value: %w", err)
	}

	return json.Unmarshal(jsonBytes, target)
}

// SetJSON sets a JSON state variable
func (m *Manager) SetJSON(key string, value interface{}) error {
	variable, ok := m.variables[key]
	if !ok {
		return fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeJSON {
		return fmt.Errorf("variable %s is not JSON", key)
	}

	// Update cache
	m.cacheMu.Lock()
	oldValue := m.cache[key]
	m.cache[key] = value
	m.cacheMu.Unlock()

	// Skip HA sync for local-only variables
	if variable.LocalOnly {
		return nil
	}

	// Convert to JSON string for HA
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		// Rollback cache on error
		m.cacheMu.Lock()
		m.cache[key] = oldValue
		m.cacheMu.Unlock()
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Sync to HA
	entityName := extractEntityName(variable.EntityID)
	if err := m.client.SetInputText(entityName, string(jsonBytes)); err != nil {
		// Rollback cache on error
		m.cacheMu.Lock()
		m.cache[key] = oldValue
		m.cacheMu.Unlock()
		return fmt.Errorf("failed to set HA value: %w", err)
	}

	return nil
}

// CompareAndSwapBool atomically compares and swaps a boolean value
func (m *Manager) CompareAndSwapBool(key string, old, new bool) (bool, error) {
	variable, ok := m.variables[key]
	if !ok {
		return false, fmt.Errorf("variable %s not found", key)
	}

	if variable.Type != TypeBool {
		return false, fmt.Errorf("variable %s is not a boolean", key)
	}

	m.cacheMu.Lock()

	currentValue, ok := m.cache[key]
	if !ok {
		currentValue = variable.Default
	}

	currentBool, ok := currentValue.(bool)
	if !ok {
		m.cacheMu.Unlock()
		return false, fmt.Errorf("cached value for %s is not a boolean", key)
	}

	if currentBool != old {
		m.cacheMu.Unlock()
		return false, nil
	}

	// Update cache (still holding lock)
	m.cache[key] = new

	// Release lock before calling HA client to avoid deadlock
	m.cacheMu.Unlock()

	// Sync to HA
	entityName := extractEntityName(variable.EntityID)
	if err := m.client.SetInputBoolean(entityName, new); err != nil {
		// Rollback on error
		m.cacheMu.Lock()
		m.cache[key] = old
		m.cacheMu.Unlock()
		return false, fmt.Errorf("failed to set HA value: %w", err)
	}

	return true, nil
}

// Subscribe subscribes to state changes for a variable
func (m *Manager) Subscribe(key string, handler StateChangeHandler) (Subscription, error) {
	if _, ok := m.variables[key]; !ok {
		return nil, fmt.Errorf("variable %s not found", key)
	}

	m.subsMu.Lock()
	m.subscribers[key] = append(m.subscribers[key], handler)
	m.subsMu.Unlock()

	return &subscription{
		key:     key,
		manager: m,
	}, nil
}

// unsubscribe removes all subscriptions for a key
func (m *Manager) unsubscribe(key string) {
	m.subsMu.Lock()
	delete(m.subscribers, key)
	m.subsMu.Unlock()
}

// GetAllValues returns all cached values
func (m *Manager) GetAllValues() map[string]interface{} {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	values := make(map[string]interface{})
	for k, v := range m.cache {
		values[k] = v
	}
	return values
}

// extractEntityName extracts the entity name from full entity ID
// e.g., "input_boolean.nick_home" -> "nick_home"
func extractEntityName(entityID string) string {
	for i := len(entityID) - 1; i >= 0; i-- {
		if entityID[i] == '.' {
			return entityID[i+1:]
		}
	}
	return entityID
}
