# State Tracking and Configuration Tabs - Comprehensive Analysis

## Tab Overview

### State Tracking Tab
- **ID**: `d7a3510d.e93d98`
- **Purpose**: Monitors and manages presence, sleep states, and derived state variables
- **Key Responsibilities**: Real-time tracking of people's location and sleep status
- **Data Flow**: Receives HomeKit inputs → Processes logic → Updates shared state → Outputs to other systems

### Configuration Tab
- **ID**: `634c78c80eb9f37e`
- **Purpose**: Loads and manages configuration files; tracks time-based day phases
- **Key Responsibilities**: Load external configs and sun event tracking
- **Data Flow**: File reads → YAML parsing → Configuration storage → Time-based phase logic

---

## DETAILED: STATE TRACKING TAB

### Purpose & Scope
The State Tracking tab is the **operational heart** of the home automation system. It:
1. **Tracks presence**: Who is home (Nick, Caroline, anyone, etc.)
2. **Detects sleep**: Who is asleep (masters, guests, everyone)
3. **Monitors doors**: Master/guest bedroom door states
4. **Derives computed states**: Combines inputs to create higher-level states
5. **Syncs with HomeKit**: Exposes sleep states via HomeKit switches for manual control

### Major Functional Areas

#### 1. PRESENCE TRACKING (Lines 1465-1560)
**Input Sources:**
- HomeKit switches: "Nick Home", "Caroline Home" (manually set via HomeKit)
- Home Assistant entities: `input_boolean.nick_home`, `input_boolean.caroline_home`

**State Variables:**
- `isNickHome` (bool): Whether Nick is home
- `isCarolineHome` (bool): Whether Caroline is home
- `isAnyoneHome` (bool): Computed - true if Nick OR Caroline is home

**Flow Logic:**
```
HomeKit Input → set-shared-state → get-shared-state →
Are either of us home? (function) → isAnyoneHome (set-shared-state)
```

**Key Pattern:**
- Uses `get-shared-state` nodes with `triggerOnInit: true` and `triggerOnChange: true`
- Triggers on both initialization and any state change
- JavaScript function evaluates: `isNickHome.value || isCarolineHome.value`

#### 2. SLEEP TRACKING (Lines 1607-2024)
**Sleep States Tracked:**
- `isMasterAsleep` (bool): Masters (Nick/Caroline) asleep
- `isGuestAsleep` (bool): Guest bedroom occupant asleep
- `isEveryoneAsleep` (bool): Computed - true if everyone asleep

**Input Mechanism:**
- HomeKit switches expose these states for manual toggling
- Can be set via Apple Home app for manual override

**Data Flow Example (Masters Asleep):**
```
HomeKit "Masters Asleep" Switch
    ↓
Move On → value (change node)
    ↓
set-shared-state: "Master Asleep"
    ↓
ALSO: get-shared-state → Move value to On →
       HomeKit (bidirectional sync)
```

**Bidirectional Sync Pattern:**
- `get-shared-state` (line 1724): Reads stored state
- `change` node: Transforms value to HomeKit format (payload.On)
- `homekit-service`: Updates HomeKit accessory

#### 3. SLEEP DETECTION AUTO-LOGIC (Lines 2224-2290)
**Sleep Detection System:**
- Automatically detects when guest falls asleep
- Validates multiple conditions:
  1. Anyone is home
  2. Guest not already marked asleep
  3. Guests are present (`isHaveGuests`)
  4. Guest bedroom door is closed

**Function Node Logic:**
```javascript
// Applicability Checks - including door closed
if(global.get("state").isAnyoneHome.value == false) return null
if(global.get("state").isGuestAsleep.value) return null
if(global.get("state").isHaveGuests.value == false) return null
if(global.get("state").isGuestBedroomDoorOpen.value == false) return null
return msg  // Pass through - all conditions met
```

#### 4. DOOR TRACKING (Lines 1948-2078)
**Door States Tracked:**
- `isMasterBedroomDoorOpen` (bool): Master bedroom door state
- `isGuestBedroomDoorOpen` (bool): Guest bedroom door state

**Input Source:**
- HomeKit service: "Guest Bedroom Door State" (line 394af9bb2a153a77) [DISABLED]
- HomeKit service: door sensors

**Purpose:**
- Used in sleep detection logic (only mark guest asleep if door closed)
- Indicates occupancy/activity

#### 5. GUESTS PRESENCE (Lines 2082-2222)
**State Variable:**
- `isHaveGuests` (bool): Whether guests are currently present

**Tracking Logic:**
- HomeKit switch "Have Guests" allows manual control
- Bidirectional sync with HomeKit for visibility

### State Variables Managed by State Tracking Tab

| Variable | Type | Source | Used For |
|----------|------|--------|----------|
| isNickHome | bool | HomeKit + HA | Presence detection |
| isCarolineHome | bool | HomeKit + HA | Presence detection |
| isAnyoneHome | bool | Computed | Global state |
| isMasterAsleep | bool | HomeKit + Auto-detect | Sleep tracking |
| isGuestAsleep | bool | HomeKit + Auto-detect | Sleep tracking |
| isEveryoneAsleep | bool | Computed | Global state |
| isMasterBedroomDoorOpen | bool | HomeKit sensor | Sleep detection validation |
| isGuestBedroomDoorOpen | bool | HomeKit sensor | Sleep detection validation |
| isHaveGuests | bool | HomeKit | Guest presence |

### Key Patterns & Behaviors

**Pattern 1: HomeKit Bidirectional Sync**
```
State Variable
  ↓
get-shared-state (triggers on change)
  ↓
change node (transform to HomeKit format)
  ↓
homekit-service (updates HomeKit)
  ↓
User toggles in HomeKit
  ↓
homekit-service (output)
  ↓
set-shared-state (updates Node-RED state)
```

**Pattern 2: Derived State Computation**
```
Input State 1
  ↓
get-shared-state ──┐
                   ├→ function node ──→ set-shared-state (Result)
Input State 2      │
  ↓                │
get-shared-state ──┘
```

**Pattern 3: Self-Triggering on Init**
- All get-shared-state nodes have `triggerOnInit: true`
- Ensures flow runs on Node-RED startup
- Initializes computed states immediately

---

## DETAILED: CONFIGURATION TAB

### Purpose & Scope
The Configuration tab is a **one-time loader** for system configurations and a **continuous tracker** of day phases. It:
1. **Loads external YAML configs**: Music, Hue lights, schedules
2. **Tracks sun events**: Sunrise, sunset, twilight, night
3. **Computes day phases**: morning, day, sunset, dusk, winddown, night
4. **Provides manual overrides**: Allow testing/adjustment of day phases
5. **Manages resets**: System reset capability via HomeKit

### Major Functional Areas

#### 1. CONFIG FILE LOADING (Lines 3978-4114)
**Trigger:** Cron job at 00:01 daily + once on startup (line 6b334e0185781b32)

**Config Files Loaded:**
1. **music_config.yaml** (line a78b91dcc37bee92)
   - Path: `/data/projects/NickNodeRed/configs/music_config.yaml`
   - Content: Music playlists, speakers, volume settings
   - Processing: File → YAML parser → JSON → Function (volume_multiplier conversion)
   - Output: `musicConfig` state variable (obj)

2. **hue_config.yaml** (line 8409a48850c82e37)
   - Path: `/data/projects/NickNodeRed/configs/hue_config.yaml`
   - Content: Philips Hue lighting configurations
   - Processing: File → YAML parser → JSON
   - Output: `hueConfig` state variable (obj)

3. **schedule_config.yaml** (line 03d49e6e6a6a882d)
   - Path: `/data/projects/NickNodeRed/configs/schedule_config.yaml`
   - Content: Daily schedules (wake, dusk, winddown, bed, night times)
   - Processing: File → YAML → Function (parse times for current day) → JSON
   - Output: `schedule` state variable (obj)

**Data Processing Pipeline:**
```
file in node
    ↓
yaml node (UTF-8 text → JSON object)
    ↓
function node (transform if needed)
    ↓
set-shared-state (store in Node-RED state)
```

**Key Function: Parse Schedule Times (line e9dd15aabc896de3)**
- Gets current day of week
- Parses all schedule times from config
- Converts to timestamp format (milliseconds since epoch)
- Returns object: { begin_wake, wake, dusk, winddown, stop_screens, go_to_bed, night }

**Key Function: Volume Multiplier Conversion (line 086ceaf287af18e9)**
- Recursively traverses music config object
- Converts `volume_multiplier` values to float strings
- Example: 0.8 → "0.80"

#### 2. SUN EVENT TRACKING (Lines 4240-4316)
**Injector: Coordinates Injection (line 38a0f923f2d7e97e)**
- Interval: Every 6 hours (21600 seconds)
- Payload: GPS coordinates (32.85486, -97.50515) - Austin, TX area
- Runs once on startup

**Sun Event Processor (line 60f3d3f665e7ea46)**
- Type: "sun events" node (custom Node-RED node)
- Input: Coordinates
- Output: Detailed sun events (sunrise, sunset, twilight, golden hour, etc.)

**Sun State Summarizer Function (line f826979d9173e4f6)**
- Input: Detailed sunevent from sun tracker
- Output: Simplified state: morning, day, sunset, dusk, night

**Mapping Logic:**
```
goldenHour, sunsetStart, sunset  → "sunset"
dusk, nauticalDusk               → "dusk"
night, nightEnd, nauticalDawn, dawn, nadir → "night"
sunrise, sunriseEnd              → "morning"
goldenHourEnd                    → "day" (with delayed output)
else                             → "day"
```

**Output:**
- Primary: `sunevent` state variable (str) - immediate value
- Secondary (delayed): Same to second output (for delayed propagation)

#### 3. DAY PHASE CALCULATION (Lines 4507-4610)
**Purpose:** Determine current "scene" based on sun events AND schedule overrides

**Trigger Sources:**
1. `get-shared-state` for `sunevent` (line b681b75836dbf49a)
   - `triggerOnInit: true`, `triggerOnChange: true`
2. Cron: Every 30 minutes during evening (4 AM - 11 PM) (line 14c93c9256bae5c4)

**Phase Decider Function (line 6acc06089b237d56)**
```javascript
var sun_event = global.get("state").sunevent.value
var now = new Date()
var this_hour = now.getHours()

// Get override times from schedule
var dusk_time = new Date(global.get("state").schedule.value.dusk)
var winddown_time = new Date(global.get("state").schedule.value.winddown)
var night_time = new Date(global.get("state").schedule.value.night)

// Logic:
// - morning: stay until noon, then switch to day
// - day: pass through
// - sunset: pass through
// - dusk: pass through
// - night: if after night_time OR before 6 AM → night, else → winddown
```

**Output:** `dayPhase` state variable (str)

**Valid Values:**
- "morning" - Sunrise to noon
- "day" - Noon until sunset
- "sunset" - Golden hour through sunset
- "dusk" - After sunset twilight
- "winddown" - Evening transition (if not yet "night" time)
- "night" - Late evening and night

#### 4. MANUAL DAY PHASE OVERRIDES (Lines 4115-4280)
**Purpose:** Allow testing and manual override of day phase logic

**Inject Nodes (all point to Phase Decider):**
- "Manually set to Morn" → payload: "morning"
- "Manually set to Day" → payload: "day"
- "Manually set to Sunset" → payload: "sunset"
- "Manually set to Dusk" → payload: "dusk"
- "Manually set to Winddown" → payload: "winddown"
- "Manually set to Night" → payload: "night"

**Data Flow:**
```
Manual Inject → Phase Decider → set-shared-state: dayPhase
```

#### 5. RESET SYSTEM (Lines 5c380b83edfd85d6 - 4452)
**Purpose:** System reset capability for testing/recovery

**Reset Sources:**
1. HomeKit "Reset" switch (line 1b9873d4c1d781ca)
   - Accessible via HomeKit app
   - Type: Switch service

**Reset Sequence:**
```
HomeKit Reset Switch (On)
    ↓
change node: Move On → value
    ↓
delay: 1 second
    ↓
Split: Two paths
    ├→ Path 1: delay (5 sec rate limit)
    │           ↓
    │       set-shared-state: Reset
    └→ Path 2: change (Turn off)
                ↓
             homekit-service (update HomeKit to Off)
```

**Purpose of Delay:**
- Prevents multiple resets in quick succession
- Rate-limited to 1 per 5 seconds

#### 6. OUTDOOR TEMPERATURE TRACKING (Lines 4743-4799)
**Purpose:** Monitor external weather conditions

**Sensor:**
- Home Assistant entity: `sensor.weather_station_temperature`
- Type: Number
- Output on change

**Processing:**
- Listens for state changes
- Updates when temperature changes
- Used by other systems for decisions

### State Variables Managed by Configuration Tab

| Variable | Type | Source | Frequency |
|----------|------|--------|-----------|
| musicConfig | obj | YAML file | Daily at 00:01 + startup |
| hueConfig | obj | YAML file | Daily at 00:01 + startup |
| schedule | obj | YAML file + parse | Daily at 00:01 + startup |
| sunevent | str | Sun tracker | Every 6 hours |
| dayPhase | str | Phase decider | On sun change + every 30 min evening |
| reset | num | HomeKit | Manual trigger |

### Key Configuration File Structures

**schedule_config.yaml example:**
```yaml
schedule:
  - begin_wake: "05:00"
    wake: "07:00"
    dusk: "18:00"
    winddown: "21:00"
    stop_screens: "22:00"
    go_to_bed: "22:30"
    night: "23:00"
  # ... entries for each day of week (0-6)
```

**music_config.yaml / hue_config.yaml:**
- Complex nested JSON structures
- Device/room specific settings
- Volume multipliers for music
- Color and scene settings for Hue

---

## INTERACTIONS & DEPENDENCIES

### State Tracking → Configuration
- **Dependency**: None (independent)
- **Consumes**: None from Configuration tab

### Configuration → State Tracking
- **Dependency**: None (independent)
- **Consumes**: None from State Tracking tab

### State Tracking → Other Tabs
- **Provides to Music Control**:
  - `isAnyoneHome`, `isEveryoneAsleep` (prevent music when sleeping/nobody home)
  - `isMasterAsleep`, `isGuestAsleep` (sleep-specific music behavior)

- **Provides to Lighting**:
  - `isAnyoneHome` (turn on/off lights based on presence)
  - `isMasterAsleep`, `isEveryoneAsleep` (night lighting modes)
  - `isMasterBedroomDoorOpen` (automatic lighting)

### Configuration → Other Tabs
- **Provides to Music Control**:
  - `musicConfig` (speaker lists, playlists, volume settings)

- **Provides to Lighting Control**:
  - `hueConfig` (light IDs, group configurations, scene settings)
  - `dayPhase` (determines which lighting scene to apply)

- **Provides to Automation**:
  - `schedule` (timing for automated actions)
  - `dayPhase` (context for what automation should run)

---

## CRITICAL PATTERNS FOR GOLANG IMPLEMENTATION

### 1. State Derivation Pattern
**Pattern:**
```
Multiple Input States → Function Logic → Derived State
```

**Examples:**
- `isAnyoneHome = isNickHome OR isCarolineHome`
- `isEveryoneAsleep = isMasterAsleep AND isGuestAsleep`
- `dayPhase = logic(sunevent, current_time, schedule)`

**Implementation Note:**
- These are computed states, not stored states
- Should be recalculated whenever inputs change
- Need subscription/observer pattern

### 2. HomeKit Bidirectional Sync
**Pattern:**
```
Node-RED State ↔ HomeKit Accessory
```

**Implementation Note:**
- Home Assistant can expose entities to HomeKit via HomeKit Bridge
- In Golang, won't directly interface with HomeKit
- Will expose states via Home Assistant entities
- HomeKit sync via HA's HomeKit integration

### 3. Configuration Loading
**Pattern:**
```
External File (YAML/JSON) → Parser → State Object → Subscribers Notified
```

**Implementation Note:**
- Needs file watcher or scheduled reload
- YAML parsing needed (Go has yaml libraries)
- Store as complex objects in state

### 4. Time-Based Phase Logic
**Pattern:**
```
Current Time + Sun Event + Schedule → Phase Decision
```

**Implementation Note:**
- Requires time zone awareness
- Schedule is per-day (updates daily)
- Sun events calculated based on GPS coordinates and date
- Fallback to manual overrides

### 5. State Subscription Triggers
**Pattern:**
```
State Change → Immediate Trigger (triggerOnChange: true)
State Init → Startup Trigger (triggerOnInit: true)
```

**Implementation Note:**
- Callbacks must be registered for state changes
- Some callbacks should fire on startup
- Need to handle initialization state properly
- Multiple subscribers per state

---

## MIGRATION CONSIDERATIONS FOR GOLANG

### What Needs to be Replicated

1. **State Tracking Tab Functions:**
   - ✅ Presence logic: `isAnyoneHome = isNickHome OR isCarolineHome`
   - ✅ Sleep logic: `isEveryoneAsleep = isMasterAsleep AND isGuestAsleep`
   - ✅ Auto-sleep detection: Check door closed + guest present + nobody asleep
   - ✅ Bidirectional HomeKit sync via Home Assistant entities
   - ⚠️ HomeKit direct integration (may use Home Assistant bridge instead)

2. **Configuration Tab Functions:**
   - ✅ Load YAML configs from files (schedule, music, hue)
   - ✅ Parse schedule times for current day
   - ✅ Sun event calculation (use external library)
   - ✅ Sun event simplification logic
   - ✅ Day phase decision logic
   - ✅ Manual phase override capability
   - ✅ System reset trigger handling
   - ✅ Temperature monitoring

### What Doesn't Need Direct Replication

1. **HomeKit node integration** - Use Home Assistant HomeKit bridge
2. **YAML file nodes** - Direct file I/O in Go
3. **Visual flow editor** - Implement as REST API + configured state

### Implementation Strategy

**Phase 1: State Tracking**
- Implement state variables (already done in homeautomation-go)
- Add helper functions for computed states
- Subscribe Home Assistant entities to state changes
- Export state via Home Assistant REST API

**Phase 2: Configuration**
- Add configuration file loading
- Implement YAML parsing
- Add schedule time parsing
- Integrate sun event library

**Phase 3: Automation**
- Implement day phase logic as helper function
- Add subscription handlers for state changes
- Trigger state updates on Home Assistant entity changes

---

## SUMMARY TABLE

| Aspect | State Tracking | Configuration |
|--------|---|---|
| **Primary Purpose** | Track presence & sleep | Load configs & calculate day phase |
| **Update Frequency** | Event-driven (state changes) | Time-based (daily or periodic) |
| **Data Sources** | HomeKit, Home Assistant | Files, Sun events, Time |
| **Data Sinks** | Home Assistant entities | State variables |
| **Complexity** | Medium (logic + sync) | Medium (parsing + calculation) |
| **Dependencies** | HomeKit Bridge, HA entities | File system, Sun event lib |
| **Critical States** | 9 variables | 6 variables |
| **User Interaction** | HomeKit switches | HomeKit switches + files |

