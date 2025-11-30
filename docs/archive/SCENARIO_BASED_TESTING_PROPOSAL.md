# Scenario-Based Integration Testing Proposal

## Executive Summary

**Current State:** Integration tests validate infrastructure (WebSocket, state management, concurrency) but do NOT test automation logic.

**Gap Identified:** No tests validate that automation plugins respond correctly to events and take the right actions.

**Proposal:** Add scenario-based integration tests that simulate real-world automation workflows and verify correct behavior.

---

## Current Integration Test Coverage

### ✅ What We Test Now (11 passing tests)

**Infrastructure Testing:**
- WebSocket connection and authentication
- State synchronization (bidirectional)
- Subscription mechanisms
- Concurrency (50 goroutines × 100 reads, 20 goroutines × 50 writes)
- Race conditions (all tests pass with `-race` flag)
- Deadlock scenarios (subscription callbacks triggering more operations)
- High-frequency state changes (1000+ events)
- Reconnection after disconnect

**File:** `homeautomation-go/test/integration/integration_test.go`

### ❌ What We DON'T Test (The Gap)

**Automation Logic / Business Rules:**
- ❌ When `dayPhase` changes to `"evening"`, do the correct lighting scenes activate?
- ❌ When battery level drops below threshold, does `batteryEnergyLevel` update to `"red"`?
- ❌ When `alarmTime` is reached, does the wake-up sequence trigger?
- ❌ When `isNickHome` → `false` AND `isCarolineHome` → `false`, does security lockdown activate?
- ❌ When TV turns on during `"evening"`, do living room lights dim?
- ❌ Do service calls get sent to the correct Home Assistant entities?

**Current Problem:**
The mock HA server (`mock_ha_server.go`) handles service calls but **doesn't track them**, making it impossible to verify that plugins called the correct services with the correct parameters.

---

## Automation Plugins That Need Testing

### 1. Lighting Control Plugin (`internal/plugins/lighting/`)
**What it does:**
- Subscribes to `dayPhase`, `sunevent`, `isAnyoneHome`, `isTVPlaying`, `isEveryoneAsleep`, `isMasterAsleep`, `isHaveGuests`
- Activates Hue scenes based on state changes
- Calls `scene.activate` with conditional logic (on_if_true, on_if_false, etc.)

**Test scenarios needed:**
- ✅ Scenario: Day phase changes → verify correct scenes activated for each room
- ✅ Scenario: TV turns on → verify brightness adjustments
- ✅ Scenario: Everyone goes to sleep → verify lights turn off or switch to night mode

### 2. Energy State Plugin (`internal/plugins/energy/`)
**What it does:**
- Monitors battery, solar production, grid availability
- Calculates energy levels (`batteryEnergyLevel`, `solarProductionEnergyLevel`, `currentEnergyLevel`)
- Updates state variables with color codes (green, yellow, orange, red, white)
- Runs periodic free energy checker

**Test scenarios needed:**
- ✅ Scenario: Battery drops from 80% → 40% → verify level changes from green → yellow → orange
- ✅ Scenario: Solar production spikes → verify `solarProductionEnergyLevel` updates
- ✅ Scenario: Grid goes offline → verify `isFreeEnergyAvailable` recalculates
- ✅ Scenario: Overall energy level changes → verify `currentEnergyLevel` reflects worst state

### 3. TV Monitoring Plugin (`internal/plugins/tv/`)
**What it does:**
- Monitors Apple TV media player state (`media_player.big_beautiful_oled`)
- Monitors sync box power (`switch.sync_box_power`)
- Monitors HDMI input selector (`select.sync_box_hdmi_input`)
- Updates `isAppleTVPlaying`, `isTVon`, `isTVPlaying`

**Test scenarios needed:**
- ✅ Scenario: Apple TV starts playing → verify `isAppleTVPlaying` = true
- ✅ Scenario: Sync box power on + HDMI input = "Apple TV" → verify `isTVPlaying` = true
- ✅ Scenario: HDMI input switches to "Xbox" → verify `isTVPlaying` = false (unless Xbox is playing)

### 4. Sleep Hygiene Plugin (`internal/plugins/sleephygiene/`)
**What it does:**
- Monitors `alarmTime` and triggers wake-up sequences
- Fade-out sleep music (begin_wake)
- Full wake sequence (lights, flashing, cuddle announcement)
- Evening "stop screens" reminder
- Daily trigger reset at midnight

**Test scenarios needed:**
- ✅ Scenario: Alarm time reached → verify wake sequence triggers
- ✅ Scenario: Wake sequence → verify lights flash, music fades, announcement plays
- ✅ Scenario: Midnight → verify trigger reset
- ✅ Scenario: Evening reminder → verify TTS notification sent

---

## Proposed Solution: Enhanced Mock Server + Scenario Tests

### Phase 1: Enhance MockHAServer to Track Service Calls

**File to modify:** `homeautomation-go/test/integration/mock_ha_server.go`

**Add service call tracking:**
```go
// MockHAServer enhancement
type MockHAServer struct {
    // ... existing fields ...
    serviceCalls []ServiceCall  // NEW: Track all service calls
    callsMu      sync.Mutex     // NEW: Protect service calls
}

// ServiceCall records a service call for testing
type ServiceCall struct {
    Timestamp   time.Time
    Domain      string
    Service     string
    ServiceData map[string]interface{}
}

// GetServiceCalls returns all service calls since last clear
func (s *MockHAServer) GetServiceCalls() []ServiceCall {
    s.callsMu.Lock()
    defer s.callsMu.Unlock()
    calls := make([]ServiceCall, len(s.serviceCalls))
    copy(calls, s.serviceCalls)
    return calls
}

// ClearServiceCalls resets the service call log
func (s *MockHAServer) ClearServiceCalls() {
    s.callsMu.Lock()
    defer s.callsMu.Unlock()
    s.serviceCalls = nil
}

// FindServiceCall finds a service call matching criteria
func (s *MockHAServer) FindServiceCall(domain, service string, entityID string) *ServiceCall {
    s.callsMu.Lock()
    defer s.callsMu.Unlock()

    for i := len(s.serviceCalls) - 1; i >= 0; i-- {
        call := s.serviceCalls[i]
        if call.Domain == domain && call.Service == service {
            if entityID == "" {
                return &call
            }
            if eid, ok := call.ServiceData["entity_id"].(string); ok && eid == entityID {
                return &call
            }
        }
    }
    return nil
}
```

**Update handleCallService:**
```go
func (s *MockHAServer) handleCallService(wrapper *connWrapper, msg json.RawMessage) {
    var req CallServiceRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return
    }

    // NEW: Track the service call
    s.callsMu.Lock()
    s.serviceCalls = append(s.serviceCalls, ServiceCall{
        Timestamp:   time.Now(),
        Domain:      req.Domain,
        Service:     req.Service,
        ServiceData: req.ServiceData,
    })
    s.callsMu.Unlock()

    // ... rest of existing code ...
}
```

### Phase 2: Create Scenario-Based Test Framework

**New file:** `homeautomation-go/test/integration/scenario_test.go`

**Test structure:**
```go
package integration

import (
    "testing"
    "time"

    "homeautomation/internal/plugins/lighting"
    "homeautomation/internal/plugins/energy"
    "homeautomation/internal/plugins/tv"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// setupScenarioTest creates test environment with plugins
func setupScenarioTest(t *testing.T) (*MockHAServer, *state.Manager, *ScenarioPlugins, func()) {
    server, client, manager, baseCleanup := setupTest(t)

    // Load configs
    lightingConfig := loadTestLightingConfig(t)
    energyConfig := loadTestEnergyConfig(t)

    // Create plugins
    plugins := &ScenarioPlugins{
        Lighting: lighting.NewManager(client, manager, lightingConfig, logger, false),
        Energy:   energy.NewManager(client, manager, energyConfig, logger, false, nil),
        TV:       tv.NewManager(client, manager, logger, false),
    }

    // Start all plugins
    require.NoError(t, plugins.Lighting.Start())
    require.NoError(t, plugins.Energy.Start())
    require.NoError(t, plugins.TV.Start())

    cleanup := func() {
        plugins.Lighting.Stop()
        plugins.Energy.Stop()
        plugins.TV.Stop()
        baseCleanup()
    }

    return server, manager, plugins, cleanup
}

type ScenarioPlugins struct {
    Lighting *lighting.Manager
    Energy   *energy.Manager
    TV       *tv.Manager
}
```

### Phase 3: Example Scenario Tests

#### Test 1: Lighting - Day Phase Change Activates Scenes

```go
func TestScenario_DayPhaseChangeActivatesScenes(t *testing.T) {
    server, manager, plugins, cleanup := setupScenarioTest(t)
    defer cleanup()

    // Clear any initialization service calls
    server.ClearServiceCalls()

    // GIVEN: Current day phase is "morning"
    require.NoError(t, manager.SetString("dayPhase", "morning"))
    time.Sleep(100 * time.Millisecond) // Allow state to propagate

    // WHEN: Day phase changes to "evening"
    server.SetState("input_text.day_phase", "evening", map[string]interface{}{})

    // THEN: Wait for automation to react
    time.Sleep(500 * time.Millisecond)

    // VERIFY: Correct scenes were activated
    calls := server.GetServiceCalls()

    // Should have called scene.activate for multiple rooms
    sceneActivations := filterServiceCalls(calls, "scene", "activate")
    assert.Greater(t, len(sceneActivations), 0, "Should activate at least one scene")

    // Check specific scene (e.g., living room evening scene)
    livingRoomCall := findServiceCallWithData(calls, "scene", "activate",
        "entity_id", "scene.living_room_evening")
    assert.NotNil(t, livingRoomCall, "Should activate living room evening scene")

    // VERIFY: State manager reflects the change
    phase, err := manager.GetString("dayPhase")
    assert.NoError(t, err)
    assert.Equal(t, "evening", phase)
}
```

#### Test 2: Energy - Battery Level Change Updates Energy State

```go
func TestScenario_BatteryLevelChangeUpdatesEnergyState(t *testing.T) {
    server, manager, plugins, cleanup := setupScenarioTest(t)
    defer cleanup()

    // GIVEN: Battery is at 80% (green level)
    server.SetState("sensor.span_panel_span_storage_battery_percentage_2", "80.0",
        map[string]interface{}{
            "unit_of_measurement": "%",
        })
    time.Sleep(200 * time.Millisecond)

    // Verify initial state
    batteryLevel, err := manager.GetString("batteryEnergyLevel")
    require.NoError(t, err)
    assert.Equal(t, "green", batteryLevel)

    // WHEN: Battery drops to 40% (yellow level)
    server.SetState("sensor.span_panel_span_storage_battery_percentage_2", "40.0",
        map[string]interface{}{
            "unit_of_measurement": "%",
        })

    // THEN: Wait for automation to react
    time.Sleep(500 * time.Millisecond)

    // VERIFY: Battery energy level updated to yellow
    batteryLevel, err = manager.GetString("batteryEnergyLevel")
    assert.NoError(t, err)
    assert.Equal(t, "yellow", batteryLevel)

    // WHEN: Battery drops to 15% (red level)
    server.SetState("sensor.span_panel_span_storage_battery_percentage_2", "15.0",
        map[string]interface{}{
            "unit_of_measurement": "%",
        })
    time.Sleep(500 * time.Millisecond)

    // VERIFY: Battery energy level updated to red
    batteryLevel, err = manager.GetString("batteryEnergyLevel")
    assert.NoError(t, err)
    assert.Equal(t, "red", batteryLevel)
}
```

#### Test 3: TV Monitoring - Apple TV Playing Updates TV State

```go
func TestScenario_AppleTVPlayingUpdatesTVState(t *testing.T) {
    server, manager, plugins, cleanup := setupScenarioTest(t)
    defer cleanup()

    // GIVEN: Apple TV is idle, sync box is on, HDMI input is "Apple TV"
    server.SetState("media_player.big_beautiful_oled", "idle",
        map[string]interface{}{
            "friendly_name": "Apple TV",
        })
    server.SetState("switch.sync_box_power", "on", map[string]interface{}{})
    server.SetState("select.sync_box_hdmi_input", "Apple TV", map[string]interface{}{})
    time.Sleep(200 * time.Millisecond)

    // Verify initial state
    isTVPlaying, _ := manager.GetBool("isTVPlaying")
    assert.False(t, isTVPlaying, "TV should not be playing initially")

    // WHEN: Apple TV starts playing
    server.SetState("media_player.big_beautiful_oled", "playing",
        map[string]interface{}{
            "friendly_name": "Apple TV",
        })

    // THEN: Wait for automation to react
    time.Sleep(500 * time.Millisecond)

    // VERIFY: isTVPlaying is true
    isTVPlaying, err := manager.GetBool("isTVPlaying")
    assert.NoError(t, err)
    assert.True(t, isTVPlaying, "TV should be playing when Apple TV is active")

    // VERIFY: isAppleTVPlaying is true
    isAppleTVPlaying, err := manager.GetBool("isAppleTVPlaying")
    assert.NoError(t, err)
    assert.True(t, isAppleTVPlaying)
}
```

#### Test 4: Lighting + TV Integration - TV Playing Dims Lights

```go
func TestScenario_TVPlayingDimsLivingRoomLights(t *testing.T) {
    server, manager, plugins, cleanup := setupScenarioTest(t)
    defer cleanup()

    server.ClearServiceCalls()

    // GIVEN: Evening, someone is home, TV is off
    require.NoError(t, manager.SetString("dayPhase", "evening"))
    require.NoError(t, manager.SetBool("isAnyoneHome", true))
    require.NoError(t, manager.SetBool("isTVPlaying", false))
    time.Sleep(200 * time.Millisecond)

    server.ClearServiceCalls() // Clear scene activation from initial setup

    // WHEN: TV starts playing
    require.NoError(t, manager.SetBool("isTVPlaying", true))

    // THEN: Wait for automation to react
    time.Sleep(500 * time.Millisecond)

    // VERIFY: Living room scene reactivated (possibly with dimmed brightness)
    calls := server.GetServiceCalls()
    sceneActivations := filterServiceCalls(calls, "scene", "activate")

    // Should have activated a scene in response to TV state change
    assert.Greater(t, len(sceneActivations), 0,
        "Should activate scene when TV state changes")
}
```

#### Test 5: Multi-Condition Automation - Security Lockdown

```go
func TestScenario_EveryoneAwayActivatesSecurityLockdown(t *testing.T) {
    server, manager, plugins, cleanup := setupScenarioTest(t)
    defer cleanup()

    // Note: This test assumes a Security plugin exists (not yet implemented)
    // This is a FUTURE test scenario

    server.ClearServiceCalls()

    // GIVEN: Nick is home, Caroline is home
    require.NoError(t, manager.SetBool("isNickHome", true))
    require.NoError(t, manager.SetBool("isCarolineHome", true))
    require.NoError(t, manager.SetBool("isExpectingSomeone", false))
    time.Sleep(200 * time.Millisecond)

    // WHEN: Both leave (isNickHome → false, then isCarolineHome → false)
    server.SetState("input_boolean.nick_home", "off", map[string]interface{}{})
    time.Sleep(200 * time.Millisecond)

    server.SetState("input_boolean.caroline_home", "off", map[string]interface{}{})
    time.Sleep(500 * time.Millisecond)

    // VERIFY: isAnyoneHome is false
    isAnyoneHome, err := manager.GetBool("isAnyoneHome")
    assert.NoError(t, err)
    assert.False(t, isAnyoneHome)

    // VERIFY: Security actions taken (when security plugin exists)
    // - Garage door closed
    // - Locks engaged
    // - Alarm armed
    // - Lights set to away mode

    // TODO: Add assertions when security plugin is implemented
}
```

---

## Implementation Roadmap

### ✅ Phase 1: Mock Server Enhancement (1-2 hours)
**Tasks:**
1. Add `serviceCalls []ServiceCall` to `MockHAServer`
2. Add `GetServiceCalls()`, `ClearServiceCalls()`, `FindServiceCall()` methods
3. Update `handleCallService()` to track calls
4. Add helper functions for filtering service calls
5. Add unit tests for tracking mechanism

**Deliverable:** Enhanced mock server that tracks all service calls

### ✅ Phase 2: Test Infrastructure (2-3 hours)
**Tasks:**
1. Create `scenario_test.go` file
2. Implement `setupScenarioTest()` helper
3. Create test config files (minimal lighting, energy configs for testing)
4. Add helper functions:
   - `filterServiceCalls(calls, domain, service)`
   - `findServiceCallWithData(calls, domain, service, key, value)`
   - `assertServiceCalled(t, server, domain, service, entityID)`
5. Document testing patterns

**Deliverable:** Reusable scenario test framework

### ✅ Phase 3: Lighting Plugin Scenarios (2-4 hours)
**Tests to add:**
1. `TestScenario_DayPhaseChangeActivatesScenes`
2. `TestScenario_SunEventChangeActivatesScenes`
3. `TestScenario_TVPlayingDimsLights`
4. `TestScenario_EveryoneAsleepTurnsOffLights`
5. `TestScenario_PresenceChangeAffectsLighting`

**Deliverable:** 5+ lighting scenario tests

### ✅ Phase 4: Energy Plugin Scenarios (2-3 hours)
**Tests to add:**
1. `TestScenario_BatteryLevelChangeUpdatesEnergyState`
2. `TestScenario_SolarProductionUpdatesEnergyLevel`
3. `TestScenario_GridOfflineRecalculatesFreeEnergy`
4. `TestScenario_OverallEnergyLevelReflectsWorstState`
5. `TestScenario_FreeEnergyCheckerPeriodicUpdate`

**Deliverable:** 5+ energy scenario tests

### ✅ Phase 5: TV + Sleep Hygiene Scenarios (2-3 hours)
**Tests to add:**
1. `TestScenario_AppleTVPlayingUpdatesTVState`
2. `TestScenario_HDMIInputSwitchUpdatesTVState`
3. `TestScenario_AlarmTimeTriggersWakeSequence` (Sleep Hygiene)
4. `TestScenario_MidnightResetsTriggers` (Sleep Hygiene)
5. `TestScenario_EveningReminderSent` (Sleep Hygiene)

**Deliverable:** 5+ TV and sleep scenario tests

### ✅ Phase 6: Documentation + CI Integration (1 hour)
**Tasks:**
1. Update `test/integration/README.md` with scenario testing guide
2. Add scenario tests to CI workflow
3. Document how to write new scenario tests
4. Create scenario testing best practices guide

**Deliverable:** Complete documentation and CI integration

---

## Benefits of Scenario-Based Testing

### 1. **Catches Business Logic Bugs**
Infrastructure tests catch concurrency bugs, but scenario tests catch logic bugs:
- ❌ Wrong scene activated for day phase
- ❌ Energy level thresholds misconfigured
- ❌ TV state not updating correctly
- ❌ Automation triggered at wrong time

### 2. **Validates Node-RED Migration**
Each scenario test validates that Go behavior matches Node-RED:
- ✅ Compare side-by-side with Node-RED behavior
- ✅ Catch regressions during migration
- ✅ Verify business rules are preserved

### 3. **Documents Expected Behavior**
Scenario tests serve as executable documentation:
- ✅ New developers understand automation logic
- ✅ Clear examples of how system should behave
- ✅ Test names describe features (Given/When/Then)

### 4. **Enables Confident Refactoring**
With comprehensive scenario tests:
- ✅ Refactor plugin internals safely
- ✅ Change implementation without breaking behavior
- ✅ Catch regressions immediately

### 5. **Facilitates Parallel Testing**
Run Go implementation and Node-RED side-by-side:
- ✅ Trigger same event in both systems
- ✅ Compare service calls and state changes
- ✅ Validate identical behavior before cutover

---

## Specific Test Scenarios to Implement

### High Priority (Implement First)

#### Lighting Control (5 tests)
1. ✅ **Day Phase → Evening**: Activates evening scenes for all rooms
2. ✅ **Sun Event → Sunset**: Activates sunset-appropriate scenes
3. ✅ **TV Playing + Evening**: Dims living room lights
4. ✅ **Everyone Asleep**: Turns off all lights or switches to night mode
5. ✅ **Guest Arrives**: Activates guest bedroom scene

#### Energy State (5 tests)
1. ✅ **Battery 80% → 40%**: batteryEnergyLevel changes green → yellow
2. ✅ **Battery 40% → 15%**: batteryEnergyLevel changes yellow → red
3. ✅ **Solar Production Spike**: solarProductionEnergyLevel updates
4. ✅ **Grid Offline**: isFreeEnergyAvailable recalculates
5. ✅ **Overall Energy Level**: Reflects worst of battery/solar/grid

#### TV Monitoring (3 tests)
1. ✅ **Apple TV Playing**: isTVPlaying = true, isAppleTVPlaying = true
2. ✅ **HDMI Input Switch**: isTVPlaying updates based on active input
3. ✅ **TV Off**: All TV state variables = false

### Medium Priority (Implement After High Priority)

#### Sleep Hygiene (4 tests)
1. ✅ **Alarm Time Reached**: Triggers wake sequence
2. ✅ **Wake Sequence**: Lights flash, music fades, announcement plays
3. ✅ **Midnight Reset**: begin_wake and full_wake triggers reset
4. ✅ **Evening Reminder**: "Stop screens" TTS notification

#### Multi-Plugin Integration (3 tests)
1. ✅ **TV + Lighting**: TV state affects lighting scenes
2. ✅ **Energy + Lighting**: Low energy disables certain scenes
3. ✅ **Presence + All**: isAnyoneHome affects multiple plugins

### Future Tests (Implement When Plugins Are Ready)

#### Security Plugin (not yet implemented)
1. 🔜 **Everyone Away**: Garage closes, locks engage, alarm arms
2. 🔜 **Everyone Asleep**: Night mode security enabled
3. 🔜 **Unexpected Entry**: Alert notifications sent

#### Music Plugin (not yet implemented)
1. 🔜 **Day Phase Change**: Switches music playlist
2. 🔜 **Sleep State Change**: Fades out music when going to sleep
3. 🔜 **Alarm Wake**: Plays wake-up music

---

## Example: Complete Scenario Test

Here's a complete example showing best practices:

```go
func TestScenario_DayPhaseEvening_ActivatesCorrectScenes(t *testing.T) {
    // ========== SETUP ==========
    server, manager, plugins, cleanup := setupScenarioTest(t)
    defer cleanup()

    // Clear service calls from initialization
    server.ClearServiceCalls()

    // ========== GIVEN ==========
    // Current state: Morning, Nick is home, no guests
    t.Log("GIVEN: Day phase is morning, Nick is home")
    require.NoError(t, manager.SetString("dayPhase", "morning"))
    require.NoError(t, manager.SetBool("isNickHome", true))
    require.NoError(t, manager.SetBool("isAnyoneHome", true))
    require.NoError(t, manager.SetBool("isHaveGuests", false))
    time.Sleep(200 * time.Millisecond) // Allow state to settle

    server.ClearServiceCalls() // Clear any setup calls

    // ========== WHEN ==========
    // Trigger: Day phase changes to evening
    t.Log("WHEN: Day phase changes to evening")
    server.SetState("input_text.day_phase", "evening", map[string]interface{}{})

    // Wait for automation to react
    time.Sleep(500 * time.Millisecond)

    // ========== THEN ==========
    t.Log("THEN: Verify correct scenes were activated")

    calls := server.GetServiceCalls()
    t.Logf("Total service calls: %d", len(calls))

    // Filter to scene activations only
    sceneActivations := filterServiceCalls(calls, "scene", "activate")
    t.Logf("Scene activations: %d", len(sceneActivations))

    // ASSERTION 1: At least one scene was activated
    assert.Greater(t, len(sceneActivations), 0,
        "Should activate at least one scene when day phase changes")

    // ASSERTION 2: Living room evening scene was activated
    livingRoomCall := findServiceCallWithData(calls, "scene", "activate",
        "entity_id", "scene.living_room_evening")
    assert.NotNil(t, livingRoomCall,
        "Should activate living room evening scene")

    // ASSERTION 3: Guest bedroom scene was NOT activated (no guests)
    guestRoomCall := findServiceCallWithData(calls, "scene", "activate",
        "entity_id", "scene.guest_bedroom_evening")
    assert.Nil(t, guestRoomCall,
        "Should NOT activate guest bedroom scene when no guests")

    // ASSERTION 4: State manager reflects the change
    dayPhase, err := manager.GetString("dayPhase")
    assert.NoError(t, err)
    assert.Equal(t, "evening", dayPhase)

    // ========== CLEANUP ==========
    // (Handled by defer cleanup())
}
```

---

## Comparison: Current vs. Proposed Testing

| Aspect | Current Integration Tests | Proposed Scenario Tests |
|--------|---------------------------|-------------------------|
| **Focus** | Infrastructure (WebSocket, state sync, concurrency) | Business logic (automation behavior) |
| **What's Tested** | State manager, HA client, subscriptions | Plugin automation logic, service calls |
| **Test Count** | 11 tests | 15-20 scenario tests (initial) |
| **Coverage** | Technical correctness | Functional correctness |
| **Bug Detection** | Race conditions, deadlocks, memory leaks | Logic errors, wrong scenes, incorrect thresholds |
| **Documentation Value** | How system works internally | What system does for users |
| **Migration Validation** | ❌ Doesn't validate Node-RED behavior match | ✅ Validates behavior matches Node-RED |

---

## Next Steps

### Immediate Actions (This Week)
1. ✅ **Review this proposal** with project stakeholders
2. ✅ **Approve approach** for scenario testing
3. ✅ **Implement Phase 1** (Mock server enhancement)
4. ✅ **Implement Phase 2** (Test infrastructure)

### Short Term (Next 2 Weeks)
1. ✅ **Implement high-priority scenarios** (Lighting, Energy, TV)
2. ✅ **Validate against Node-RED** behavior
3. ✅ **Document testing patterns**
4. ✅ **Integrate into CI pipeline**

### Long Term (Next Month)
1. ✅ **Add Sleep Hygiene scenarios**
2. ✅ **Add multi-plugin integration tests**
3. ✅ **Implement remaining plugin scenario tests**
4. ✅ **Use scenario tests to validate Node-RED cutover**

---

## Questions & Answers

**Q: Won't this slow down test execution?**
A: Scenario tests run in parallel with infrastructure tests. Each scenario test takes ~500ms. 20 tests = ~10 seconds total, acceptable for comprehensive validation.

**Q: How do we maintain test configs (lighting_config.yaml, etc.)?**
A: Create minimal test configs in `test/integration/testdata/` with just enough data to validate behavior. Keep them small and focused.

**Q: What if automation logic changes?**
A: Update scenario tests to match. Tests document expected behavior, so when behavior changes intentionally, tests should update too.

**Q: How do we test time-based triggers (e.g., alarm time)?**
A: Mock server can simulate time changes, or we can trigger state changes directly. For alarm time, we update `alarmTime` state and verify wake sequence triggers.

**Q: Should we test error cases?**
A: Yes! Add scenarios for:
- HA service call failures
- Missing config entries
- Invalid state values
- Network timeouts

---

## Conclusion

**The Gap:** Current integration tests validate infrastructure but not automation logic.

**The Solution:** Add scenario-based tests that simulate real-world events and verify correct automation responses.

**The Impact:**
- ✅ Catch business logic bugs before production
- ✅ Validate Go implementation matches Node-RED
- ✅ Document expected behavior through tests
- ✅ Enable confident refactoring and migration

**Recommendation:** Proceed with implementing scenario-based tests starting with high-priority scenarios (Lighting, Energy, TV monitoring).

---

**Last Updated:** 2025-11-16
**Status:** Proposal - Awaiting Approval
**Effort Estimate:** 10-15 hours for complete implementation
**Priority:** HIGH - Critical for Node-RED migration validation
