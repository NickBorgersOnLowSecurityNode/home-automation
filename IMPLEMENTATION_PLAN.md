# Home Automation Golang Implementation - MVP Phase

## Current Status

**Session Date:** 2025-11-15
**Phase:** Minimal MVP - HA Client + State Manager
**Location:** `/Users/nborgers/code/node-red/homeautomation-go/`

### What's Been Done
- ✅ Design document reviewed (GOLANG_DESIGN.md)
- ✅ Migration mapping analyzed (33 state variables identified)
- ✅ Project location decided: `homeautomation-go/` subdirectory
- ✅ Basic directory structure created: `cmd/`, `internal/ha/`, `internal/state/`
- ⏸️ Paused for devcontainer setup (Go not installed)

### User Decisions Made
- **Location:** New subdirectory `/Users/nborgers/code/node-red/homeautomation-go/`
- **Phase:** Minimal MVP (HA Client + State Manager only)
- **HA Instance:** Available with access token (see `/Users/nborgers/code/node-red/token`)
- **Testing:** Full test coverage from the start

---

## Project Structure

```
homeautomation-go/
├── cmd/
│   └── main.go                 # Entry point - demo application
├── internal/
│   ├── ha/                     # Home Assistant WebSocket client
│   │   ├── client.go           # Main client implementation
│   │   ├── client_test.go      # Unit tests
│   │   ├── types.go            # HA message types & structs
│   │   └── mock.go             # Mock client for testing
│   └── state/                  # State Manager
│       ├── manager.go          # State manager implementation
│       ├── manager_test.go     # Unit tests
│       └── variables.go        # 33 state variable definitions
├── go.mod
├── go.sum
├── .env.example                # Template for HA credentials
├── .gitignore                  # Ignore .env file
└── README.md                   # Quick start guide
```

---

## Implementation Checklist

### Phase 1: Project Setup
- [ ] **1.1** Initialize Go module
  ```bash
  cd homeautomation-go
  go mod init homeautomation
  ```

- [ ] **1.2** Add dependencies
  ```bash
  go get github.com/gorilla/websocket@v1.5.0
  go get github.com/joho/godotenv@v1.5.1
  go get go.uber.org/zap@v1.26.0
  go get github.com/stretchr/testify@v1.8.4
  ```

- [ ] **1.3** Create `.env.example`
  ```env
  HA_URL=ws://homeassistant.local:8123/api/websocket
  HA_TOKEN=your_token_here
  ```

- [ ] **1.4** Create `.gitignore`
  ```
  .env
  *.log
  ```

---

### Phase 2: Home Assistant WebSocket Client

**File:** `internal/ha/types.go`

- [ ] **2.1** Define HA message types:
  - `Message` - Base message structure (type, id, success, result, error)
  - `AuthMessage` - Authentication request/response
  - `StateChangedEvent` - State change event payload
  - `CallServiceRequest` - Service call structure
  - `GetStateRequest` - Get state structure
  - `SubscribeEventsRequest` - Event subscription
  - Entity state representation

**File:** `internal/ha/client.go`

- [ ] **2.2** Implement `HAClient` struct with:
  - WebSocket connection
  - Message ID counter (for request/response correlation)
  - Response channels map (id -> chan Message)
  - Event subscriber callbacks
  - Mutex for thread safety

- [ ] **2.3** Implement connection methods:
  - `Connect()` - WebSocket connection + authentication flow
  - `Disconnect()` - Graceful shutdown
  - `IsConnected()` - Connection status
  - Auto-reconnect with exponential backoff

- [ ] **2.4** Implement message handling:
  - `sendMessage()` - Send with unique ID
  - `receiveMessages()` - Background goroutine for incoming
  - Response routing to waiting channels
  - Event routing to subscribers

- [ ] **2.5** Implement state operations:
  - `GetState(entityID string) (*State, error)`
  - `SetState(entityID string, value interface{}) error`

- [ ] **2.6** Implement service calls:
  - `CallService(domain, service string, data map[string]interface{}) error`
  - Convenience methods:
    - `SetInputBoolean(name string, value bool) error`
    - `SetInputNumber(name string, value float64) error`
    - `SetInputText(name string, value string) error`

- [ ] **2.7** Implement event subscription:
  - `SubscribeStateChanges(entityID string, handler StateChangeHandler) error`
  - Pattern matching for wildcards (e.g., "input_boolean.*")
  - Unsubscribe mechanism

**File:** `internal/ha/client_test.go`

- [ ] **2.8** Write comprehensive tests:
  - Mock WebSocket server using `httptest`
  - Test authentication flow
  - Test message ID correlation
  - Test state get/set operations
  - Test service calls
  - Test event subscription and delivery
  - Test reconnection logic
  - Test concurrent operations (thread safety)

**File:** `internal/ha/mock.go`

- [ ] **2.9** Create mock client for testing other components:
  - Implement HAClient interface
  - In-memory state storage
  - Configurable responses
  - Event simulation

---

### Phase 3: State Manager

**File:** `internal/state/variables.go`

- [ ] **3.1** Define state variable metadata:
  ```go
  type StateVariable struct {
      Key        string      // Go variable name (e.g., "isNickHome")
      EntityID   string      // HA entity ID (e.g., "input_boolean.nick_home")
      Type       StateType   // bool, string, number, json
      Default    interface{} // Default value
      ReadOnly   bool        // Whether it's read-only from HA
  }
  ```

- [ ] **3.2** Define all 33 state variables (from migration_mapping.md):

  **Booleans (18):**
  - isNickHome → input_boolean.nick_home
  - isCarolineHome → input_boolean.caroline_home
  - isToriHere → input_boolean.tori_here
  - isAnyOwnerHome → input_boolean.any_owner_home
  - isAnyoneHome → input_boolean.anyone_home
  - isMasterAsleep → input_boolean.master_asleep
  - isGuestAsleep → input_boolean.guest_asleep
  - isAnyoneAsleep → input_boolean.anyone_asleep
  - isEveryoneAsleep → input_boolean.everyone_asleep
  - isGuestBedroomDoorOpen → input_boolean.guest_bedroom_door_open
  - isHaveGuests → input_boolean.have_guests
  - isAppleTVPlaying → input_boolean.apple_tv_playing
  - isTVPlaying → input_boolean.tv_playing
  - isTVon → input_boolean.tv_on
  - isFadeOutInProgress → input_boolean.fade_out_in_progress
  - isFreeEnergyAvailable → input_boolean.free_energy_available
  - isGridAvailable → input_boolean.grid_available
  - isExpectingSomeone → input_boolean.expecting_someone

  **Numbers (3):**
  - alarmTime → input_number.alarm_time
  - remainingSolarGeneration → input_number.remaining_solar_generation
  - thisHourSolarGeneration → input_number.this_hour_solar_generation

  **Text (6):**
  - dayPhase → input_text.day_phase
  - sunevent → input_text.sunevent
  - musicPlaybackType → input_text.music_playback_type
  - batteryEnergyLevel → input_text.battery_energy_level
  - currentEnergyLevel → input_text.current_energy_level
  - solarProductionEnergyLevel → input_text.solar_production_energy_level

  **JSON (1):**
  - currentlyPlayingMusic → input_text.currently_playing_music

**File:** `internal/state/manager.go`

- [ ] **3.3** Implement `StateManager` struct:
  - In-memory cache: `map[string]interface{}`
  - Thread safety: `sync.RWMutex`
  - Reference to HAClient
  - Variables registry from variables.go

- [ ] **3.4** Implement type-safe getters:
  - `GetBool(key string) (bool, error)`
  - `GetString(key string) (string, error)`
  - `GetNumber(key string) (float64, error)`
  - `GetJSON(key string, target interface{}) error`
  - Include validation and type checking

- [ ] **3.5** Implement type-safe setters:
  - `SetBool(key string, value bool) error`
  - `SetString(key string, value string) error`
  - `SetNumber(key string, value float64) error`
  - `SetJSON(key string, value interface{}) error`
  - Update cache + sync to HA

- [ ] **3.6** Implement synchronization:
  - `SyncFromHA() error` - Read all 33 variables from HA on startup
  - `syncToHA(key string, value interface{}) error` - Write single variable
  - Handle HA entity type conversion (bool → turn_on/turn_off)
  - Error handling and retry logic

- [ ] **3.7** Implement state change subscriptions:
  - Subscribe to all 33 input_* entities via HAClient
  - Update cache when HA changes detected
  - Callback mechanism for consumers
  - `Subscribe(key string, handler StateChangeHandler) Subscription`

- [ ] **3.8** Implement atomic operations:
  - `CompareAndSwapBool(key string, old, new bool) (bool, error)`

**File:** `internal/state/manager_test.go`

- [ ] **3.9** Write comprehensive tests:
  - Test initialization with mock HA client
  - Test SyncFromHA with all variable types
  - Test all getter methods (including type validation)
  - Test all setter methods (cache update + HA sync)
  - Test concurrent access (race conditions)
  - Test state change subscriptions
  - Test error handling (missing variables, type mismatches)
  - Test atomic operations

---

### Phase 4: Demo Application

**File:** `cmd/main.go`

- [ ] **4.1** Implement main application:
  - Load environment variables (HA_URL, HA_TOKEN)
  - Initialize logger (zap)
  - Create HA client
  - Connect to Home Assistant
  - Create State Manager
  - Sync all state from HA

- [ ] **4.2** Display current state:
  - Print all 33 variables organized by type
  - Format nicely with colors/sections

- [ ] **4.3** Subscribe to state changes:
  - Log all state changes as they occur
  - Show old value → new value

- [ ] **4.4** Demonstrate setting values:
  - Toggle a boolean (e.g., isExpectingSomeone)
  - Update a text value (e.g., dayPhase)
  - Show changes reflected in HA

- [ ] **4.5** Keep running and monitoring:
  - Graceful shutdown on SIGINT/SIGTERM
  - Connection health monitoring
  - Reconnection handling

**File:** `README.md`

- [ ] **4.6** Create quick start guide:
  - Prerequisites (Go 1.21+)
  - Setup instructions
  - Environment configuration
  - Running the demo
  - Expected output
  - Testing commands

---

### Phase 5: Testing & Validation

- [ ] **5.1** Run all unit tests:
  ```bash
  go test ./... -v -cover
  ```

- [ ] **5.2** Verify test coverage:
  ```bash
  go test ./... -coverprofile=coverage.out
  go tool cover -html=coverage.out
  ```
  - Target: >80% coverage

- [ ] **5.3** End-to-end testing with real HA:
  - Connect to your HA instance
  - Verify all 33 variables sync correctly
  - Change a value in HA → verify Golang sees it
  - Change a value in Golang → verify HA reflects it
  - Test reconnection (stop/start HA WebSocket)

- [ ] **5.4** Performance testing:
  - Test concurrent state reads/writes
  - Measure latency of state changes
  - Verify no memory leaks (long-running test)

---

## Code Templates & Examples

### HA Client Interface
```go
type HAClient interface {
    Connect() error
    Disconnect() error
    IsConnected() bool
    GetState(entityID string) (*State, error)
    SetState(entityID string, state interface{}) error
    CallService(domain, service string, data map[string]interface{}) error
    SubscribeStateChanges(entityID string, handler StateChangeHandler) (Subscription, error)
    SetInputBoolean(name string, value bool) error
    SetInputNumber(name string, value float64) error
    SetInputText(name string, value string) error
}
```

### State Manager Interface
```go
type StateManager interface {
    GetBool(key string) (bool, error)
    SetBool(key string, value bool) error
    GetString(key string) (string, error)
    SetString(key string, value string) error
    GetNumber(key string) (float64, error)
    SetNumber(key string, value float64) error
    GetJSON(key string, target interface{}) error
    SetJSON(key string, value interface{}) error
    CompareAndSwapBool(key string, old, new bool) (bool, error)
    Subscribe(key string, handler StateChangeHandler) Subscription
    SyncFromHA() error
}
```

### HA WebSocket Authentication Flow
```
1. Connect WebSocket to ws://homeassistant.local:8123/api/websocket
2. Receive: {"type": "auth_required"}
3. Send: {"type": "auth", "access_token": "YOUR_TOKEN"}
4. Receive: {"type": "auth_ok", "ha_version": "..."}
5. Now authenticated - can send commands
```

### HA Message Examples
```json
// Get state
{"id": 1, "type": "get_states"}

// Subscribe to events
{"id": 2, "type": "subscribe_events", "event_type": "state_changed"}

// Call service
{
  "id": 3,
  "type": "call_service",
  "domain": "input_boolean",
  "service": "turn_on",
  "service_data": {"entity_id": "input_boolean.nick_home"}
}

// Set input_number
{
  "id": 4,
  "type": "call_service",
  "domain": "input_number",
  "service": "set_value",
  "service_data": {
    "entity_id": "input_number.alarm_time",
    "value": 1668524400000
  }
}

// Set input_text
{
  "id": 5,
  "type": "call_service",
  "domain": "input_text",
  "service": "set_value",
  "service_data": {
    "entity_id": "input_text.day_phase",
    "value": "morning"
  }
}
```

---

## Next Steps After MVP

Once the MVP is complete and tested:

1. **Event Bus** - Add internal pub/sub system
2. **Config Loader** - YAML config file support
3. **Plugin Framework** - Plugin manager and interface
4. **First Plugin** - Implement State Tracking plugin
5. **Gradual Migration** - Add remaining plugins one by one

---

## Reference Files

- **Design:** `/Users/nborgers/code/node-red/GOLANG_DESIGN.md`
- **Variable Mapping:** `/Users/nborgers/code/node-red/migration_mapping.md`
- **HA Token:** `/Users/nborgers/code/node-red/token`
- **Config Files:** `/Users/nborgers/code/node-red/configs/*.yaml`

---

## Success Criteria for MVP

✅ **MVP is complete when:**
1. All 33 state variables sync from HA to Golang on startup
2. State changes in HA are reflected in Golang cache within 1 second
3. State changes in Golang are written to HA successfully
4. WebSocket reconnection works automatically
5. All unit tests pass with >80% coverage
6. Demo application runs without errors for 5+ minutes
7. Thread-safe concurrent access verified

---

## Development Notes

**HA WebSocket Documentation:**
- https://developers.home-assistant.io/docs/api/websocket

**Testing Strategy:**
- Use `httptest.NewServer()` for WebSocket testing
- Use mock HA client for State Manager testing
- Test both happy path and error conditions
- Test concurrent access patterns

**Common Gotchas:**
- HA booleans use turn_on/turn_off services, not set_value
- Message IDs must be unique and sequential
- State change events have nested structure: event.data.new_state
- JSON values in input_text need to be stringified
- WebSocket connection needs keep-alive/ping

---

**Resume Point:** Start with Phase 1.1 in your devcontainer with Go installed.
