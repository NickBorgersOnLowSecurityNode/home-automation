package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

func TestHandleGetState(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create mock HA client
	mockClient := ha.NewMockClient()

	// Create state manager
	stateManager := state.NewManager(mockClient, logger, false)

	// Set some test values
	stateManager.SetBool("isNickHome", true)
	stateManager.SetBool("isCarolineHome", false)
	stateManager.SetNumber("alarmTime", 7.5)
	stateManager.SetString("dayPhase", "morning")
	stateManager.SetString("musicPlaybackType", "default")

	// Create API server
	server := NewServer(stateManager, logger, 8080)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	w := httptest.NewRecorder()

	// Handle request
	server.handleGetState(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Parse response
	var response StateResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify boolean values
	if !response.Booleans["isNickHome"] {
		t.Error("Expected isNickHome to be true")
	}
	if response.Booleans["isCarolineHome"] {
		t.Error("Expected isCarolineHome to be false")
	}

	// Verify number values
	if response.Numbers["alarmTime"] != 7.5 {
		t.Errorf("Expected alarmTime to be 7.5, got %f", response.Numbers["alarmTime"])
	}

	// Verify string values
	if response.Strings["dayPhase"] != "morning" {
		t.Errorf("Expected dayPhase to be 'morning', got '%s'", response.Strings["dayPhase"])
	}
	if response.Strings["musicPlaybackType"] != "default" {
		t.Errorf("Expected musicPlaybackType to be 'default', got '%s'", response.Strings["musicPlaybackType"])
	}

	// Verify all expected keys are present (at least some of them)
	expectedBoolKeys := []string{"isNickHome", "isCarolineHome", "isToriHere", "isAnyoneHome"}
	for _, key := range expectedBoolKeys {
		if _, ok := response.Booleans[key]; !ok {
			t.Errorf("Expected boolean key %s to be present", key)
		}
	}

	expectedNumberKeys := []string{"alarmTime", "remainingSolarGeneration", "thisHourSolarGeneration"}
	for _, key := range expectedNumberKeys {
		if _, ok := response.Numbers[key]; !ok {
			t.Errorf("Expected number key %s to be present", key)
		}
	}

	expectedStringKeys := []string{"dayPhase", "sunevent", "musicPlaybackType"}
	for _, key := range expectedStringKeys {
		if _, ok := response.Strings[key]; !ok {
			t.Errorf("Expected string key %s to be present", key)
		}
	}
}

func TestHandleGetStateMethodNotAllowed(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	server := NewServer(stateManager, logger, 8080)

	// Test POST method (should be rejected)
	req := httptest.NewRequest(http.MethodPost, "/api/state", nil)
	w := httptest.NewRecorder()

	server.handleGetState(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleHealth(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := ha.NewMockClient()
	stateManager := state.NewManager(mockClient, logger, false)
	server := NewServer(stateManager, logger, 8080)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
}
