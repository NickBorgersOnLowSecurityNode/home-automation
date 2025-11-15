# Containerized Integration Testing Strategy

**Version:** 1.0
**Date:** 2025-11-15
**Status:** Design Phase

---

## Table of Contents

1. [Overview](#overview)
2. [Testing Philosophy](#testing-philosophy)
3. [Test Architecture](#test-architecture)
4. [Mock Home Assistant Service](#mock-home-assistant-service)
5. [Test Environment Setup](#test-environment-setup)
6. [Test Scenarios](#test-scenarios)
7. [Test Implementation Guide](#test-implementation-guide)
8. [CI/CD Integration](#cicd-integration)
9. [Performance Testing](#performance-testing)
10. [Appendices](#appendices)

---

## Overview

### Purpose

This document defines the containerized integration testing strategy for the Golang home automation system. Integration tests verify that the entire system works correctly when all components interact together, including:

- Home Assistant WebSocket communication
- State synchronization across 33 variables
- Plugin event handling and coordination
- Configuration file loading and hot-reload
- Service call execution
- Error handling and recovery

### Goals

1. **Comprehensive Coverage** - Test all critical integration points
2. **Isolation** - Tests run in isolated containers without external dependencies
3. **Repeatability** - Tests produce consistent results across environments
4. **Fast Feedback** - Complete test suite runs in under 5 minutes
5. **CI/CD Ready** - Automated execution in continuous integration pipelines
6. **Parallel Testing** - Compare behavior with Node-RED during migration

### Test Levels

```
┌─────────────────────────────────────────────────────────────┐
│                    Testing Pyramid                          │
│                                                              │
│                  ┌──────────────────┐                        │
│                  │  E2E Tests       │  ← Parallel Node-RED  │
│                  │  (Manual/Staged) │     comparison        │
│                  └──────────────────┘                        │
│              ┌────────────────────────┐                      │
│              │  Integration Tests     │  ← THIS DOCUMENT    │
│              │  (Containerized)       │                      │
│              └────────────────────────┘                      │
│          ┌──────────────────────────────┐                    │
│          │     Unit Tests               │                    │
│          │     (Component-level)        │                    │
│          └──────────────────────────────┘                    │
└─────────────────────────────────────────────────────────────┘
```

---

## Testing Philosophy

### What Integration Tests Cover

**In Scope:**
- System startup and initialization sequence
- WebSocket connection establishment and recovery
- State synchronization (HA ↔ Golang)
- Event bus message routing
- Plugin initialization and lifecycle
- Inter-plugin communication
- Configuration loading and hot-reload
- Service call execution
- Error handling and retry logic
- Graceful shutdown

**Out of Scope (Unit Tests):**
- Individual plugin business logic
- Configuration file parsing
- State manager internal data structures
- Event bus implementation details

**Out of Scope (E2E Tests):**
- Actual device control (Sonos, Hue, etc.)
- Real Home Assistant instance
- HomeKit integration
- Production deployment scenarios

### Test Principles

1. **Mock External Dependencies** - Use a mock Home Assistant instead of real HA
2. **Verify Behavior, Not Implementation** - Test observable outcomes
3. **Fail Fast** - Tests should fail quickly when something breaks
4. **Self-Contained** - Each test sets up and tears down its own state
5. **Deterministic** - No flaky tests due to timing or randomness
6. **Readable** - Test names clearly describe what they verify

---

## Test Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│               Docker Compose Test Environment               │
│                                                              │
│  ┌────────────────────┐         ┌──────────────────────┐   │
│  │  homeautomation    │◄────────┤  Mock HA Service     │   │
│  │  (System Under     │ WebSocket│  (Python/Go)         │   │
│  │   Test)            │         │                      │   │
│  │                    │         │  - WebSocket server  │   │
│  │  - All plugins     │         │  - 33 input helpers  │   │
│  │  - Config files    │         │  - Event simulation  │   │
│  │  - Event bus       │         │  - Service recording │   │
│  └────────┬───────────┘         └──────────────────────┘   │
│           │                                                  │
│           │ HTTP                                             │
│           ▼                                                  │
│  ┌────────────────────┐         ┌──────────────────────┐   │
│  │  Test Orchestrator │         │  Test Data Volume    │   │
│  │  (Go test binary)  │         │                      │   │
│  │                    │         │  - Config files      │   │
│  │  - Scenario setup  │◄────────┤  - Test fixtures     │   │
│  │  - Assertions      │         │  - Expected outputs  │   │
│  │  - Reporting       │         │                      │   │
│  └────────────────────┘         └──────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Test Flow

```
1. Docker Compose Up
   ↓
2. Mock HA Service Starts
   ↓
3. Initialize 33 input helpers with test data
   ↓
4. Homeautomation Service Starts
   ↓
5. Wait for WebSocket connection
   ↓
6. Test Orchestrator Runs Test Suite
   ↓
7. Send events to Mock HA → Observe homeautomation behavior
   ↓
8. Send commands to homeautomation → Verify HA service calls
   ↓
9. Collect results and assertions
   ↓
10. Tear down containers
```

---

## Mock Home Assistant Service

### Requirements

The mock HA service simulates Home Assistant's WebSocket API for testing purposes.

**Core Features:**
- WebSocket server accepting HA API protocol
- Maintain state for 33 input helpers
- Accept service call requests
- Emit state_changed events
- Record all service calls for verification
- Support event injection for testing

### Implementation Options

#### Option 1: Python-based Mock (Recommended)

**Advantages:**
- Can reuse Home Assistant's actual WebSocket protocol libraries
- Easier to implement HA-specific quirks
- Faster to develop

**Disadvantages:**
- Additional language dependency

#### Option 2: Go-based Mock

**Advantages:**
- Single language for entire test stack
- Better performance

**Disadvantages:**
- More work to replicate HA protocol exactly
- Harder to maintain protocol compatibility

### Mock HA API Surface

```python
# Python pseudo-code for Mock HA Service

class MockHomeAssistant:
    def __init__(self):
        self.entities = {}
        self.service_calls = []
        self.event_handlers = []

    def initialize_entities(self, entity_configs):
        """Set up 33 input helpers with initial values"""
        for entity in entity_configs:
            self.entities[entity['id']] = entity['initial_value']

    def handle_websocket_message(self, msg):
        """Process WebSocket messages from homeautomation system"""
        if msg['type'] == 'subscribe_events':
            return self.handle_subscribe(msg)
        elif msg['type'] == 'call_service':
            return self.handle_service_call(msg)
        elif msg['type'] == 'get_states':
            return self.handle_get_states(msg)

    def handle_service_call(self, msg):
        """Record service call and update entity state"""
        self.service_calls.append(msg)

        # Update entity if it's an input helper set
        if msg['domain'] == 'input_boolean':
            entity_id = msg['service_data']['entity_id']
            if msg['service'] == 'turn_on':
                self.entities[entity_id] = True
            elif msg['service'] == 'turn_off':
                self.entities[entity_id] = False

            # Emit state_changed event
            self.emit_state_changed(entity_id, self.entities[entity_id])

    def emit_state_changed(self, entity_id, new_state):
        """Send state_changed event to all subscribers"""
        event = {
            'type': 'event',
            'event': {
                'event_type': 'state_changed',
                'data': {
                    'entity_id': entity_id,
                    'old_state': {...},
                    'new_state': {'state': new_state, ...}
                }
            }
        }
        self.broadcast(event)

    def inject_event(self, entity_id, new_state):
        """Test helper: manually trigger a state change"""
        self.entities[entity_id] = new_state
        self.emit_state_changed(entity_id, new_state)

    def get_service_calls(self, filter=None):
        """Test helper: retrieve recorded service calls"""
        if filter:
            return [c for c in self.service_calls if matches(c, filter)]
        return self.service_calls

    def reset(self):
        """Clear all recorded calls and events"""
        self.service_calls = []
```

### Mock HA HTTP API for Test Control

```http
# Test orchestrator can inject events and query state via HTTP

POST /test/inject_event
{
  "entity_id": "input_boolean.nick_home",
  "new_state": true
}

GET /test/service_calls?domain=media_player&service=play_media
{
  "calls": [
    {
      "domain": "media_player",
      "service": "play_media",
      "service_data": {
        "entity_id": "media_player.living_room",
        "media_content_id": "spotify:playlist:...",
        "media_content_type": "playlist"
      },
      "timestamp": "2025-11-15T10:30:00Z"
    }
  ]
}

POST /test/reset
# Clears all recorded calls and events

GET /test/health
{
  "status": "ready",
  "connected_clients": 1,
  "entities": 33
}
```

---

## Test Environment Setup

### Directory Structure

```
tests/
├── integration/
│   ├── docker-compose.test.yml      # Test environment definition
│   ├── Dockerfile.mockha            # Mock HA service container
│   ├── Dockerfile.homeautomation    # System under test
│   ├── mockha/
│   │   ├── main.py                  # Mock HA service implementation
│   │   ├── requirements.txt
│   │   └── test_fixtures.json       # 33 entities initial state
│   ├── testdata/
│   │   ├── configs/                 # Test YAML configs
│   │   │   ├── music_config.yaml
│   │   │   ├── hue_config.yaml
│   │   │   ├── schedule_config.yaml
│   │   │   └── energy_config.yaml
│   │   └── scenarios/               # Test scenario definitions
│   │       ├── presence_arrival.json
│   │       ├── music_mode_change.json
│   │       └── energy_state.json
│   ├── tests/
│   │   ├── startup_test.go          # Startup sequence tests
│   │   ├── state_sync_test.go       # State synchronization tests
│   │   ├── plugin_test.go           # Plugin integration tests
│   │   ├── config_reload_test.go    # Hot config reload tests
│   │   └── recovery_test.go         # Error recovery tests
│   ├── helpers/
│   │   ├── mock_client.go           # HTTP client for Mock HA
│   │   └── assertions.go            # Custom test assertions
│   └── run_tests.sh                 # Test runner script
└── parallel_testing/
    └── nodered_comparison.md        # Parallel testing guide
```

### Docker Compose Configuration

```yaml
# tests/integration/docker-compose.test.yml

version: '3.8'

services:
  # Mock Home Assistant service
  mockha:
    build:
      context: .
      dockerfile: Dockerfile.mockha
    container_name: test-mockha
    ports:
      - "8123:8123"  # WebSocket + HTTP API
    volumes:
      - ./mockha:/app
      - ./testdata:/testdata:ro
    environment:
      - LOG_LEVEL=debug
      - FIXTURES_FILE=/testdata/test_fixtures.json
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8123/test/health"]
      interval: 2s
      timeout: 1s
      retries: 5

  # System under test
  homeautomation:
    build:
      context: ../..
      dockerfile: Dockerfile
    container_name: test-homeautomation
    depends_on:
      mockha:
        condition: service_healthy
    environment:
      - HA_URL=http://mockha:8123
      - HA_TOKEN=test_token
      - CONFIG_DIR=/configs
      - LOG_LEVEL=debug
    volumes:
      - ./testdata/configs:/configs:ro
      - test-logs:/logs
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 2s
      timeout: 1s
      retries: 10

volumes:
  test-logs:

networks:
  default:
    name: homeautomation-test
```

### Mock HA Dockerfile

```dockerfile
# tests/integration/Dockerfile.mockha

FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY mockha/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy mock service
COPY mockha/ .

# Expose WebSocket + HTTP port
EXPOSE 8123

CMD ["python", "main.py"]
```

### Test Fixtures

```json
// tests/integration/testdata/test_fixtures.json

{
  "entities": [
    {
      "entity_id": "input_boolean.nick_home",
      "state": false,
      "attributes": {}
    },
    {
      "entity_id": "input_boolean.caroline_home",
      "state": false,
      "attributes": {}
    },
    {
      "entity_id": "input_boolean.tori_here",
      "state": false,
      "attributes": {}
    },
    {
      "entity_id": "input_boolean.master_asleep",
      "state": false,
      "attributes": {}
    },
    {
      "entity_id": "input_text.day_phase",
      "state": "morning",
      "attributes": {}
    },
    {
      "entity_id": "input_text.music_playback_type",
      "state": "none",
      "attributes": {}
    },
    {
      "entity_id": "input_number.alarm_time",
      "state": 0,
      "attributes": {
        "min": 0,
        "max": 86400000
      }
    }
    // ... all 33 entities
  ]
}
```

---

## Test Scenarios

### Scenario Categories

1. **System Lifecycle**
   - Startup and initialization
   - Graceful shutdown
   - Crash recovery

2. **State Synchronization**
   - HA → Golang sync on startup
   - Golang → HA sync on state change
   - HA → Golang sync on external change
   - Conflict resolution
   - Periodic full sync

3. **Plugin Integration**
   - Event routing between plugins
   - Service call execution
   - Configuration usage
   - Error handling

4. **Configuration Management**
   - YAML loading
   - Hot reload
   - Invalid config handling

5. **WebSocket Connectivity**
   - Connection establishment
   - Auto-reconnect on disconnect
   - Message queuing during downtime
   - Heartbeat/keepalive

### Example Test Scenario: Presence Arrival Flow

**Scenario:** Nick arrives home, triggering music playback

**Test Steps:**
1. Initialize: Nick not home, music off, daytime
2. Inject event: `input_boolean.nick_home` → `true`
3. Verify state sync: `isNickHome` → `true` in Golang cache
4. Verify Music plugin triggered
5. Verify service calls:
   - `media_player.play_media` called with correct playlist
   - `media_player.join` called to group speakers
   - `media_player.volume_set` called with appropriate volume
6. Verify state update: `musicPlaybackType` → `"day"`
7. Verify TTS announcement: `tts.google_say` called with arrival message

**Expected Service Calls:**
```json
[
  {
    "domain": "media_player",
    "service": "play_media",
    "service_data": {
      "entity_id": "media_player.living_room",
      "media_content_id": "spotify:playlist:37i9dQZF1DX...",
      "media_content_type": "playlist"
    }
  },
  {
    "domain": "media_player",
    "service": "join",
    "service_data": {
      "entity_id": "media_player.living_room",
      "group_members": ["media_player.kitchen", "media_player.office"]
    }
  },
  {
    "domain": "tts",
    "service": "google_say",
    "service_data": {
      "entity_id": "media_player.living_room",
      "message": "Welcome home Nick"
    }
  }
]
```

### Example Test Scenario: Energy State Change

**Scenario:** Battery drops to low level, triggering load shedding

**Test Steps:**
1. Initialize: Battery at 80%, thermostat in schedule mode
2. Inject event: `sensor.battery_percentage` → `15`
3. Verify Energy plugin calculates: `currentEnergyLevel` → `"red"`
4. Verify Load Shedding plugin triggered
5. Verify service calls:
   - `climate.set_hvac_mode` → `"heat"` (hold mode)
   - `climate.set_temperature` with widened range
6. Wait 1 minute (rate limiting)
7. Inject event: `sensor.battery_percentage` → `85`
8. Verify Energy plugin calculates: `currentEnergyLevel` → `"green"`
9. Verify service calls:
   - `climate.set_hvac_mode` → `"auto"` (schedule mode)
   - `climate.set_temperature` with comfort range

### Example Test Scenario: Config Hot Reload

**Scenario:** Music config file updated while system running

**Test Steps:**
1. Initialize: System running, music playing in "day" mode
2. Modify `music_config.yaml`: Change "day" mode playlist URI
3. Trigger file change event (inotify simulation)
4. Verify config reload event published
5. Verify Music plugin receives reload event
6. Change music mode to trigger reload
7. Verify new playlist URI used in `media_player.play_media` call

---

## Test Implementation Guide

### Test Structure

```go
// tests/integration/tests/presence_test.go

package integration_test

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestPresenceArrival_TriggersMusic(t *testing.T) {
    // Setup
    mockHA := NewMockHAClient("http://localhost:8123")
    defer mockHA.Reset()

    // Wait for system to be ready
    require.Eventually(t, func() bool {
        return isSystemHealthy()
    }, 30*time.Second, 1*time.Second, "System failed to start")

    // Act: Nick arrives home
    err := mockHA.InjectEvent("input_boolean.nick_home", true)
    require.NoError(t, err)

    // Assert: Music should start playing
    calls := mockHA.WaitForServiceCalls(
        ServiceCallFilter{Domain: "media_player", Service: "play_media"},
        3*time.Second,
    )

    require.Len(t, calls, 1, "Expected exactly one play_media call")

    playCall := calls[0]
    assert.Equal(t, "media_player.living_room", playCall.ServiceData["entity_id"])
    assert.Contains(t, playCall.ServiceData["media_content_id"], "spotify:playlist:")

    // Assert: TTS announcement
    ttsCalls := mockHA.WaitForServiceCalls(
        ServiceCallFilter{Domain: "tts"},
        2*time.Second,
    )

    require.Len(t, ttsCalls, 1, "Expected TTS announcement")
    assert.Contains(t, ttsCalls[0].ServiceData["message"], "Nick")
}

func TestPresenceDeparture_StopsMusic(t *testing.T) {
    mockHA := NewMockHAClient("http://localhost:8123")
    defer mockHA.Reset()

    // Setup: Nick is home, music playing
    mockHA.InjectEvent("input_boolean.nick_home", true)
    mockHA.WaitForServiceCalls(
        ServiceCallFilter{Domain: "media_player", Service: "play_media"},
        3*time.Second,
    )
    mockHA.Reset() // Clear previous calls

    // Act: Nick leaves
    err := mockHA.InjectEvent("input_boolean.nick_home", false)
    require.NoError(t, err)

    // Assert: Music should stop
    calls := mockHA.WaitForServiceCalls(
        ServiceCallFilter{Domain: "media_player", Service: "media_stop"},
        3*time.Second,
    )

    require.NotEmpty(t, calls, "Expected music to stop")
}

func TestMultiplePresenceChanges_HandlesConcurrency(t *testing.T) {
    mockHA := NewMockHAClient("http://localhost:8123")
    defer mockHA.Reset()

    // Act: Rapid presence changes
    for i := 0; i < 10; i++ {
        mockHA.InjectEvent("input_boolean.nick_home", i%2 == 0)
        time.Sleep(100 * time.Millisecond)
    }

    // Assert: System should remain stable
    assert.True(t, isSystemHealthy(), "System crashed during rapid changes")

    // Final state should be consistent
    state := mockHA.GetEntityState("input_boolean.nick_home")
    assert.Equal(t, false, state) // Last change was false (i=9, odd)
}
```

### Mock HA Client Helper

```go
// tests/integration/helpers/mock_client.go

package helpers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type MockHAClient struct {
    baseURL string
    client  *http.Client
}

func NewMockHAClient(url string) *MockHAClient {
    return &MockHAClient{
        baseURL: url,
        client:  &http.Client{Timeout: 5 * time.Second},
    }
}

func (m *MockHAClient) InjectEvent(entityID string, newState interface{}) error {
    payload := map[string]interface{}{
        "entity_id": entityID,
        "new_state": newState,
    }

    data, _ := json.Marshal(payload)
    resp, err := m.client.Post(
        m.baseURL+"/test/inject_event",
        "application/json",
        bytes.NewBuffer(data),
    )

    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("inject event failed: %d", resp.StatusCode)
    }

    return nil
}

type ServiceCallFilter struct {
    Domain  string
    Service string
}

type ServiceCall struct {
    Domain      string                 `json:"domain"`
    Service     string                 `json:"service"`
    ServiceData map[string]interface{} `json:"service_data"`
    Timestamp   time.Time              `json:"timestamp"`
}

func (m *MockHAClient) WaitForServiceCalls(filter ServiceCallFilter, timeout time.Duration) []ServiceCall {
    deadline := time.Now().Add(timeout)

    for time.Now().Before(deadline) {
        calls := m.GetServiceCalls(filter)
        if len(calls) > 0 {
            return calls
        }
        time.Sleep(100 * time.Millisecond)
    }

    return []ServiceCall{}
}

func (m *MockHAClient) GetServiceCalls(filter ServiceCallFilter) []ServiceCall {
    url := fmt.Sprintf("%s/test/service_calls?domain=%s&service=%s",
        m.baseURL, filter.Domain, filter.Service)

    resp, err := m.client.Get(url)
    if err != nil {
        return nil
    }
    defer resp.Body.Close()

    var result struct {
        Calls []ServiceCall `json:"calls"`
    }

    json.NewDecoder(resp.Body).Decode(&result)
    return result.Calls
}

func (m *MockHAClient) GetEntityState(entityID string) interface{} {
    url := fmt.Sprintf("%s/test/entity/%s", m.baseURL, entityID)

    resp, err := m.client.Get(url)
    if err != nil {
        return nil
    }
    defer resp.Body.Close()

    var result struct {
        State interface{} `json:"state"`
    }

    json.NewDecoder(resp.Body).Decode(&result)
    return result.State
}

func (m *MockHAClient) Reset() error {
    resp, err := m.client.Post(m.baseURL+"/test/reset", "", nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}
```

### Test Runner Script

```bash
#!/bin/bash
# tests/integration/run_tests.sh

set -e

echo "=== Starting Integration Test Suite ==="

# Build and start test environment
echo "Starting Docker Compose test environment..."
docker-compose -f docker-compose.test.yml up -d --build

# Wait for services to be healthy
echo "Waiting for services to be ready..."
timeout 60s bash -c 'until docker-compose -f docker-compose.test.yml exec -T mockha curl -f http://localhost:8123/test/health; do sleep 2; done'
timeout 60s bash -c 'until docker-compose -f docker-compose.test.yml exec -T homeautomation wget -q --spider http://localhost:8080/health; do sleep 2; done'

echo "Services are ready. Running tests..."

# Run integration tests
docker-compose -f docker-compose.test.yml exec -T homeautomation go test -v ./tests/integration/tests/... -timeout 10m

TEST_EXIT_CODE=$?

# Collect logs
echo "Collecting logs..."
docker-compose -f docker-compose.test.yml logs homeautomation > test-logs/homeautomation.log
docker-compose -f docker-compose.test.yml logs mockha > test-logs/mockha.log

# Cleanup
echo "Shutting down test environment..."
docker-compose -f docker-compose.test.yml down -v

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "=== All tests passed! ==="
else
    echo "=== Tests failed! ==="
    exit $TEST_EXIT_CODE
fi
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/integration-tests.yml

name: Integration Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  integration-test:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Cache Docker layers
        uses: actions/cache@v3
        with:
          path: /tmp/.buildx-cache
          key: ${{ runner.os }}-buildx-${{ github.sha }}
          restore-keys: |
            ${{ runner.os }}-buildx-

      - name: Run integration tests
        run: |
          cd tests/integration
          bash run_tests.sh

      - name: Upload test logs
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: test-logs
          path: tests/integration/test-logs/

      - name: Upload coverage
        if: success()
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
          flags: integration
```

### GitLab CI Pipeline

```yaml
# .gitlab-ci.yml

stages:
  - test

integration-tests:
  stage: test
  image: docker/compose:latest
  services:
    - docker:dind
  script:
    - cd tests/integration
    - docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from homeautomation
  artifacts:
    when: always
    paths:
      - tests/integration/test-logs/
    reports:
      junit: tests/integration/test-results.xml
  coverage: '/coverage: \d+\.\d+/'
```

---

## Performance Testing

### Load Testing Strategy

**Goal:** Verify system handles high event throughput without degradation

**Approach:**
```go
func TestHighEventThroughput(t *testing.T) {
    mockHA := NewMockHAClient("http://localhost:8123")
    defer mockHA.Reset()

    // Fire 1000 events in rapid succession
    startTime := time.Now()
    for i := 0; i < 1000; i++ {
        mockHA.InjectEvent("sensor.test_sensor", i)
    }

    // System should process all events within reasonable time
    duration := time.Since(startTime)
    assert.Less(t, duration, 10*time.Second, "Event processing too slow")

    // System should remain healthy
    assert.True(t, isSystemHealthy())
}
```

### Memory Leak Detection

```bash
# Run tests with memory profiling
go test -v ./tests/integration/tests/... -memprofile=mem.prof -timeout 30m

# Analyze memory profile
go tool pprof -http=:8080 mem.prof
```

### Benchmark Tests

```go
func BenchmarkStateSynchronization(b *testing.B) {
    mockHA := NewMockHAClient("http://localhost:8123")
    defer mockHA.Reset()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        mockHA.InjectEvent("input_boolean.nick_home", i%2 == 0)
    }
}
```

---

## Appendices

### A. Complete Test Scenario List

**State Synchronization:**
- [ ] Startup sync from HA
- [ ] State change: Golang → HA
- [ ] State change: HA → Golang
- [ ] Periodic full sync
- [ ] Sync retry on failure
- [ ] Conflict resolution (HA wins)

**Plugin Integration:**
- [ ] State Tracking: Presence changes
- [ ] State Tracking: Sleep detection
- [ ] Lighting: Day phase scene activation
- [ ] Lighting: Sun event triggers
- [ ] Music: Mode selection logic
- [ ] Music: Volume management
- [ ] Music: Speaker grouping
- [ ] Sleep: Wake-up sequence
- [ ] Energy: Battery level calculation
- [ ] Load Shedding: Thermostat adjustment
- [ ] Security: Lockdown trigger
- [ ] Security: Garage automation
- [ ] TV: Playback detection
- [ ] TV: Brightness adjustment
- [ ] Calendar: Meeting notifications
- [ ] Nagging: Weather reminders

**Configuration:**
- [ ] YAML loading on startup
- [ ] Hot reload music_config.yaml
- [ ] Hot reload hue_config.yaml
- [ ] Invalid config handling
- [ ] Missing config file handling

**Connectivity:**
- [ ] WebSocket connection on startup
- [ ] Auto-reconnect after disconnect
- [ ] Message queuing during downtime
- [ ] Heartbeat/keepalive
- [ ] Connection timeout handling

**Error Handling:**
- [ ] Service call failure retry
- [ ] Invalid entity ID handling
- [ ] Malformed event handling
- [ ] Plugin crash recovery
- [ ] Graceful degradation

### B. Mock HA Service API Reference

**WebSocket Protocol:**
```
→ {"type": "auth", "access_token": "test_token"}
← {"type": "auth_ok"}

→ {"type": "subscribe_events", "event_type": "state_changed", "id": 1}
← {"type": "result", "success": true, "result": null, "id": 1}

→ {"type": "call_service", "domain": "input_boolean", "service": "turn_on", "service_data": {...}, "id": 2}
← {"type": "result", "success": true, "result": {...}, "id": 2}
```

**HTTP Test API:**
```
POST /test/inject_event
POST /test/reset
GET /test/service_calls
GET /test/entity/{entity_id}
GET /test/health
```

### C. Troubleshooting Guide

**Problem:** Tests timeout waiting for service calls

**Solutions:**
- Check homeautomation logs for errors
- Verify Mock HA received the triggering event
- Increase timeout duration
- Check plugin is loaded and initialized

**Problem:** Flaky tests (intermittent failures)

**Solutions:**
- Add retry logic with exponential backoff
- Increase wait times for async operations
- Use `WaitForServiceCalls` instead of fixed sleeps
- Check for race conditions in plugin code

**Problem:** Docker Compose services won't start

**Solutions:**
- Check port 8123 not already in use
- Verify Dockerfiles build successfully
- Check healthcheck commands are correct
- Review service dependency order

### D. Parallel Testing with Node-RED

**Setup:**
1. Run both Node-RED and Golang system
2. Both connect to real Home Assistant
3. Monitor service calls from both systems
4. Compare outputs for equivalence

**Comparison Tool:**
```go
type ServiceCallComparator struct {
    nodeRedCalls []ServiceCall
    golangCalls  []ServiceCall
}

func (c *ServiceCallComparator) Compare() ComparisonReport {
    // Normalize timestamps
    // Match calls by domain, service, and service_data
    // Report differences
}
```

---

## Summary

This containerized integration testing strategy provides:

1. **Isolated Test Environment** - Docker Compose orchestrates all components
2. **Mock Home Assistant** - Simulates HA WebSocket API without external dependencies
3. **Comprehensive Test Coverage** - All integration points tested systematically
4. **Fast Feedback Loop** - Complete suite runs in under 5 minutes
5. **CI/CD Ready** - Automated execution in GitHub Actions / GitLab CI
6. **Parallel Testing Support** - Framework for comparing with Node-RED

**Next Steps:**
1. Implement Mock HA service (Python or Go)
2. Set up Docker Compose test environment
3. Write initial test suite for core flows
4. Integrate into CI/CD pipeline
5. Expand test coverage during plugin development
