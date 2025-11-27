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

### Phase 1: Core Infrastructure + Lighting Plugin (Pilot)

**1.1 Create Shadow State Package**
- `internal/shadowstate/tracker.go` - Core tracker
- `internal/shadowstate/types.go` - Common types (InputSnapshot, ActionRecord, etc.)
- Thread-safe recording with mutexes

**1.2 Define Common Interfaces**
```go
type PluginShadowState interface {
    GetCurrentInputs() map[string]interface{}
    GetLastActionInputs() map[string]interface{}
    GetOutputs() interface{}
    GetMetadata() StateMetadata
}
```

**1.3 Implement Lighting Shadow State**
- Track subscribed variables: `dayPhase`, `sunevent`, `isAnyoneHome`, `isTVPlaying`, `isEveryoneAsleep`, `isMasterAsleep`, `isHaveGuests`
- Snapshot inputs on every action
- Track output state: active scene per room, last action, reason
- Implement `GetShadowState()` on `lighting.Manager`

**1.4 Add API Endpoint**
- `/api/shadow/lighting` - Returns lighting shadow state
- Test with existing lighting triggers

**Validation:**
- ✅ Changing `dayPhase` shows up in current inputs
- ✅ Scene activation snapshots inputs and records output
- ✅ API returns both current and at-last-action values

---

### Phase 2: Music Plugin

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

### Phase 4: Sleep Hygiene Plugin

**4.1 Define Sleep Hygiene Shadow State**
- **Inputs:** `isMasterAsleep`, `alarmTime`, `musicPlaybackType`, `currentlyPlayingMusic`
- **Outputs:**
  - Wake sequence status (inactive/in_progress/complete)
  - Fade-out progress per speaker
  - Last TTS announcement
  - Screen stop / bedtime reminder triggers

**4.2 Add API Endpoint**
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

### Phase 6: Read-Heavy Plugins

**6.1 Energy Plugin**
- **Inputs:** HA sensor entities (battery, solar, grid)
- **Outputs:** Computed energy levels (`batteryEnergyLevel`, `currentEnergyLevel`, `solarProductionEnergyLevel`)

**6.2 State Tracking Plugin**
- **Inputs:** HA presence/door/sleep sensors
- **Outputs:** Computed presence/sleep states (`isAnyOwnerHome`, `isAnyoneHome`, `isAnyoneAsleep`, etc.)

**6.3 Day Phase Plugin**
- **Inputs:** HA sun/time sensors
- **Outputs:** Computed `dayPhase`, `sunevent`

**6.4 TV Plugin**
- **Inputs:** HA media_player states
- **Outputs:** Computed TV states (`isTVPlaying`, `isAppleTVPlaying`, `isTVon`)

**6.5 Reset Plugin**
- **Inputs:** `reset` variable
- **Outputs:** Reset triggers, affected variables

**6.6 Add API Endpoints**
- `/api/shadow/energy`
- `/api/shadow/statetracking`
- `/api/shadow/dayphase`
- `/api/shadow/tv`
- `/api/shadow/reset`

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

Each plugin manager implements:
```go
type ShadowStateProvider interface {
    GetShadowState() PluginShadowState
    RecordAction(actionType string, reason string, details interface{})
}
```

When a plugin takes action:
```go
// In lighting.Manager.activateScene()
m.RecordAction("activate_scene",
    fmt.Sprintf("dayPhase changed from '%s' to '%s'", oldPhase, newPhase),
    map[string]interface{}{
        "room": roomName,
        "scene": sceneName,
    })
```

---

## Testing Strategy

**Unit Tests (per plugin):**
- Input snapshot capture
- Output state updates
- Thread safety (concurrent reads/writes)
- API response formatting

**Integration Tests:**
- End-to-end: Trigger state change → Action taken → Shadow state updated → API returns correct data
- Verify current vs. at-last-action input values differ correctly
- Verify all plugins represented in `/api/shadow`

**Manual Testing:**
- Change `dayPhase` → verify `/api/shadow/lighting` shows scene changes
- Play music → verify `/api/shadow/music` shows mode/playlist
- Trigger lockdown → verify `/api/shadow/security` shows activation

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

- ✅ All 10 plugins have shadow state endpoints
- ✅ Each shows current + at-last-action input values
- ✅ Each shows plugin-specific output state
- ✅ `/api/shadow` returns complete home state snapshot
- ✅ Actions trigger input snapshots correctly
- ✅ No performance degradation
- ✅ Tests pass with ≥70% coverage

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

**Document Status:** Planning
**Last Updated:** 2025-11-27
**Author:** System Design (Claude Code)
