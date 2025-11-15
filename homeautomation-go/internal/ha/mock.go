package ha

import (
	"fmt"
	"sync"
	"time"
)

// MockClient implements HAClient interface for testing
type MockClient struct {
	states      map[string]*State
	statesMu    sync.RWMutex
	subscribers map[string][]StateChangeHandler
	subsMu      sync.RWMutex
	connected   bool
	connMu      sync.RWMutex
	serviceCalls []ServiceCall
	callsMu      sync.Mutex
}

// ServiceCall records a service call for testing
type ServiceCall struct {
	Domain  string
	Service string
	Data    map[string]interface{}
	Time    time.Time
}

// NewMockClient creates a new mock HA client
func NewMockClient() *MockClient {
	return &MockClient{
		states:      make(map[string]*State),
		subscribers: make(map[string][]StateChangeHandler),
		serviceCalls: make([]ServiceCall, 0),
		connected:   false,
	}
}

// Connect simulates connecting to Home Assistant
func (m *MockClient) Connect() error {
	m.connMu.Lock()
	defer m.connMu.Unlock()

	if m.connected {
		return fmt.Errorf("already connected")
	}

	m.connected = true
	return nil
}

// Disconnect simulates disconnecting
func (m *MockClient) Disconnect() error {
	m.connMu.Lock()
	defer m.connMu.Unlock()

	m.connected = false
	return nil
}

// IsConnected returns connection status
func (m *MockClient) IsConnected() bool {
	m.connMu.RLock()
	defer m.connMu.RUnlock()
	return m.connected
}

// GetState retrieves a mock state
func (m *MockClient) GetState(entityID string) (*State, error) {
	m.statesMu.RLock()
	defer m.statesMu.RUnlock()

	state, ok := m.states[entityID]
	if !ok {
		return nil, fmt.Errorf("entity %s not found", entityID)
	}

	return state, nil
}

// GetAllStates retrieves all mock states
func (m *MockClient) GetAllStates() ([]*State, error) {
	m.statesMu.RLock()
	defer m.statesMu.RUnlock()

	states := make([]*State, 0, len(m.states))
	for _, state := range m.states {
		states = append(states, state)
	}

	return states, nil
}

// CallService records a service call
func (m *MockClient) CallService(domain, service string, data map[string]interface{}) error {
	m.callsMu.Lock()
	m.serviceCalls = append(m.serviceCalls, ServiceCall{
		Domain:  domain,
		Service: service,
		Data:    data,
		Time:    time.Now(),
	})
	m.callsMu.Unlock()

	// Update mock state based on service call
	if entityID, ok := data["entity_id"].(string); ok {
		m.updateStateFromServiceCall(entityID, domain, service, data)
	}

	return nil
}

// SubscribeStateChanges subscribes to state changes
func (m *MockClient) SubscribeStateChanges(entityID string, handler StateChangeHandler) (Subscription, error) {
	m.subsMu.Lock()
	m.subscribers[entityID] = append(m.subscribers[entityID], handler)
	m.subsMu.Unlock()

	return &subscription{
		id:     entityID,
		client: &Client{}, // Dummy client for subscription
	}, nil
}

// SetInputBoolean sets a mock input_boolean
func (m *MockClient) SetInputBoolean(name string, value bool) error {
	service := "turn_off"
	if value {
		service = "turn_on"
	}

	return m.CallService("input_boolean", service, map[string]interface{}{
		"entity_id": fmt.Sprintf("input_boolean.%s", name),
	})
}

// SetInputNumber sets a mock input_number
func (m *MockClient) SetInputNumber(name string, value float64) error {
	return m.CallService("input_number", "set_value", map[string]interface{}{
		"entity_id": fmt.Sprintf("input_number.%s", name),
		"value":     value,
	})
}

// SetInputText sets a mock input_text
func (m *MockClient) SetInputText(name string, value string) error {
	return m.CallService("input_text", "set_value", map[string]interface{}{
		"entity_id": fmt.Sprintf("input_text.%s", name),
		"value":     value,
	})
}

// SetState sets a mock state (for testing)
func (m *MockClient) SetState(entityID string, stateValue string, attributes map[string]interface{}) {
	m.statesMu.Lock()
	defer m.statesMu.Unlock()

	now := time.Now()
	oldState := m.states[entityID]

	newState := &State{
		EntityID:    entityID,
		State:       stateValue,
		Attributes:  attributes,
		LastChanged: now,
		LastUpdated: now,
	}

	m.states[entityID] = newState

	// Notify subscribers
	m.notifySubscribers(entityID, oldState, newState)
}

// SimulateStateChange simulates a state change event
func (m *MockClient) SimulateStateChange(entityID string, newStateValue string) {
	m.statesMu.Lock()
	oldState := m.states[entityID]

	now := time.Now()
	newState := &State{
		EntityID:    entityID,
		State:       newStateValue,
		Attributes:  make(map[string]interface{}),
		LastChanged: now,
		LastUpdated: now,
	}

	if oldState != nil {
		newState.Attributes = oldState.Attributes
	}

	m.states[entityID] = newState
	m.statesMu.Unlock()

	// Notify subscribers
	m.notifySubscribers(entityID, oldState, newState)
}

// GetServiceCalls returns all recorded service calls
func (m *MockClient) GetServiceCalls() []ServiceCall {
	m.callsMu.Lock()
	defer m.callsMu.Unlock()

	calls := make([]ServiceCall, len(m.serviceCalls))
	copy(calls, m.serviceCalls)
	return calls
}

// ClearServiceCalls clears the service call history
func (m *MockClient) ClearServiceCalls() {
	m.callsMu.Lock()
	defer m.callsMu.Unlock()
	m.serviceCalls = make([]ServiceCall, 0)
}

// updateStateFromServiceCall updates state based on a service call
func (m *MockClient) updateStateFromServiceCall(entityID, domain, service string, data map[string]interface{}) {
	m.statesMu.Lock()
	defer m.statesMu.Unlock()

	oldState := m.states[entityID]
	now := time.Now()

	var newStateValue string
	attributes := make(map[string]interface{})

	if oldState != nil {
		newStateValue = oldState.State
		attributes = oldState.Attributes
	}

	switch domain {
	case "input_boolean":
		if service == "turn_on" {
			newStateValue = "on"
		} else if service == "turn_off" {
			newStateValue = "off"
		}
	case "input_number":
		if value, ok := data["value"].(float64); ok {
			newStateValue = fmt.Sprintf("%.2f", value)
		}
	case "input_text":
		if value, ok := data["value"].(string); ok {
			newStateValue = value
		}
	}

	newState := &State{
		EntityID:    entityID,
		State:       newStateValue,
		Attributes:  attributes,
		LastChanged: now,
		LastUpdated: now,
	}

	m.states[entityID] = newState
	m.statesMu.Unlock()

	// Notify subscribers
	m.notifySubscribers(entityID, oldState, newState)
	m.statesMu.Lock()
}

// notifySubscribers notifies all subscribers of a state change
func (m *MockClient) notifySubscribers(entityID string, oldState, newState *State) {
	m.subsMu.RLock()
	handlers := m.subscribers[entityID]
	m.subsMu.RUnlock()

	for _, handler := range handlers {
		handler(entityID, oldState, newState)
	}
}
