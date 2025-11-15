package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MockHAClient provides HTTP client for interacting with Mock HA service
type MockHAClient struct {
	baseURL string
	client  *http.Client
}

// NewMockHAClient creates a new Mock HA client
func NewMockHAClient(url string) *MockHAClient {
	return &MockHAClient{
		baseURL: url,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// InjectEvent injects a state change event for testing
func (m *MockHAClient) InjectEvent(entityID string, newState interface{}) error {
	payload := map[string]interface{}{
		"entity_id": entityID,
		"new_state": newState,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	resp, err := m.client.Post(
		m.baseURL+"/test/inject_event",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("post error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("inject event failed: status %d", resp.StatusCode)
	}

	return nil
}

// ServiceCallFilter filters service calls by domain and service
type ServiceCallFilter struct {
	Domain  string
	Service string
}

// ServiceCall represents a recorded service call
type ServiceCall struct {
	Domain      string                 `json:"domain"`
	Service     string                 `json:"service"`
	ServiceData map[string]interface{} `json:"service_data"`
	Timestamp   time.Time              `json:"timestamp"`
}

// WaitForServiceCalls waits for service calls matching the filter
func (m *MockHAClient) WaitForServiceCalls(filter ServiceCallFilter, timeout time.Duration) ([]ServiceCall, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		calls, err := m.GetServiceCalls(filter)
		if err != nil {
			return nil, err
		}

		if len(calls) > 0 {
			return calls, nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return []ServiceCall{}, nil
}

// GetServiceCalls retrieves recorded service calls matching the filter
func (m *MockHAClient) GetServiceCalls(filter ServiceCallFilter) ([]ServiceCall, error) {
	url := fmt.Sprintf("%s/test/service_calls?domain=%s&service=%s",
		m.baseURL, filter.Domain, filter.Service)

	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get error: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Calls []ServiceCall `json:"calls"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	return result.Calls, nil
}

// GetEntityState retrieves the current state of an entity
func (m *MockHAClient) GetEntityState(entityID string) (interface{}, error) {
	url := fmt.Sprintf("%s/test/entity/%s", m.baseURL, entityID)

	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	var result struct {
		State interface{} `json:"state"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	return result.State, nil
}

// Reset clears all recorded service calls and events
func (m *MockHAClient) Reset() error {
	resp, err := m.client.Post(m.baseURL+"/test/reset", "", nil)
	if err != nil {
		return fmt.Errorf("reset error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("reset failed: status %d", resp.StatusCode)
	}

	return nil
}

// HealthCheck verifies the Mock HA service is ready
func (m *MockHAClient) HealthCheck() error {
	url := fmt.Sprintf("%s/test/health", m.baseURL)

	resp, err := m.client.Get(url)
	if err != nil {
		return fmt.Errorf("health check error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode error: %w", err)
	}

	if result.Status != "ready" {
		return fmt.Errorf("service not ready: %s", result.Status)
	}

	return nil
}

// WaitForReady waits for the Mock HA service to be ready
func (m *MockHAClient) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if err := m.HealthCheck(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for Mock HA to be ready")
}
