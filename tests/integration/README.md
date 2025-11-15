# Integration Tests

This directory contains containerized integration tests for the Home Automation system.

## Overview

The integration tests verify that the entire system works correctly when all components interact together. Tests run in isolated Docker containers with a mock Home Assistant service.

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- Bash shell (Linux/macOS/WSL)

### Running Tests

```bash
# From the tests/integration directory
./run_tests.sh
```

This will:
1. Build Docker images for Mock HA and the homeautomation service
2. Start the test environment
3. Wait for services to be ready
4. Run the integration test suite
5. Collect logs
6. Tear down the environment

### Running Tests Manually

```bash
# Start the test environment
docker-compose -f docker-compose.test.yml up -d

# Wait for services to be ready
# (Check logs: docker-compose -f docker-compose.test.yml logs -f)

# Run tests in the homeautomation container
docker-compose -f docker-compose.test.yml exec homeautomation \
  go test -v ./tests/integration/tests/...

# Clean up
docker-compose -f docker-compose.test.yml down -v
```

## Test Structure

```
tests/integration/
├── docker-compose.test.yml      # Test environment definition
├── Dockerfile.mockha            # Mock HA service container
├── Dockerfile.homeautomation    # System under test container
├── run_tests.sh                 # Test runner script
├── mockha/                      # Mock Home Assistant service
│   ├── main.py                  # WebSocket server implementation
│   └── requirements.txt         # Python dependencies
├── testdata/                    # Test data and fixtures
│   ├── test_fixtures.json       # Initial entity states (33 variables)
│   ├── configs/                 # Test YAML configurations
│   └── scenarios/               # Test scenario definitions
├── tests/                       # Go integration tests
│   ├── startup_test.go          # System startup tests
│   ├── state_sync_test.go       # State synchronization tests
│   └── plugin_integration_test.go # Plugin integration tests
└── helpers/                     # Test helper libraries
    └── mock_client.go           # Mock HA HTTP client
```

## Writing Tests

### Test Template

```go
package integration_test

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "home-automation/tests/integration/helpers"
)

func TestMyFeature(t *testing.T) {
    // Setup: Reset state
    require.NoError(t, mockHA.Reset())

    // Arrange: Set up initial conditions
    mockHA.InjectEvent("input_boolean.nick_home", "on")
    time.Sleep(500 * time.Millisecond)

    // Act: Trigger the behavior you want to test
    err := mockHA.InjectEvent("input_text.day_phase", "evening")
    require.NoError(t, err)

    // Assert: Verify expected outcomes
    calls, err := mockHA.WaitForServiceCalls(
        helpers.ServiceCallFilter{Domain: "light", Service: "turn_on"},
        3*time.Second,
    )
    require.NoError(t, err)
    assert.NotEmpty(t, calls, "Expected lighting service calls")
}
```

### Key Helper Functions

#### Injecting Events

```go
// Inject a state change event
mockHA.InjectEvent("input_boolean.nick_home", "on")
mockHA.InjectEvent("input_number.alarm_time", 25200000)
mockHA.InjectEvent("input_text.day_phase", "evening")
```

#### Waiting for Service Calls

```go
// Wait up to 3 seconds for matching service calls
calls, err := mockHA.WaitForServiceCalls(
    helpers.ServiceCallFilter{
        Domain: "media_player",
        Service: "play_media",
    },
    3*time.Second,
)
```

#### Getting Entity State

```go
state, err := mockHA.GetEntityState("input_boolean.nick_home")
assert.Equal(t, "on", state)
```

#### Resetting State

```go
// Clear all recorded service calls
mockHA.Reset()
```

## Mock Home Assistant Service

The Mock HA service (`mockha/main.py`) simulates Home Assistant's WebSocket API.

### WebSocket API

- Authenticates clients with token `test_token_12345`
- Maintains state for 33 input helper entities
- Accepts service calls and updates state
- Emits `state_changed` events to subscribers

### HTTP Test API

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/test/health` | GET | Health check |
| `/test/inject_event` | POST | Inject state change event |
| `/test/service_calls` | GET | Get recorded service calls |
| `/test/entity/{id}` | GET | Get entity state |
| `/test/reset` | POST | Reset recorded calls |

### Example: Injecting an Event

```bash
curl -X POST http://localhost:8123/test/inject_event \
  -H "Content-Type: application/json" \
  -d '{
    "entity_id": "input_boolean.nick_home",
    "new_state": "on"
  }'
```

### Example: Getting Service Calls

```bash
curl "http://localhost:8123/test/service_calls?domain=media_player&service=play_media"
```

## Test Data

### Fixtures (test_fixtures.json)

Contains initial state for all 33 entities:
- 18 input_boolean entities
- 3 input_number entities
- 12 input_text entities (including 6 JSON configs)

### Test Configs

Simplified YAML configuration files for testing:
- `music_config.yaml` - Music modes and playlists
- `hue_config.yaml` - Lighting scenes
- `schedule_config.yaml` - Time-based schedules
- `energy_config.yaml` - Energy thresholds

## CI/CD Integration

Integration tests run automatically in GitHub Actions on:
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop`
- Manual workflow dispatch

See `.github/workflows/integration-tests.yml` for configuration.

## Troubleshooting

### Tests timeout waiting for services

**Solution:** Check logs for startup errors:
```bash
docker-compose -f docker-compose.test.yml logs mockha
docker-compose -f docker-compose.test.yml logs homeautomation
```

### Mock HA won't start

**Solution:** Verify port 8123 is not in use:
```bash
lsof -i :8123
docker-compose -f docker-compose.test.yml down -v
```

### Tests fail intermittently

**Solution:** Increase wait times in tests, check for race conditions:
```go
// Instead of fixed sleep
time.Sleep(1 * time.Second)

// Use WaitForServiceCalls with timeout
mockHA.WaitForServiceCalls(filter, 5*time.Second)
```

### Service calls not recorded

**Solution:** Ensure the homeautomation system has fully initialized:
```go
// Add longer wait after injecting events
time.Sleep(2 * time.Second)
```

## Development Workflow

1. **Add a new test:**
   - Create test file in `tests/` directory
   - Use `mockHA` helper functions
   - Follow AAA pattern (Arrange, Act, Assert)

2. **Run tests locally:**
   ```bash
   ./run_tests.sh
   ```

3. **Debug failing tests:**
   ```bash
   # Keep environment running
   docker-compose -f docker-compose.test.yml up -d

   # Check logs
   docker-compose -f docker-compose.test.yml logs -f homeautomation

   # Run specific test
   docker-compose -f docker-compose.test.yml exec homeautomation \
     go test -v ./tests/integration/tests/ -run TestMySpecificTest
   ```

4. **Update fixtures or configs:**
   - Edit files in `testdata/`
   - Rebuild containers: `docker-compose -f docker-compose.test.yml build`

## Performance

The complete test suite should run in under 5 minutes:
- Environment startup: ~30 seconds
- Test execution: ~3 minutes
- Teardown: ~10 seconds

## Future Enhancements

- [ ] Add parallel test execution
- [ ] Generate HTML test reports
- [ ] Add performance benchmarks
- [ ] Implement test coverage tracking
- [ ] Add chaos testing (network failures, etc.)
- [ ] Create visual test reports with diagrams

## Resources

- [Integration Testing Strategy](../../INTEGRATION_TESTING_STRATEGY.md)
- [Golang Design Document](../../GOLANG_DESIGN.md)
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket)
