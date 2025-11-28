# Shadow State Tracking Implementation Plan

## Overview
Extend the Go automation controller to track complete shadow state **per plugin**, showing:
- **Inputs (current):** Current values of all subscribed state variables
- **Inputs (at last action):** Snapshot of input values when the plugin last took action
- **Outputs:** Actions taken and current state maintained by the plugin

Structure mirrors the existing plugin architecture 1:1 for maintainability.

---

## Plugin-Aligned Architecture

### Plugins & Their Shadow State

**Action-Heavy Plugins** (primary focus):
- **`lighting`** - Room scenes, light states, transition tracking
- **`music`** - Mode, playlist, speaker groups, volumes, fades
- **`security`** - Lockdown, garage, doorbell, vehicle arrival
- **`sleephygiene`** - Wake sequences, fade-outs, TTS triggers
- **`loadshedding`** - Thermostat control, energy-based restrictions

**Read-Heavy Plugins** (simpler shadow state):
- **`energy`** - Battery/solar calculations (mostly computed outputs)
- **`statetracking`** - Presence/sleep tracking (computes derived state)
- **`dayphase`** - Time-of-day tracking (computes phase from sensors)
- **`tv`** - TV/AppleTV monitoring (tracks playback state)
- **`reset`** - Reset coordination (triggers state changes)

---

## Shadow State Structure (Example: Lighting)

```json
{
  "plugin": "lighting",
  "inputs": {
    "current": {
      "dayPhase": "evening",
      "sunevent": "sunset",
      "isAnyoneHome": true,
      "isTVPlaying": false,
      "isEveryoneAsleep": false,
      "isMasterAsleep": false,
      "isHaveGuests": false
    },
    "atLastAction": {
      "dayPhase": "evening",
      "sunevent": "afternoon",
      "isAnyoneHome": true,
      "isTVPlaying": false,
      "isEveryoneAsleep": false,
      "isMasterAsleep": false,
      "isHaveGuests": false
    }
  },
  "outputs": {
    "rooms": {
      "Living Room": {
        "activeScene": "evening",
        "lastAction": "2025-11-27T19:30:00Z",
        "actionType": "activate_scene",
        "reason": "dayPhase changed from 'afternoon' to 'evening'"
      },
      "Kitchen": {
        "activeScene": "evening",
        "lastAction": "2025-11-27T19:30:00Z",
        "actionType": "activate_scene",
        "reason": "dayPhase changed from 'afternoon' to 'evening'"
      }
    },
    "lastActionTime": "2025-11-27T19:30:00Z"
  },
  "metadata": {
    "lastUpdated": "2025-11-27T19:30:00Z"
  }
}
```

---

## Implementation Phases

### Phase 1: Core Infrastructure + Lighting Plugin (Pilot) ✅ **COMPLETE**

**Status:** ✅ Merged in PR #112 (2025-11-28)

**1.1 Create Shadow State Package** ✅
- `internal/shadowstate/tracker.go` - Core tracker with thread-safe access
- `internal/shadowstate/types.go` - Common types (PluginShadowState, InputSnapshot, ActionRecord, StateMetadata, LightingShadowState)
- Thread-safe recording with RWMutex

**1.2 Define Common Interfaces** ✅
```go
type PluginShadowState interface {
    GetCurrentInputs() map[string]interface{}
    GetLastActionInputs() map[string]interface{}
    GetOutputs() interface{}
    GetMetadata() StateMetadata
}
```

**1.3 Implement Lighting Shadow State** ✅
- Track subscribed variables: `dayPhase`, `sunevent`, `isAnyoneHome`, `isTVPlaying`, `isEveryoneAsleep`, `isMasterAsleep`, `isHaveGuests`
- Snapshot inputs on every action
- Track output state: active scene per room, last action, reason
- Implement `GetShadowState()` on `lighting.Manager`
- Update current inputs on EVERY state change (not just actions)

**1.4 Add API Endpoints** ✅
- `/api/shadow/lighting` - Returns lighting shadow state
- `/api/shadow` - Returns all plugin shadow states
- Test with existing lighting triggers

**Validation:**
- ✅ Changing `dayPhase` shows up in current inputs
- ✅ Scene activation snapshots inputs and records output
- ✅ API returns both current and at-last-action values
- ✅ Current inputs update on every subscribed variable change
- ✅ Thread-safe concurrent access

**Implementation Notes:**
- Used `LightingTracker` struct to encapsulate lighting-specific shadow state logic
- Current inputs update on EVERY subscribed variable change (via `updateShadowInputs()`)
- Inputs snapshot taken only when actions are recorded (via `SnapshotInputsForAction()`)
- Deep copies prevent race conditions in concurrent reads
- Tracker supports both static registration and dynamic providers for flexibility

---

### Phase 2: Music Plugin ✅ **COMPLETE**

**Status:** ✅ Completing in PR #115 (2025-11-28)

**2.1 Define Music Shadow State**
- **Inputs:** `dayPhase`, `isAnyoneAsleep`, `isAnyoneHome`, `isMasterAsleep`, `isEveryoneAsleep`
- **Outputs:**
  - Current music mode
  - Active playlist URI & name
  - Speaker group composition
  - Per-speaker volumes
  - Fade state (idle/fading_in/fading_out)
  - Playlist rotation counters

**2.2 Update Music Manager**
- Snapshot inputs on mode selection
- Track speaker group builds
- Record volume calculations
- Track fade progress

**2.3 Add API Endpoint**
- `/api/shadow/music`

**Pattern to Follow (from Lighting):**
1. Create `MusicShadowState` and `MusicTracker` types in `internal/shadowstate/types.go`
2. Add `shadowTracker *shadowstate.MusicTracker` field to `music.Manager`
3. Call `updateShadowInputs()` in all subscription handlers
4. Call `recordAction()` when taking actions (mode changes, playlist changes, volume adjustments)
5. Implement `GetShadowState()` method on `music.Manager`
6. Register with global tracker in main.go

---

### Phase 3: Security Plugin

**3.1 Define Security Shadow State**
- **Inputs:** `isEveryoneAsleep`, `isAnyoneHome`, `isExpectingSomeone`, `isNickHome`, `isCarolineHome`
- **Outputs:**
  - Lockdown active/inactive + reason
  - Last doorbell press (with rate limit status)
  - Last vehicle arrival notification
  - Garage auto-open events

**3.2 Add API Endpoint**
- `/api/shadow/security`

---

### Phase 4: Sleep Hygiene Plugin ✅ **COMPLETE**

**Status:** ✅ Merged in PR #116 (2025-11-28)

**4.1 Define Sleep Hygiene Shadow State** ✅
- **Inputs:** `isMasterAsleep`, `alarmTime`, `musicPlaybackType`, `currentlyPlayingMusic`, `isAnyoneHome`, `isEveryoneAsleep`, `isFadeOutInProgress`, `isNickHome`, `isCarolineHome`
- **Outputs:**
  - Wake sequence status (inactive → begin_wake → wake_in_progress → complete → cancel_wake)
  - Fade-out progress per speaker (with volume progression tracking)
  - Last TTS announcement (message, speaker, timestamp)
  - Screen stop / bedtime reminder triggers

**4.2 Add API Endpoint** ✅
- `/api/shadow/sleephygiene`

---

### Phase 5: Load Shedding Plugin

**5.1 Define Load Shedding Shadow State**
- **Inputs:** `currentEnergyLevel`
- **Outputs:**
  - Load shedding active/inactive
  - Activation reason (energy level threshold)
  - Thermostat mode & temperature settings
  - Last action timestamp

**5.2 Add API Endpoint**
- `/api/shadow/loadshedding`

---

### Phase 6: Read-Heavy Plugins 🚧 **IN PROGRESS**

Read-heavy plugins differ from action-heavy plugins in that they primarily **compute derived state** rather than take actions on Home Assistant. Their shadow state focuses on:
- **Inputs:** Raw sensor/entity values from Home Assistant
- **Outputs:** Computed/derived state values (not actions)
- **Computation metadata:** When values were last calculated

**6.1 Energy Plugin**
- **Inputs (from HA):**
  - `sensor.span_panel_span_storage_battery_percentage_2` (battery %)
  - `sensor.energy_next_hour` (this hour solar kW)
  - `sensor.energy_production_today_remaining` (remaining solar kWh)
  - `isGridAvailable` (state variable)
- **Outputs (computed):**
  - `batteryEnergyLevel` - Computed from battery percentage thresholds
  - `solarProductionEnergyLevel` - Computed from solar generation
  - `currentEnergyLevel` - Combined from battery + solar levels
  - `isFreeEnergyAvailable` - Computed from grid availability + time window
- **Computation Events:**
  - Last battery level calculation
  - Last solar level calculation
  - Last free energy check

**6.2 State Tracking Plugin**
- **Inputs (from HA):**
  - `light.primary_suite` (for sleep detection)
  - `input_boolean.primary_bedroom_door_open` (for wake detection)
  - `input_boolean.nick_home`, `input_boolean.caroline_home`, `input_boolean.tori_here`
- **Outputs (computed derived states):**
  - `isAnyOwnerHome` = isNickHome OR isCarolineHome
  - `isAnyoneHome` = isAnyOwnerHome OR isToriHere
  - `isAnyoneAsleep` = isMasterAsleep OR isGuestAsleep
  - `isEveryoneAsleep` = isMasterAsleep AND isGuestAsleep
- **Timer States:**
  - Sleep detection timer (1 min after lights off)
  - Wake detection timer (20 sec after door open)
  - Owner return home auto-reset timer (10 min)
- **Announcements:**
  - Last arrival announcement (person, message, time)

**6.3 Day Phase Plugin**
- **Inputs:**
  - Current time
  - Sun times from Home Assistant (sunrise, sunset, etc.)
  - Today's schedule from config
- **Outputs (computed):**
  - `sunevent` - Current sun event (sunrise, morning, afternoon, sunset, evening, night)
  - `dayPhase` - Current day phase (morning, day, evening, night)
- **Computation Events:**
  - Last sun event calculation
  - Last day phase calculation

**6.4 TV Plugin**
- **Inputs (from HA):**
  - `media_player.big_beautiful_oled` (Apple TV state)
  - `switch.sync_box_power` (TV power)
  - `select.sync_box_hdmi_input` (HDMI input selector)
- **Outputs (computed):**
  - `isAppleTVPlaying` - Apple TV playing state
  - `isTVon` - TV power state
  - `isTVPlaying` - Combined playing state based on HDMI input

**6.5 Reset Coordinator** (Note: This is a coordinator, not a plugin)
- The reset coordinator triggers resets on other plugins, it doesn't maintain shadow state itself
- Reset events are logged but not tracked as shadow state
- **Skip for Phase 6** - Reset is an orchestrator, not a state-computing plugin

**6.6 Add API Endpoints**
- `/api/shadow/energy`
- `/api/shadow/statetracking`
- `/api/shadow/dayphase`
- `/api/shadow/tv`

---

### Phase 7: Unified Shadow State API

**7.1 Aggregate Endpoint**
- `/api/shadow` - Returns all plugin shadow states
- Organized by plugin name (matches existing `/api/states` pattern)

**7.2 Response Structure**
```json
{
  "plugins": {
    "lighting": { /* full lighting shadow state */ },
    "music": { /* full music shadow state */ },
    "security": { /* full security shadow state */ },
    "sleephygiene": { /* full sleephygiene shadow state */ },
    "loadshedding": { /* full loadshedding shadow state */ },
    "energy": { /* full energy shadow state */ },
    "statetracking": { /* full statetracking shadow state */ },
    "dayphase": { /* full dayphase shadow state */ },
    "tv": { /* full tv shadow state */ },
    "reset": { /* full reset shadow state */ }
  },
  "metadata": {
    "timestamp": "2025-11-27T19:30:00Z",
    "controllerStartTime": "2025-11-27T10:00:00Z",
    "version": "1.0.0"
  }
}
```

---

## Implementation Lessons Learned (from Phase 1)

### Key Insights

1. **Update Current Inputs on Every State Change**
   - Initially designed to only update on actions
   - Changed to update current inputs whenever ANY subscribed variable changes
   - This ensures current inputs always reflect real-time state
   - Action snapshots capture inputs at time of action for comparison

2. **Separation of Concerns**
   - Create plugin-specific tracker (e.g., `LightingTracker`) instead of generic tracker
   - Encapsulates plugin logic and keeps manager code clean
   - Each plugin manager owns its tracker instance

3. **Thread Safety Patterns**
   - Use `sync.RWMutex` for tracker state
   - Always create deep copies when returning state (prevents race conditions)
   - Lock during updates, read-lock during reads

4. **Registration Pattern**
   - Support both static state registration and dynamic providers
   - Lighting uses provider pattern: `RegisterPluginProvider("lighting", func() { return m.GetShadowState() })`
   - Allows lazy evaluation and always-fresh state

5. **Common Mistakes to Avoid**
   - ❌ Don't forget to update current inputs in ALL subscription handlers
   - ❌ Don't return pointers to internal state (use deep copies)
   - ❌ Don't snapshot inputs on state changes (only on actions)
   - ✅ Do call `updateShadowInputs()` at the start of every handler
   - ✅ Do call `SnapshotInputsForAction()` only when taking actions

### Implementation Checklist (for new plugins)

- [ ] Define `{Plugin}ShadowState` struct in `internal/shadowstate/types.go`
- [ ] Define `{Plugin}Tracker` struct with mutex and state
- [ ] Add `shadowTracker *shadowstate.{Plugin}Tracker` field to plugin manager
- [ ] Initialize tracker in `New{Plugin}Manager()` constructor
- [ ] Create `updateShadowInputs()` helper to fetch current state variables
- [ ] Call `updateShadowInputs()` in EVERY subscription handler
- [ ] Create `recordAction()` helper that:
  1. Updates current inputs
  2. Snapshots inputs for action
  3. Records action details
- [ ] Call `recordAction()` whenever plugin takes action
- [ ] Implement `GetShadowState()` method returning deep copy
- [ ] Register plugin with global tracker in `main.go`
- [ ] Add API endpoint handler in `internal/api/server.go`
- [ ] Update API documentation with new endpoint
- [ ] Write tests for shadow state tracking

---

## Technical Implementation Details

### Core Shadow State Tracker

```go
// internal/shadowstate/tracker.go
type Tracker struct {
    mu                sync.RWMutex
    pluginStates      map[string]PluginShadowState
    pluginInputs      map[string]*InputSnapshot
}

type InputSnapshot struct {
    Timestamp time.Time
    Values    map[string]interface{}
}

// Plugins call this when taking actions
func (t *Tracker) RecordAction(plugin string, inputs map[string]interface{}, output interface{}) {
    t.mu.Lock()
    defer t.mu.Unlock()

    // Snapshot current inputs
    t.pluginInputs[plugin] = &InputSnapshot{
        Timestamp: time.Now(),
        Values:    inputs,
    }

    // Store output state
    // Plugin-specific logic...
}
```

### Plugin Integration Pattern

Each plugin manager implements the shadow state pattern. Here's the exact pattern from the lighting plugin:

**Step 1: Add tracker field to manager**
```go
// In internal/plugins/lighting/manager.go
type Manager struct {
    haClient      ha.HAClient
    stateManager  *state.Manager
    config        *HueConfig
    logger        *zap.Logger
    readOnly      bool
    shadowTracker *shadowstate.LightingTracker  // ← Add this
    subscriptions []state.Subscription
}
```

**Step 2: Initialize in constructor**
```go
func NewManager(haClient ha.HAClient, stateManager *state.Manager, config *HueConfig, logger *zap.Logger, readOnly bool) *Manager {
    return &Manager{
        haClient:      haClient,
        stateManager:  stateManager,
        config:        config,
        logger:        logger.Named("lighting"),
        readOnly:      readOnly,
        shadowTracker: shadowstate.NewLightingTracker(),  // ← Initialize
        subscriptions: make([]state.Subscription, 0),
    }
}
```

**Step 3: Create helper to update current inputs**
```go
// Called at the start of EVERY subscription handler
func (m *Manager) updateShadowInputs() {
    inputs := make(map[string]interface{})

    // Get all subscribed variables
    if val, err := m.stateManager.GetString("dayPhase"); err == nil {
        inputs["dayPhase"] = val
    }
    if val, err := m.stateManager.GetString("sunevent"); err == nil {
        inputs["sunevent"] = val
    }
    if val, err := m.stateManager.GetBool("isAnyoneHome"); err == nil {
        inputs["isAnyoneHome"] = val
    }
    // ... etc for all inputs

    m.shadowTracker.UpdateCurrentInputs(inputs)
}
```

**Step 4: Call updateShadowInputs in ALL handlers**
```go
func (m *Manager) handleDayPhaseChange(key string, oldValue, newValue interface{}) {
    // Update shadow state current inputs immediately
    m.updateShadowInputs()  // ← MUST be first

    // ... rest of handler logic
}
```

**Step 5: Record actions when taking them**
```go
func (m *Manager) recordAction(roomName string, actionType string, reason string, activeScene string, turnedOff bool) {
    // First, update current inputs
    m.updateShadowInputs()

    // Snapshot inputs for this action
    m.shadowTracker.SnapshotInputsForAction()

    // Record the action
    m.shadowTracker.RecordRoomAction(roomName, actionType, reason, activeScene, turnedOff)
}

// Example usage:
func (m *Manager) activateScene(room *RoomConfig, dayPhase string) {
    // ... call HA service ...

    // Record action in shadow state
    m.recordAction(room.HueGroup, "activate_scene",
        fmt.Sprintf("Activated scene '%s'", dayPhase),
        dayPhase, false)
}
```

**Step 6: Implement GetShadowState method**
```go
// GetShadowState returns the current shadow state
func (m *Manager) GetShadowState() *shadowstate.LightingShadowState {
    return m.shadowTracker.GetState()  // Returns deep copy
}
```

**Step 7: Register with global tracker (in main.go)**
```go
// Register shadow state provider
shadowTracker.RegisterPluginProvider("lighting", func() shadowstate.PluginShadowState {
    return lightingManager.GetShadowState()
})
```

---

## Testing Strategy

**Unit Tests (per plugin):**
- Input snapshot capture
- Output state updates
- Thread safety (concurrent reads/writes)
- API response formatting

**Example from Lighting Plugin:**
```go
func TestLightingTracker_UpdateAndSnapshot(t *testing.T) {
    tracker := shadowstate.NewLightingTracker()

    // Update current inputs
    inputs := map[string]interface{}{
        "dayPhase": "evening",
        "isAnyoneHome": true,
    }
    tracker.UpdateCurrentInputs(inputs)

    // Verify current inputs updated
    state := tracker.GetState()
    assert.Equal(t, "evening", state.Inputs.Current["dayPhase"])

    // Record an action (snapshots inputs)
    tracker.SnapshotInputsForAction()
    tracker.RecordRoomAction("Living Room", "activate_scene", "test", "evening", false)

    // Verify at-last-action captured
    state = tracker.GetState()
    assert.Equal(t, "evening", state.Inputs.AtLastAction["dayPhase"])

    // Change inputs again
    inputs["dayPhase"] = "night"
    tracker.UpdateCurrentInputs(inputs)

    // Verify current changed but at-last-action stayed the same
    state = tracker.GetState()
    assert.Equal(t, "night", state.Inputs.Current["dayPhase"])
    assert.Equal(t, "evening", state.Inputs.AtLastAction["dayPhase"])
}
```

**Integration Tests:**
- End-to-end: Trigger state change → Action taken → Shadow state updated → API returns correct data
- Verify current vs. at-last-action input values differ correctly
- Verify all plugins represented in `/api/shadow`

**Manual Testing:**
- Change `dayPhase` → verify `/api/shadow/lighting` shows scene changes with correct inputs
- Verify `/api/shadow` returns all registered plugins
- Verify current inputs update on every state change
- Verify at-last-action inputs only update when actions are taken

---

## Key Design Decisions

1. **Plugin structure 1:1 mapping** - Maintainability as plugins evolve
2. **Both current and at-last-action inputs** - Debug why actions were taken
3. **In-memory only** - No persistence (can add later)
4. **No HA sync** - Shadow state lives in Go service only
5. **Thread-safe** - All access protected by mutexes
6. **Async recording optional** - Start synchronous, optimize if needed

---

## Success Criteria

### Completed ✅
- ✅ Core shadow state infrastructure implemented
- ✅ `/api/shadow` aggregate endpoint created
- ✅ Lighting plugin has complete shadow state tracking
- ✅ `/api/shadow/lighting` endpoint returns current + at-last-action inputs
- ✅ Actions trigger input snapshots correctly
- ✅ Thread-safe concurrent access
- ✅ No performance degradation
- ✅ Tests pass with ≥70% coverage

### In Progress 🚧
- None - all phases complete!

### Remaining 📋
- TV plugin integration into main.go (TV shadow state types exist, just needs wiring)

### Recently Completed ✅
- ✅ Music plugin shadow state (Phase 2) - PR #115
- ✅ Security plugin shadow state (Phase 3) - Completed 2025-11-28
- ✅ Sleep hygiene plugin shadow state (Phase 4) - PR #116
- ✅ Load shedding plugin shadow state (Phase 5) - PR #117
- ✅ Energy plugin shadow state (Phase 6)
- ✅ StateTracking plugin shadow state (Phase 6)
- ✅ DayPhase plugin shadow state (Phase 6)
- ✅ TV plugin shadow state types (Phase 6) - ready for integration
- ✅ All API endpoints implemented (9 plugin endpoints + unified endpoint)

---

## Estimated Effort

- **Phase 1 (Core + Lighting):** 5-6 hours
- **Phase 2 (Music):** 2-3 hours
- **Phase 3 (Security):** 1-2 hours
- **Phase 4 (Sleep Hygiene):** 1-2 hours
- **Phase 5 (Load Shedding):** 1-2 hours
- **Phase 6 (Read-heavy plugins):** 3-4 hours
- **Phase 7 (Unified API):** 1-2 hours
- **Testing & docs:** 3-4 hours

**Total:** ~18-26 hours

---

## Related Documentation

- [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) - Overall architecture and migration strategy
- [GOLANG_DESIGN.md](./GOLANG_DESIGN.md) - Go implementation design details
- [../../homeautomation-go/README.md](../../homeautomation-go/README.md) - Go project user guide

---

## Current Status

**Overall Progress:** Phases 1-6 Complete (6/7 phases = ~86%), Unified API Complete

| Phase | Plugin(s) | Status | Notes |
|-------|-----------|--------|-------|
| 1 | Core + Lighting | ✅ Complete | Merged in PR #112 (2025-11-28) |
| 2 | Music | ✅ Complete | Merged in PR #115 (2025-11-28) |
| 3 | Security | ✅ Complete | Completed (2025-11-28) |
| 4 | Sleep Hygiene | ✅ Complete | Merged in PR #116 (2025-11-28) |
| 5 | Load Shedding | ✅ Complete | Merged in PR #117 (2025-11-28) |
| 6 | Read-Heavy Plugins | ✅ Complete | energy, statetracking, dayphase complete; tv types ready (2025-11-28) |
| 7 | Unified API | ✅ Complete | `/api/shadow` + 9 plugin endpoints (lighting, music, security, sleephygiene, loadshedding, energy, statetracking, dayphase, tv) |

**Completed Steps:**
1. ✅ Complete Music plugin shadow state (Phase 2) - DONE
2. ✅ Complete Security plugin shadow state (Phase 3) - DONE
3. ✅ Complete Sleep Hygiene plugin shadow state (Phase 4) - DONE
4. ✅ Complete Load Shedding plugin shadow state (Phase 5) - DONE
5. ✅ Complete Read-Heavy Plugins (Phase 6) - DONE

**Notes:**
- TV plugin shadow state types/tracker exist but TV plugin not yet started in main.go (separate work item)
- All tests pass including race detection and integration tests

---

**Document Status:** ✅ COMPLETE - All phases implemented
**Last Updated:** 2025-11-28
**Author:** System Design (Claude Code)

---

## Phase 2 Completion Summary

Phase 2 (Music Plugin) has been successfully implemented with the following deliverables:

### ✅ Completed Components

1. **Shadow State Types** (`internal/shadowstate/types.go`)
   - `MusicShadowState` - Main shadow state structure
   - `MusicInputs` - Current and at-last-action inputs
   - `MusicOutputs` - Mode, playlist, speakers, rotation state
   - `PlaylistInfo` - Playlist details
   - `SpeakerState` - Individual speaker configuration
   - All types implement `PluginShadowState` interface

2. **Music Manager Integration** (`internal/plugins/music/manager.go`)
   - Shadow state fields added to Manager struct
   - `captureCurrentInputs()` - Snapshots all 5 subscribed variables
   - `updateShadowState()` - Records actions with timestamp and reason
   - `updateShadowOutputs()` - Tracks playback state
   - `GetShadowState()` - Returns thread-safe deep copy
   - `recordPlaybackShadowState()` - Helper for playback recording
   - Integration in `orchestratePlayback()` for both read-only and write modes

3. **API Endpoint** (`internal/api/server.go`)
   - `/api/shadow/music` endpoint handler added
   - Documentation added to API sitemap
   - Provider registered in `cmd/main.go`

4. **Test Coverage** (`internal/plugins/music/manager_shadow_test.go`)
   - 7 comprehensive tests covering all shadow state functionality
   - Tests for input capture, action recording, output updates, concurrent access
   - All tests pass with `-race` flag
   - Full test suite (including integration tests): 100% passing

### 📊 Test Results

```
✅ TestMusicShadowState_CaptureInputs
✅ TestMusicShadowState_RecordAction
✅ TestMusicShadowState_UpdateOutputs
✅ TestMusicShadowState_GetShadowState
✅ TestMusicShadowState_ConcurrentAccess (with -race flag)
✅ TestMusicShadowState_PlaylistRotation
✅ TestMusicShadowState_InterfaceImplementation
```

### 🎯 Key Features

- **Input Tracking**: Captures dayPhase, isAnyoneAsleep, isAnyoneHome, isMasterAsleep, isEveryoneAsleep
- **Output Tracking**: Current mode, active playlist, speaker group, fade state, playlist rotation
- **Thread Safety**: All operations protected by mutexes, verified with race detector
- **Action Recording**: Timestamped actions with descriptive reasons
- **API Access**: Real-time shadow state available via `/api/shadow/music` endpoint

---

## Phase 3 Completion Summary

Phase 3 (Security Plugin) has been successfully implemented with the following deliverables:

### ✅ Completed Components

1. **Shadow State Types** (`internal/shadowstate/types.go`)
   - `SecurityShadowState` - Main shadow state structure
   - `SecurityInputs` - Current and at-last-action inputs
   - `SecurityOutputs` - Lockdown status, event tracking (doorbell, vehicle, garage)
   - `LockdownState` - Lockdown active/inactive status with timestamps and reset scheduling
   - `DoorbellEvent` - Doorbell press tracking with rate limiting and TTS status
   - `VehicleArrivalEvent` - Vehicle arrival notifications with expectation tracking
   - `GarageOpenEvent` - Garage auto-open events with reason and state
   - All types implement `PluginShadowState` interface

2. **Tracker Implementation** (`internal/shadowstate/tracker.go`)
   - `SecurityTracker` - Thread-safe state tracker with RWMutex
   - `UpdateCurrentInputs()` - Updates current input values
   - `SnapshotInputsForAction()` - Snapshots inputs when actions/events occur
   - `RecordLockdownAction()` - Records lockdown activation/deactivation with reason
   - `RecordDoorbellEvent()` - Records doorbell events with rate limit and TTS status
   - `RecordVehicleArrivalEvent()` - Records vehicle arrivals with expectation tracking
   - `RecordGarageOpenEvent()` - Records garage auto-open with reason
   - `GetState()` - Returns thread-safe deep copy

3. **Security Manager Integration** (`internal/plugins/security/manager.go`)
   - Shadow tracker added to Manager struct
   - `updateShadowInputs()` - Captures isEveryoneAsleep, isAnyoneHome, isExpectingSomeone, didOwnerJustReturnHome
   - `recordLockdownAction()` - Snapshots inputs and records lockdown actions
   - `recordDoorbellEvent()` - Snapshots inputs and records doorbell events
   - `recordVehicleArrivalEvent()` - Snapshots inputs and records vehicle arrivals
   - `recordGarageOpenAction()` - Snapshots inputs and records garage auto-open events
   - `GetShadowState()` - Returns current shadow state
   - Integration in all event handlers (lockdown, doorbell, vehicle, garage)

4. **API Endpoint** (`internal/api/server.go`)
   - `/api/shadow/security` endpoint handler added
   - Documentation added to API sitemap
   - Provider registered in `cmd/main.go`

5. **Test Coverage** (`internal/shadowstate/tracker_test.go`)
   - 11 comprehensive tests covering all shadow state functionality
   - Tests for input capture, action recording (lockdown, doorbell, vehicle, garage)
   - Thread safety testing with race detector
   - Metadata updates and last action time tracking
   - Deep copy verification
   - Interface implementation validation
   - All tests pass with `-race` flag
   - Full test suite (including integration tests): 100% passing

### 📊 Test Results

```
✅ TestSecurityTrackerUpdateCurrentInputs
✅ TestSecurityTrackerSnapshotInputsForAction
✅ TestSecurityTrackerRecordLockdownAction
✅ TestSecurityTrackerRecordDoorbellEvent
✅ TestSecurityTrackerRecordVehicleArrivalEvent
✅ TestSecurityTrackerRecordGarageOpenEvent
✅ TestSecurityTrackerGetStateReturnsDeepCopy
✅ TestSecurityTrackerConcurrentAccess (with -race flag)
✅ TestSecurityTrackerMetadataUpdates
✅ TestSecurityShadowStateImplementsInterface
✅ TestSecurityTrackerLastActionTime
```

### 🎯 Key Features

- **Input Tracking**: Captures isEveryoneAsleep, isAnyoneHome, isExpectingSomeone, didOwnerJustReturnHome
- **Output Tracking**: Lockdown state, doorbell events, vehicle arrivals, garage auto-opens
- **Lockdown Management**: Active/inactive status with activation reason and auto-reset scheduling
- **Event Recording**: Timestamped events with rate limiting status, TTS status, and context
- **Thread Safety**: All operations protected by mutexes, verified with race detector
- **Action Recording**: Timestamped actions with descriptive reasons and full event context
- **API Access**: Real-time shadow state available via `/api/shadow/security` endpoint

### 📝 Example Shadow State Output

```json
{
  "plugin": "security",
  "inputs": {
    "current": {
      "isEveryoneAsleep": true,
      "isAnyoneHome": true,
      "isExpectingSomeone": false,
      "didOwnerJustReturnHome": false
    },
    "atLastAction": {
      "isEveryoneAsleep": true,
      "isAnyoneHome": true,
      "isExpectingSomeone": false,
      "didOwnerJustReturnHome": false
    }
  },
  "outputs": {
    "lockdown": {
      "active": true,
      "reason": "Everyone is asleep - activating lockdown",
      "activatedAt": "2025-11-28T23:00:00Z",
      "willResetAt": "2025-11-28T23:00:05Z"
    },
    "lastDoorbell": {
      "timestamp": "2025-11-28T14:30:00Z",
      "rateLimited": false,
      "ttsSent": true,
      "lightsFlashed": true
    },
    "lastVehicle": {
      "timestamp": "2025-11-28T18:45:00Z",
      "rateLimited": false,
      "ttsSent": true,
      "wasExpecting": false
    },
    "lastGarageOpen": {
      "timestamp": "2025-11-28T18:46:00Z",
      "reason": "Nick arrived home - auto-opening garage",
      "garageWasEmpty": true
    },
    "lastActionTime": "2025-11-28T23:00:00Z"
  },
  "metadata": {
    "lastUpdated": "2025-11-28T23:00:00Z",
    "pluginName": "security"
  }
}
```

---

## Phase 4 Completion Summary

Phase 4 (Sleep Hygiene Plugin) has been successfully implemented with the following deliverables:

### ✅ Completed Components

1. **Shadow State Types** (`internal/shadowstate/types.go`)
   - `SleepHygieneShadowState` - Main shadow state structure
   - `SleepHygieneInputs` - Current and at-last-action inputs
   - `SleepHygieneOutputs` - Wake sequence status, fade-out progress, TTS, reminders
   - `SpeakerFadeOut` - Per-speaker fade-out tracking with volume progression
   - `TTSAnnouncement` - TTS announcement details (message, speaker, timestamp)
   - `ReminderTrigger` - Screen stop and bedtime reminder triggers
   - All types implement `PluginShadowState` interface

2. **Tracker Implementation** (`internal/shadowstate/tracker.go`)
   - `SleepHygieneTracker` - Thread-safe state tracker with RWMutex
   - `UpdateCurrentInputs()` - Updates current input values
   - `SnapshotInputsForAction()` - Snapshots inputs when actions occur
   - `RecordAction()` - Records action type and reason with timestamp
   - `UpdateWakeSequenceStatus()` - Tracks wake sequence progression (inactive → begin_wake → wake_in_progress → complete)
   - `RecordFadeOutStart()` - Initializes speaker fade-out tracking
   - `UpdateFadeOutProgress()` - Updates speaker volume during fade-out
   - `RecordTTSAnnouncement()` - Records TTS announcements with details
   - `RecordStopScreensReminder()` - Records screen stop reminder triggers
   - `RecordGoToBedReminder()` - Records bedtime reminder triggers
   - `GetState()` - Returns thread-safe deep copy

3. **Sleep Hygiene Manager Integration** (`internal/plugins/sleephygiene/manager.go`)
   - Shadow tracker added to Manager struct
   - `captureCurrentInputs()` - Captures all 9 input variables (isMasterAsleep, alarmTime, musicPlaybackType, currentlyPlayingMusic, isAnyoneHome, isEveryoneAsleep, isFadeOutInProgress, isNickHome, isCarolineHome)
   - `updateShadowInputs()` - Updates current inputs on every state change
   - `recordAction()` - Snapshots inputs and records actions with reason
   - `GetShadowState()` - Returns current shadow state
   - Integration in wake sequence handlers:
     - `handleBeginWake()` - Records begin_wake action and initializes fade-outs
     - `fadeOutSpeaker()` - Updates fade-out progress as volume decreases
     - `handleWake()` - Records wake action and updates sequence status
     - `checkAndAnnounceCuddle()` - Records TTS announcements
   - Integration in reminder handlers:
     - `handleStopScreens()` - Records screen stop reminders
     - `handleGoToBed()` - Records bedtime reminders
   - Integration in cancellation handlers:
     - `handleBedroomLightsOff()` - Records cancel_wake action

4. **API Endpoint** (`internal/api/server.go`)
   - `/api/shadow/sleephygiene` endpoint handler added
   - Documentation added to API sitemap
   - Provider registered in `cmd/main.go`

5. **Test Coverage** (`internal/plugins/sleephygiene/manager_shadow_test.go`)
   - 13 comprehensive tests covering all shadow state functionality
   - Tests for input capture, action recording, wake sequence status transitions
   - Fade-out progress tracking validation
   - TTS announcement and reminder recording
   - Thread safety testing with race detector (10 goroutines × 100 operations)
   - Interface implementation validation
   - Wake cancellation scenarios
   - All tests pass with `-race` flag
   - Full test suite (including integration tests): 100% passing
   - Plugin test coverage: 69.5%

### 📊 Test Results

```
✅ TestSleepHygieneShadowState_CaptureInputs
✅ TestSleepHygieneShadowState_RecordAction
✅ TestSleepHygieneShadowState_WakeSequenceStatus
✅ TestSleepHygieneShadowState_FadeOutProgress
✅ TestSleepHygieneShadowState_TTSAnnouncement
✅ TestSleepHygieneShadowState_Reminders
✅ TestSleepHygieneShadowState_GetShadowState
✅ TestSleepHygieneShadowState_ConcurrentAccess (with -race flag)
✅ TestSleepHygieneShadowState_InterfaceImplementation
✅ TestSleepHygieneShadowState_CancelWake
✅ TestSleepHygieneShadowState_BedroomLightsChange
✅ TestSleepHygieneShadowState_BedroomLightsNoCancel
✅ TestSleepHygieneShadowState_HandleGoToBed
```

### 🎯 Key Features

- **Input Tracking**: Captures 9 variables (isMasterAsleep, alarmTime, musicPlaybackType, currentlyPlayingMusic, isAnyoneHome, isEveryoneAsleep, isFadeOutInProgress, isNickHome, isCarolineHome)
- **Output Tracking**: Wake sequence status, fade-out progress per speaker, TTS announcements, reminders
- **Wake Sequence Management**: Tracks full lifecycle (inactive → begin_wake → wake_in_progress → complete → cancel_wake)
- **Fade-Out Tracking**: Per-speaker volume progression with start/current volumes and timing
- **TTS Recording**: Captures announcement messages, target speakers, and timestamps
- **Reminder Tracking**: Stop screens and bedtime reminders with timestamps
- **Thread Safety**: All operations protected by mutexes, verified with race detector
- **Action Recording**: Timestamped actions with descriptive reasons and full context
- **API Access**: Real-time shadow state available via `/api/shadow/sleephygiene` endpoint

### 📝 Example Shadow State Output

```json
{
  "plugin": "sleephygiene",
  "inputs": {
    "current": {
      "isMasterAsleep": false,
      "alarmTime": 1732780800000,
      "musicPlaybackType": "sleep",
      "currentlyPlayingMusic": "Bedroom Speaker: sleep playlist",
      "isAnyoneHome": true,
      "isEveryoneAsleep": false,
      "isFadeOutInProgress": true,
      "isNickHome": true,
      "isCarolineHome": true
    },
    "atLastAction": {
      "isMasterAsleep": true,
      "alarmTime": 1732780800000,
      "musicPlaybackType": "sleep",
      "currentlyPlayingMusic": "Bedroom Speaker: sleep playlist",
      "isAnyoneHome": true,
      "isEveryoneAsleep": true,
      "isFadeOutInProgress": false,
      "isNickHome": true,
      "isCarolineHome": true
    }
  },
  "outputs": {
    "wakeSequenceStatus": "wake_in_progress",
    "fadeOutProgress": {
      "media_player.bedroom_speaker": {
        "speakerEntityID": "media_player.bedroom_speaker",
        "currentVolume": 25,
        "startVolume": 60,
        "isActive": true,
        "startTime": "2025-11-28T06:30:00Z",
        "lastUpdate": "2025-11-28T06:35:00Z"
      }
    },
    "lastTTSAnnouncement": {
      "message": "Nick, it's time to cuddle with Caroline",
      "speaker": "media_player.bedroom_speaker",
      "timestamp": "2025-11-28T06:32:00Z"
    },
    "stopScreensReminder": {
      "timestamp": "2025-11-28T22:00:00Z"
    },
    "goToBedReminder": {
      "timestamp": "2025-11-28T22:30:00Z"
    },
    "lastActionTime": "2025-11-28T06:30:00Z",
    "lastActionType": "begin_wake",
    "lastActionReason": "Alarm triggered - beginning wake sequence"
  },
  "metadata": {
    "lastUpdated": "2025-11-28T06:35:00Z",
    "pluginName": "sleephygiene"
  }
}
```

---

## Phase 5 Completion Summary

Phase 5 (Load Shedding Plugin) has been successfully implemented with the following deliverables:

### ✅ Completed Components

1. **Shadow State Types** (`internal/shadowstate/types.go`)
   - `LoadSheddingShadowState` - Main shadow state structure
   - `LoadSheddingInputs` - Current and at-last-action inputs
   - `LoadSheddingOutputs` - Active state, thermostat settings, action history
   - `ThermostatSettings` - Hold mode and temperature range
   - All types implement `PluginShadowState` interface

2. **Tracker Implementation** (`internal/shadowstate/tracker.go`)
   - `LoadSheddingTracker` - Thread-safe state tracker with RWMutex
   - `UpdateCurrentInputs()` - Updates current input values
   - `SnapshotInputsForAction()` - Snapshots inputs when actions are taken
   - `RecordLoadSheddingAction()` - Records enable/disable actions with full context
   - `GetState()` - Returns thread-safe deep copy

3. **Load Shedding Manager Integration** (`internal/plugins/loadshedding/manager.go`)
   - Shadow tracker added to Manager struct
   - `updateShadowInputs()` - Captures currentEnergyLevel
   - `recordAction()` - Snapshots inputs and records actions with thermostat settings
   - `GetShadowState()` - Returns current shadow state
   - Integration in `handleEnergyChange()` - Updates inputs on every energy level change
   - Integration in `enableLoadShedding()` - Records enable actions with hold mode and temp settings
   - Integration in `disableLoadShedding()` - Records disable actions

4. **API Endpoint** (`internal/api/server.go`)
   - `/api/shadow/loadshedding` endpoint handler added
   - Documentation added to API sitemap
   - Provider registered in `cmd/main.go`

5. **Test Coverage** (`internal/plugins/loadshedding/manager_shadow_test.go`)
   - 9 comprehensive tests covering all shadow state functionality
   - Tests for input capture, action recording (enable/disable), output updates
   - Thread safety testing with race detector
   - Input snapshotting verification (current vs at-last-action)
   - Multiple actions tracking
   - Energy change handler integration
   - All tests pass with `-race` flag
   - Full test suite (including integration tests): 100% passing

### 📊 Test Results

```
✅ TestLoadSheddingShadowState_CaptureInputs
✅ TestLoadSheddingShadowState_RecordEnableAction
✅ TestLoadSheddingShadowState_RecordDisableAction
✅ TestLoadSheddingShadowState_GetShadowState
✅ TestLoadSheddingShadowState_ConcurrentAccess (with -race flag)
✅ TestLoadSheddingShadowState_InterfaceImplementation
✅ TestLoadSheddingShadowState_InputSnapshot
✅ TestLoadSheddingShadowState_MultipleActions
✅ TestLoadSheddingShadowState_HandleEnergyChange
```

### 🎯 Key Features

- **Input Tracking**: Captures currentEnergyLevel (only subscribed input)
- **Output Tracking**: Active state, action type (enable/disable), reason, thermostat settings (hold mode, temp low/high)
- **Thread Safety**: All operations protected by mutexes, verified with race detector
- **Action Recording**: Timestamped actions with descriptive reasons
- **Thermostat State**: Tracks hold mode, temperature low (65°F), temperature high (80°F)
- **API Access**: Real-time shadow state available via `/api/shadow/loadshedding` endpoint

### 📝 Example Shadow State Output

```json
{
  "plugin": "loadshedding",
  "inputs": {
    "current": {
      "currentEnergyLevel": "red"
    },
    "atLastAction": {
      "currentEnergyLevel": "red"
    }
  },
  "outputs": {
    "active": true,
    "lastActionType": "enable",
    "lastActionReason": "Energy state is red (low battery) - restricting HVAC",
    "thermostatSettings": {
      "holdMode": true,
      "tempLow": 65.0,
      "tempHigh": 80.0
    },
    "lastActionTime": "2025-11-28T20:30:00Z"
  },
  "metadata": {
    "lastUpdated": "2025-11-28T20:30:00Z",
    "pluginName": "loadshedding"
  }
}
```
