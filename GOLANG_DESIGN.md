# Golang Home Automation System - Design Document

**Version:** 1.0
**Date:** 2025-11-15
**Status:** Design Phase

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [System Architecture](#system-architecture)
3. [Core Components](#core-components)
4. [Automation Plugins](#automation-plugins)
5. [Data Flow](#data-flow)
6. [State Synchronization](#state-synchronization)
7. [Configuration Management](#configuration-management)
8. [Home Assistant Integration](#home-assistant-integration)
9. [Deployment Architecture](#deployment-architecture)
10. [Migration Strategy](#migration-strategy)
11. [Implementation Roadmap](#implementation-roadmap)

---

## Executive Summary

### Purpose

This document describes the architecture for a Golang-based home automation system that will replace the existing Node-RED implementation. The new system will maintain exact functional parity with the current Node-RED flows while providing better performance, maintainability, and type safety.

### Goals

1. **1:1 Functional Migration** - Replicate all active Node-RED flows exactly as they currently behave
2. **Home Assistant as State Store** - Use HA input helpers (33 variables) as the persistent data store
3. **Modular Architecture** - Plugin-based design allows independent development and testing of each automation domain
4. **Configuration Compatibility** - Reuse existing YAML configuration files without modification
5. **Seamless Transition** - Run in parallel with Node-RED during migration, allowing gradual cutover

### Non-Goals (Future Work)

- Redesigning automation logic or improving behaviors
- Creating new automations beyond what exists in Node-RED
- Replacing YAML configs with a new format
- Direct device integration (all done through Home Assistant)

---

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Home Automation System (Golang)               │
│                                                                   │
│  ┌──────────────┐  ┌─────────────────────────────────────────┐  │
│  │    Main      │  │         Plugin Manager                   │  │
│  │  Application │  │                                          │  │
│  │              │  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ │  │
│  │  - Bootstrap │  │  │  Music   │ │ Lighting │ │ Security │ │  │
│  │  - Lifecycle │  │  │  Plugin  │ │  Plugin  │ │  Plugin  │ │  │
│  │  - Logging   │  │  └──────────┘ └──────────┘ └──────────┘ │  │
│  │              │  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ │  │
│  └──────┬───────┘  │  │  Sleep   │ │  Energy  │ │    TV    │ │  │
│         │          │  │  Plugin  │ │  Plugin  │ │  Plugin  │ │  │
│         │          │  └──────────┘ └──────────┘ └──────────┘ │  │
│  ┌──────▼───────┐  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ │  │
│  │    Core      │  │  │  State   │ │   Load   │ │ Calendar │ │  │
│  │  Components  │  │  │ Tracking │ │ Shedding │ │  Plugin  │ │  │
│  │              │  │  └──────────┘ └──────────┘ └──────────┘ │  │
│  │ - State Mgr  │  │  ┌──────────┐                            │  │
│  │ - Config Ldr │  │  │ Nagging  │                            │  │
│  │ - Event Bus  │  │  │  Plugin  │                            │  │
│  │ - HA Client  │  │  └──────────┘                            │  │
│  └──────┬───────┘  └─────────────────────────────────────────┘  │
│         │                                                         │
└─────────┼─────────────────────────────────────────────────────────┘
          │
          │ WebSocket
          │
┌─────────▼─────────────────────────────────────────────────────────┐
│                      Home Assistant                                │
│                                                                     │
│  ┌──────────────────┐  ┌──────────────┐  ┌───────────────────┐   │
│  │  Input Helpers   │  │   Devices    │  │   Services        │   │
│  │  (33 variables)  │  │              │  │                   │   │
│  │                  │  │ - Sonos      │  │ - call_service    │   │
│  │ - Booleans (18)  │  │ - Hue        │  │ - set_value       │   │
│  │ - Numbers (3)    │  │ - Apple TV   │  │ - turn_on/off     │   │
│  │ - Text (6)       │  │ - Bravia TV  │  │ - media_player.*  │   │
│  │ - JSON (6)       │  │ - Lutron     │  │                   │   │
│  │                  │  │ - Roborock   │  │                   │   │
│  └──────────────────┘  │ - ratgdo     │  └───────────────────┘   │
│                        │ - Thermostat │                           │
│                        └──────────────┘                           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Architecture Principles

1. **Plugin-Based Monolith**
   - Single compiled binary for simplified deployment
   - Each automation domain is a separate plugin
   - Plugins are loaded at startup and communicate via an event bus
   - Hot-reload of configuration without service restart

2. **Event-Driven Design**
   - All state changes trigger events on an internal event bus
   - Plugins subscribe to relevant events
   - Loose coupling between plugins

3. **Home Assistant as Source of Truth**
   - All persistent state stored in HA input helpers
   - Golang system maintains in-memory cache for performance
   - Bidirectional synchronization ensures consistency

4. **Configuration as Code**
   - YAML configuration files define behavior
   - No hardcoded logic in business rules
   - Git-tracked configuration for version control

---

## Core Components

### 1. State Manager

**Responsibility:** Manages the 33 synchronized state variables and provides thread-safe access.

**Key Features:**
- In-memory cache of all 33 HA input helpers
- Thread-safe read/write operations using sync.RWMutex
- Automatic synchronization with Home Assistant
- Event emission on state changes
- Support for atomic compare-and-swap operations

**Interface:**
```go
type StateManager interface {
    // Get/Set primitives
    GetBool(key string) (bool, error)
    SetBool(key string, value bool) error
    GetString(key string) (string, error)
    SetString(key string, value string) error
    GetNumber(key string) (float64, error)
    SetNumber(key string, value float64) error
    GetJSON(key string, target interface{}) error
    SetJSON(key string, value interface{}) error

    // Atomic operations
    CompareAndSwapBool(key string, old, new bool) (bool, error)

    // Subscribe to state changes
    Subscribe(key string, handler StateChangeHandler) Subscription

    // Sync with Home Assistant
    SyncFromHA() error
    SyncToHA(key string, value interface{}) error
}
```

**State Variables (33 total):**
- See `migration_mapping.md` for complete list
- Boolean: 18 variables (presence, sleep states, device states)
- Number: 3 variables (alarm time, solar generation)
- Text: 6 variables (day phase, energy levels, music mode)
- JSON: 6 variables (configs and current state objects)

### 2. Config Loader

**Responsibility:** Loads and validates YAML configuration files.

**Supported Configs:**
- `music_config.yaml` - Music playback modes and playlists
- `hue_config.yaml` - Lighting scenes and room mappings
- `schedule_config.yaml` - Time-based schedules
- `energy_config.yaml` - Energy level thresholds and behaviors

**Features:**
- Hot-reload capability with file watching
- YAML validation on load
- Structured Go types for type safety
- Event emission on config changes

**Interface:**
```go
type ConfigLoader interface {
    Load(configPath string) error
    GetMusicConfig() (*MusicConfig, error)
    GetHueConfig() (*HueConfig, error)
    GetScheduleConfig() (*ScheduleConfig, error)
    GetEnergyConfig() (*EnergyConfig, error)

    // Watch for changes and reload
    Watch() error
    OnConfigChange(handler ConfigChangeHandler) Subscription
}
```

### 3. Event Bus

**Responsibility:** Internal publish-subscribe system for inter-plugin communication.

**Event Types:**
- State change events (e.g., `state.isNickHome.changed`)
- Device events (e.g., `device.apple_tv.playing`)
- Time events (e.g., `time.sun.sunset`, `time.schedule.morning`)
- Config change events (e.g., `config.music.reloaded`)
- Command events (e.g., `command.music.play`, `command.lights.activate_scene`)

**Interface:**
```go
type EventBus interface {
    Publish(topic string, data interface{})
    Subscribe(topic string, handler EventHandler) Subscription
    SubscribePattern(pattern string, handler EventHandler) Subscription
    Unsubscribe(sub Subscription)
}
```

### 4. Home Assistant Client

**Responsibility:** Manages communication with Home Assistant via WebSocket.

**Features:**
- Connection management with auto-reconnect
- Entity state queries
- Service call execution
- Event subscription
- Rate limiting and retry logic

**Interface:**
```go
type HAClient interface {
    // Connection
    Connect() error
    Disconnect() error
    IsConnected() bool

    // State operations
    GetState(entityID string) (*State, error)
    SetState(entityID string, state interface{}) error

    // Service calls
    CallService(domain, service string, data map[string]interface{}) error

    // Event subscription
    SubscribeEvents(eventType string, handler EventHandler) Subscription
    SubscribeStateChanges(entityID string, handler StateChangeHandler) Subscription

    // Input helper operations (convenience methods)
    SetInputBoolean(name string, value bool) error
    SetInputNumber(name string, value float64) error
    SetInputText(name string, value string) error
}
```

### 5. Plugin Manager

**Responsibility:** Loads, initializes, and manages lifecycle of automation plugins.

**Features:**
- Plugin discovery and loading
- Dependency injection (provides core components to plugins)
- Graceful startup and shutdown
- Health monitoring

**Interface:**
```go
type PluginManager interface {
    RegisterPlugin(plugin Plugin) error
    LoadPlugins() error
    StartPlugins() error
    StopPlugins() error
    GetPlugin(name string) (Plugin, error)
}

type Plugin interface {
    Name() string
    Initialize(ctx PluginContext) error
    Start() error
    Stop() error
    Health() HealthStatus
}

type PluginContext struct {
    StateManager StateManager
    ConfigLoader ConfigLoader
    EventBus     EventBus
    HAClient     HAClient
    Logger       Logger
}
```

---

## Automation Plugins

Each plugin corresponds to a Node-RED flow and implements domain-specific automation logic.

### 1. State Tracking Plugin

**Node-RED Flow:** State Tracking

**Responsibilities:**
- Track presence (Nick, Caroline, Tori home/away)
- Determine derived states (any owner home, anyone home)
- Track sleep states (master asleep, guest asleep, everyone asleep)
- Monitor door states (guest bedroom door, office occupancy)
- Announce arrivals/departures via TTS

**Key Automations:**
- **Presence Detection**: Listen to HA presence triggers → Update `isNickHome`, `isCarolineHome`, `isToriHere`
- **Derived Presence**: Calculate `isAnyOwnerHome`, `isAnyoneHome` from individual states
- **Sleep Detection**: Monitor bedroom lights/doors → Update sleep states
- **Door Tracking**: Subscribe to door sensor events → Update door states
- **Arrival Notifications**: On owner arrival → Check if anyone else home → Announce via TTS

**Events Consumed:**
- `ha.binary_sensor.nick_home.changed`
- `ha.binary_sensor.caroline_home.changed`
- `ha.binary_sensor.tori_here.changed`
- `ha.light.master_bedroom.*.changed`
- `ha.binary_sensor.guest_bedroom_door.changed`

**Events Published:**
- `state.presence.changed`
- `state.sleep.changed`
- `event.arrival` / `event.departure`

### 2. Lighting Control Plugin

**Node-RED Flow:** Lighting Control

**Responsibilities:**
- Activate lighting scenes based on day phase
- Respond to sun events (sunrise, sunset, dusk)
- Handle manual overrides
- Manage Christmas tree seasonal lighting
- Control brightness levels by time of day

**Key Automations:**
- **Sun Event Scenes**: On sun event change → Determine active rooms → Activate appropriate scene
- **Day Phase Scenes**: When `dayPhase` changes → Apply scene to each room
- **Occupancy-Based**: Only activate in rooms where people are present
- **TV Brightness**: Dim TV area when TV playing
- **Christmas Tree**: Seasonal control with brightness by time of day

**Events Consumed:**
- `state.dayPhase.changed`
- `state.sunevent.changed`
- `state.isAnyoneHome.changed`
- `state.isTVPlaying.changed`
- `config.hue.reloaded`

**Events Published:**
- `command.lights.activate_scene`
- `event.scene_activated`

**Config File:** `hue_config.yaml`

### 3. Music Management Plugin

**Node-RED Flow:** Music

**Responsibilities:**
- Manage Sonos speaker groups and playback
- Select appropriate music mode based on context
- Handle volume management with fade in/out
- Coordinate multi-room audio
- Prevent playback when inappropriate (sleep, away)

**Key Automations:**
- **Mode Selection**: Based on `dayPhase`, presence, sleep state → Determine music mode
- **Playback Control**: Mode change → Build participant groups → Set volumes → Start playback
- **Volume Management**: Gradual volume increases/decreases, mute logic for sleeping areas
- **Shutdown on Exit**: Everyone leaves → Stop all playback
- **Tori Arrival**: Increase volume when Tori present
- **Coordinated Playback**: Ensure speaker groups synchronized

**Events Consumed:**
- `state.dayPhase.changed`
- `state.isAnyoneHome.changed`
- `state.isMasterAsleep.changed`
- `state.isGuestAsleep.changed`
- `state.isToriHere.changed`
- `state.isTVPlaying.changed`
- `config.music.reloaded`

**Events Published:**
- `command.music.play`
- `command.music.stop`
- `command.music.set_volume`
- `state.musicPlaybackType.changed`

**Config File:** `music_config.yaml`

**Complex Logic:**
- Music mode decision tree (see Node-RED function: "Pick Appropriate Music")
- Volume calculation with multipliers
- Playlist selection from Spotify URIs

### 4. Sleep Hygiene Plugin

**Node-RED Flow:** Sleep Hygiene

**Responsibilities:**
- Fade out sleep sounds in the morning
- Trigger wake-up sequences
- Coordinate with lighting for gentle wake
- Handle cuddle notifications
- Manage rain sound playback

**Key Automations:**
- **Wake Detection**: Morning time + master occupied → Begin fade out
- **Fade Out**: Gradually reduce volume → Turn on bedroom lights → Switch to day music
- **Cuddle Notification**: Check if owners can cuddle → TTS announcement
- **Schedule-Based**: Read wakeup time from schedule config → Trigger at appropriate time
- **Light Coordination**: Ensure lights turn on slowly during wake sequence

**Events Consumed:**
- `state.dayPhase.changed`
- `state.isMasterAsleep.changed`
- `state.alarmTime.changed`
- `time.schedule.wakeup`
- `config.schedule.reloaded`

**Events Published:**
- `command.music.fadeout`
- `command.lights.wakeup_sequence`
- `event.wakeup_started`

**Config File:** `schedule_config.yaml`

### 5. Energy State Plugin

**Node-RED Flow:** Energy State

**Responsibilities:**
- Calculate current energy availability level
- Track solar generation (current hour, remaining day)
- Monitor battery state
- Determine if free energy available (grid)
- Publish energy level for other automations

**Key Automations:**
- **Battery Level**: HA sensor → Convert to energy level enum → Update `batteryEnergyLevel`
- **Solar Calculation**: Solar forecast → Calculate remaining generation → Update variables
- **Overall Level**: Combine battery + solar + grid → Determine `currentEnergyLevel`
- **Free Energy Detection**: Grid state → Update `isFreeEnergyAvailable`
- **Lighting Hints**: Set light colors based on energy state

**Events Consumed:**
- `ha.sensor.battery_percentage.changed`
- `ha.sensor.solar_generation.changed`
- `ha.binary_sensor.grid_available.changed`
- `config.energy.reloaded`

**Events Published:**
- `state.energyLevel.changed`
- `state.freeEnergy.changed`

**Config File:** `energy_config.yaml`

### 6. Load Shedding Plugin

**Node-RED Flow:** Load Shedding

**Responsibilities:**
- Adjust thermostat settings based on energy state
- Widen temperature ranges when energy is scarce
- Restore comfort settings when energy is plentiful
- Emergency load shedding in critical situations

**Key Automations:**
- **Energy State Red/Black**: Energy critical → Set thermostat to hold mode + widen range
- **Energy State Green/White**: Energy abundant → Restore schedule mode + comfort range
- **Rate Limiting**: Prevent rapid thermostat changes

**Events Consumed:**
- `state.currentEnergyLevel.changed`

**Events Published:**
- `command.thermostat.set_mode`
- `command.thermostat.set_range`

### 7. Security Plugin

**Node-RED Flow:** Security

**Responsibilities:**
- Automatic lockdown when everyone asleep or away
- Garage door automation on arrival
- Doorbell notifications
- "Expecting someone" mode

**Key Automations:**
- **Lockdown**: Everyone asleep + no one home → Activate lockdown (lock doors, etc.)
- **Lockdown Release**: Owner returns → Reset lockdown switch
- **Garage Open on Return**: Owner arrives + garage empty → Open garage door
- **Doorbell**: Doorbell pressed → Rate-limited TTS announcement + light flash
- **Expecting Someone**: Manual toggle → Vehicle arrives → Notify via TTS → Reset flag

**Events Consumed:**
- `state.isEveryoneAsleep.changed`
- `state.isAnyoneHome.changed`
- `state.isExpectingSomeone.changed`
- `ha.binary_sensor.doorbell.pressed`
- `ha.binary_sensor.vehicle_arriving.detected`
- `event.arrival`

**Events Published:**
- `command.lock.engage`
- `command.garage.open`
- `command.notify.doorbell`

### 8. TV Monitoring Plugin

**Node-RED Flow:** TV Monitoring and Manipulation

**Responsibilities:**
- Detect when TV or Apple TV is playing
- Control soundbar input selection
- Adjust TV brightness by time of day
- Manage sync box state

**Key Automations:**
- **Playback Detection**: Monitor Apple TV state → Update `isAppleTVPlaying` and `isTVPlaying`
- **TV State Tracking**: Sync box sensors → Determine `isTVOn`
- **Soundbar Control**: TV on → Force soundbar to correct input (repeat to override bad behavior)
- **Brightness Adjustment**: Day phase change → Set TV brightness level
- **Sync Control**: Turn sync box on/off with TV

**Events Consumed:**
- `ha.media_player.apple_tv.changed`
- `ha.sensor.hue_sync.changed`
- `state.dayPhase.changed`

**Events Published:**
- `state.isTVPlaying.changed`
- `state.isTVOn.changed`
- `command.tv.set_brightness`

### 9. Calendar Plugin

**Node-RED Flow:** Calendar

**Responsibilities:**
- Monitor work calendars for upcoming meetings
- Send morning notifications for today's schedule
- Context-aware notifications (only when home)

**Key Automations:**
- **Daily Check**: Morning trigger → Fetch calendar events → Check if meetings tomorrow → TTS notification
- **Presence Filter**: Only notify if person is home
- **Per-Person Calendars**: Separate handling for Nick and Caroline's calendars

**Events Consumed:**
- `state.isNickHome.changed`
- `state.isCarolineHome.changed`
- `time.schedule.morning`

**Events Published:**
- `command.notify.calendar`

### 10. Nagging Plugin

**Node-RED Flow:** Nagging

**Responsibilities:**
- Remind to close windows when rain is forecasted
- Other periodic reminders and notifications

**Key Automations:**
- **Rain Reminder**: Check forecast → Rain expected → Check if anyone home → TTS reminder every 12 hours
- **Context Aware**: Only nag when people are home and awake

**Events Consumed:**
- `state.isAnyoneHome.changed`
- `state.musicPlaybackType.changed` (to check if not sleeping)
- `ha.weather.forecast.changed`

**Events Published:**
- `command.notify.reminder`

---

## Data Flow

### Startup Sequence

```
1. Main Application Start
   ↓
2. Initialize Logger
   ↓
3. Load Configuration Files (YAML)
   ↓
4. Connect to Home Assistant (WebSocket)
   ↓
5. Initialize State Manager
   ↓
6. Sync State from HA (read all 33 input helpers)
   ↓
7. Initialize Event Bus
   ↓
8. Load and Initialize Plugins
   ↓
9. Start Plugins (begin event subscriptions)
   ↓
10. System Ready - Begin Event Processing
```

### State Change Propagation

```
┌──────────────────┐
│  Home Assistant  │
│  Input Helper    │
│  Value Changes   │
└────────┬─────────┘
         │
         │ WebSocket Event
         ↓
┌────────────────────┐
│   HA Client        │
│  Event Listener    │
└────────┬───────────┘
         │
         │ Internal Event
         ↓
┌────────────────────┐
│  State Manager     │
│  Update Cache      │
└────────┬───────────┘
         │
         │ Publish state.{key}.changed
         ↓
┌────────────────────┐
│   Event Bus        │
└────────┬───────────┘
         │
         │ Fan out to subscribers
         ↓
┌────────────────────┐
│  Plugins           │
│  Event Handlers    │
│  Business Logic    │
└────────┬───────────┘
         │
         │ Actions (call HA services, update state)
         ↓
┌────────────────────┐
│   HA Client        │
│  Service Calls     │
└────────┬───────────┘
         │
         │ API Calls
         ↓
┌────────────────────┐
│  Home Assistant    │
│  Execute Action    │
└────────────────────┘
```

### Example: Music Mode Change Flow

```
1. User Arrives Home
   → HA detects presence
   → HA updates input_boolean.nick_home = true

2. HA Client receives event
   → Updates State Manager cache
   → Publishes "state.isNickHome.changed" event

3. Music Plugin receives event
   → Evaluates music mode decision logic
   → Determines: daytime + someone home = "day" mode
   → Checks if mode different from current

4. Music Plugin publishes mode change
   → Updates state.musicPlaybackType = "day"
   → State Manager syncs to HA

5. Music Plugin executes playback
   → Reads music_config.yaml for "day" mode settings
   → Calls HA service: media_player.play_media
   → Calls HA service: media_player.join (for groups)
   → Calls HA service: media_player.volume_set

6. State Tracking Plugin receives event
   → Announces arrival via TTS if appropriate
```

---

## State Synchronization

### Synchronization Strategy

**Bidirectional Sync:**
- **HA → Golang**: Websocket events update in-memory cache immediately
- **Golang → HA**: All state changes written to HA input helpers via service calls

**Conflict Resolution:**
- Home Assistant is always the source of truth
- On startup, Golang loads all state from HA
- In case of sync failures, Golang retries with exponential backoff
- Periodic full sync (every 5 minutes) to detect and correct drift

**State Variable Mapping:**

See `migration_mapping.md` for complete mapping. Summary:

| Category | Count | Examples |
|----------|-------|----------|
| Presence/Sleep Booleans | 18 | isNickHome, isMasterAsleep, isAnyoneHome |
| Device State Booleans | - | isTVPlaying, isAppleTVPlaying |
| Energy Booleans | - | isFreeEnergyAvailable, isGridAvailable |
| Numbers | 3 | alarmTime, remainingSolarGeneration, thisHourSolarGeneration |
| Simple Text | 6 | dayPhase, sunevent, musicPlaybackType, energyLevels |
| JSON Objects | 6 | currentlyPlayingMusic, musicConfig, hueConfig, energyConfig, musicPlaylistNumbers, schedule |

### Synchronization Implementation

**On Startup:**
```go
func (sm *StateManager) SyncFromHA() error {
    // Read all 33 input helpers from HA
    for _, variable := range allVariables {
        value, err := sm.haClient.GetState(variable.EntityID)
        if err != nil {
            return err
        }
        sm.cache[variable.Key] = value
    }
    return nil
}
```

**On State Change (Golang → HA):**
```go
func (sm *StateManager) SetBool(key string, value bool) error {
    // Update cache
    sm.mutex.Lock()
    sm.cache[key] = value
    sm.mutex.Unlock()

    // Publish event
    sm.eventBus.Publish(fmt.Sprintf("state.%s.changed", key), value)

    // Sync to HA asynchronously
    go sm.syncToHA(key, value)

    return nil
}
```

**On State Change (HA → Golang):**
```go
func (sm *StateManager) handleHAEvent(event HAEvent) {
    key := sm.entityToKey(event.EntityID)

    // Update cache
    sm.mutex.Lock()
    oldValue := sm.cache[key]
    sm.cache[key] = event.NewState
    sm.mutex.Unlock()

    // Only publish if value actually changed
    if oldValue != event.NewState {
        sm.eventBus.Publish(fmt.Sprintf("state.%s.changed", key), event.NewState)
    }
}
```

---

## Configuration Management

### Configuration Files

The system reuses the existing YAML configuration files without modification:

**1. music_config.yaml**
- Defines music playback modes (morning, day, evening, winddown, sleep, airplay, sex, birds)
- Specifies Spotify playlist URIs for each mode
- Volume settings and participant groupings
- Mute logic for different scenarios

**2. hue_config.yaml**
- Maps rooms to Hue scene names
- Defines which rooms to activate for each day phase
- Brightness levels by time of day

**3. schedule_config.yaml**
- Daily schedule with time-based triggers
- Wakeup times and sleep times
- Day phase transition times

**4. energy_config.yaml**
- Energy level thresholds (white, green, yellow, orange, red, black)
- Battery percentage ranges for each level
- Solar generation expectations
- Load shedding behavior per level

### Configuration Structure

Each config is loaded into strongly-typed Go structs:

```go
type MusicConfig struct {
    Modes map[string]MusicMode `yaml:"modes"`
}

type MusicMode struct {
    URI              string              `yaml:"uri"`
    Coordinator      string              `yaml:"coordinator"`
    Participants     []string            `yaml:"participants"`
    Volumes          map[string]int      `yaml:"volumes"`
    VolumeMultiplier float64             `yaml:"volume_multiplier"`
    Shuffle          bool                `yaml:"shuffle"`
    Repeat           string              `yaml:"repeat"`
    MuteConditions   []MuteCondition     `yaml:"mute_conditions"`
}
```

### Hot Reload

- File watcher monitors config files for changes
- On change detected → Reload → Validate → Publish config change event
- Plugins receive event and update their behavior
- No service restart required

---

## Home Assistant Integration

### Connection Options

**Primary: WebSocket API (Recommended)**

WebSocket is the recommended approach:

**Advantages:**
- Native HA protocol with full support
- Real-time event streaming
- Bidirectional communication
- Built into Home Assistant, no additional setup
- Reliable reconnection handling

**Connection:**
```
ws://homeassistant.local:8123/api/websocket
Authorization: Bearer <long-lived-access-token>
```

**Event Subscription:**
```json
{
  "type": "subscribe_events",
  "event_type": "state_changed",
  "id": 1
}
```

**Service Calls:**
```json
{
  "type": "call_service",
  "domain": "input_boolean",
  "service": "turn_on",
  "service_data": {
    "entity_id": "input_boolean.nick_home"
  },
  "id": 2
}
```

### WebSocket Client Implementation

**Library:** `github.com/gorilla/websocket`

**Features Needed:**
- Auto-reconnect with exponential backoff
- Message queue during disconnections
- Subscription management
- Request/response correlation
- Heartbeat/ping-pong for connection health

**State Change Subscription:**
```go
// Subscribe to all input_boolean changes
client.SubscribeStateChanges("input_boolean.*", handler)

// Subscribe to specific entity
client.SubscribeStateChanges("input_boolean.nick_home", handler)
```

**Service Call Example:**
```go
// Turn on boolean
client.CallService("input_boolean", "turn_on", map[string]interface{}{
    "entity_id": "input_boolean.nick_home",
})

// Set number value
client.CallService("input_number", "set_value", map[string]interface{}{
    "entity_id": "input_number.alarm_time",
    "value":     1668524400000,
})

// Set text value
client.CallService("input_text", "set_value", map[string]interface{}{
    "entity_id": "input_text.day_phase",
    "value":     "morning",
})

// Media player control
client.CallService("media_player", "play_media", map[string]interface{}{
    "entity_id":    "media_player.living_room",
    "media_content_type": "playlist",
    "media_content_id": "spotify:playlist:37i9dQZF1DX...",
})
```

---

## Deployment Architecture

### Containerized Deployment (Recommended)

**Dockerfile:**
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /homeautomation cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /homeautomation /homeautomation
COPY configs/ /configs/
ENTRYPOINT ["/homeautomation"]
```

**Docker Compose:**
```yaml
version: '3.8'
services:
  homeautomation:
    build: .
    container_name: homeautomation
    restart: unless-stopped
    environment:
      - HA_URL=http://homeassistant.local:8123
      - HA_TOKEN=${HA_TOKEN}
      - LOG_LEVEL=info
    volumes:
      - ./configs:/configs:ro
      - ./logs:/logs
    network_mode: host
```

### Systemd Service (Alternative)

**Service File:** `/etc/systemd/system/homeautomation.service`
```ini
[Unit]
Description=Home Automation System
After=network.target

[Service]
Type=simple
User=homeautomation
WorkingDirectory=/opt/homeautomation
ExecStart=/opt/homeautomation/homeautomation
Restart=always
RestartSec=10
Environment="HA_URL=http://localhost:8123"
Environment="HA_TOKEN=your_token_here"

[Install]
WantedBy=multi-user.target
```

### Configuration

**Environment Variables:**
- `HA_URL` - Home Assistant URL (e.g., http://homeassistant.local:8123)
- `HA_TOKEN` - Long-lived access token for HA API
- `HA_CONNECTION_TYPE` - "websocket" (default: websocket)
- `CONFIG_DIR` - Path to YAML config files (default: ./configs)
- `LOG_LEVEL` - Log level: debug, info, warn, error (default: info)
- `LOG_FILE` - Path to log file (default: stdout)
- `SYNC_INTERVAL` - Full state sync interval in seconds (default: 300)

**Config File:** `config.yaml` (optional, can use env vars)
```yaml
homeassistant:
  url: http://homeassistant.local:8123
  token: ${HA_TOKEN}
  connection_type: websocket
  reconnect_interval: 5s
  max_reconnect_attempts: 10

logging:
  level: info
  format: json
  output: /logs/homeautomation.log

state:
  sync_interval: 5m
  cache_size: 1000

plugins:
  enabled:
    - state_tracking
    - lighting
    - music
    - sleep
    - energy
    - load_shedding
    - security
    - tv
    - calendar
    - nagging
```

### Monitoring and Observability

**Health Endpoint:**
```
GET /health
{
  "status": "healthy",
  "ha_connected": true,
  "plugins_loaded": 10,
  "plugins_running": 10,
  "uptime": "5h32m",
  "last_ha_sync": "2025-11-15T10:30:00Z"
}
```

**Metrics (Prometheus format):**
```
# HELP homeautomation_events_processed_total Total events processed
# TYPE homeautomation_events_processed_total counter
homeautomation_events_processed_total{plugin="music"} 1234

# HELP homeautomation_ha_api_calls_total Total HA API calls
# TYPE homeautomation_ha_api_calls_total counter
homeautomation_ha_api_calls_total{method="call_service",domain="media_player"} 567

# HELP homeautomation_ha_connected HA connection status
# TYPE homeautomation_ha_connected gauge
homeautomation_ha_connected 1
```

**Logging:**
- Structured JSON logging
- Per-plugin log tagging
- Configurable log levels
- Log rotation support

---

## Migration Strategy

### Phase 1: Infrastructure Setup (Week 1)

**Goals:** Core system running, connected to HA, state synchronized

**Tasks:**
1. Set up Golang project structure
2. Implement core components:
   - State Manager with HA sync
   - Config Loader for YAML files
   - Event Bus
   - HA WebSocket Client
3. Implement plugin framework and manager
4. Create health check and monitoring endpoints
5. Set up Docker deployment
6. Deploy and verify HA connection
7. Verify all 33 state variables sync correctly

**Success Criteria:**
- System connects to HA via WebSocket
- All 33 input helpers read on startup
- State changes in HA reflected in Golang
- State changes in Golang sync to HA
- Health endpoint reports healthy

### Phase 2: Plugin Implementation (Weeks 2-4)

**Goals:** Implement all 10 automation plugins with exact Node-RED parity

**Week 2: Foundation Plugins**
1. State Tracking Plugin (presence, sleep, doors)
2. Configuration Plugin (load YAMLs, sun events)
3. Energy State Plugin (energy levels)

**Week 3: Control Plugins**
4. Lighting Control Plugin (scenes, Hue)
5. Music Management Plugin (Sonos, modes)
6. TV Monitoring Plugin (state tracking, brightness)

**Week 4: Integration Plugins**
7. Sleep Hygiene Plugin (wake sequences)
8. Load Shedding Plugin (thermostat control)
9. Security Plugin (lockdown, garage)
10. Calendar & Nagging Plugins (notifications)

**Testing Per Plugin:**
- Unit tests for business logic
- Integration tests with mock HA client
- Parallel testing with Node-RED (compare behaviors)

**Success Criteria:**
- Each plugin implements all automations from corresponding Node-RED flow
- Side-by-side testing shows identical behavior
- All config files loaded correctly

### Phase 3: Parallel Operation (Week 5)

**Goals:** Run Golang and Node-RED in parallel, prove functional equivalence

**Tasks:**
1. Deploy Golang system alongside Node-RED
2. Both systems share state via HA input helpers
3. Run observability/monitoring
4. Compare system behaviors over multiple days
5. Fix any discrepancies found
6. Document any intentional differences

**Success Criteria:**
- Both systems operate without conflicts
- Behaviors are identical across all automations
- No state corruption or race conditions
- Golang system demonstrates stability

### Phase 4: Cutover (Week 6)

**Goals:** Disable Node-RED, run on Golang exclusively

**Tasks:**
1. Choose low-risk time window (e.g., weekend morning)
2. Disable Node-RED flows (keep service running for rollback)
3. Monitor Golang system closely
4. Verify all automations functioning
5. Keep Node-RED disabled for 1 week observation
6. If stable, fully decommission Node-RED

**Rollback Plan:**
- Re-enable Node-RED flows (< 5 minutes)
- Both systems continue using shared HA state
- No data loss or corruption

**Success Criteria:**
- All automations working correctly on Golang alone
- 1 week of stable operation
- No major issues or rollbacks needed

---

## Implementation Roadmap

### Project Structure

```
homeautomation/
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── core/
│   │   ├── state/              # State Manager
│   │   ├── config/             # Config Loader
│   │   ├── events/             # Event Bus
│   │   ├── ha/                 # HA Client (WebSocket)
│   │   └── plugins/            # Plugin Manager
│   ├── plugins/
│   │   ├── state_tracking/     # State Tracking Plugin
│   │   ├── lighting/           # Lighting Control Plugin
│   │   ├── music/              # Music Management Plugin
│   │   ├── sleep/              # Sleep Hygiene Plugin
│   │   ├── energy/             # Energy State Plugin
│   │   ├── load_shedding/      # Load Shedding Plugin
│   │   ├── security/           # Security Plugin
│   │   ├── tv/                 # TV Monitoring Plugin
│   │   ├── calendar/           # Calendar Plugin
│   │   └── nagging/            # Nagging Plugin
│   └── models/                 # Shared data types
├── configs/                    # YAML config files (from Node-RED)
│   ├── music_config.yaml
│   ├── hue_config.yaml
│   ├── schedule_config.yaml
│   └── energy_config.yaml
├── tests/
│   ├── unit/
│   └── integration/
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

### Key Dependencies

```go.mod
module homeautomation

go 1.23

require (
    github.com/gorilla/websocket v1.5.0   // WebSocket client for HA
    gopkg.in/yaml.v3 v3.0.1               // YAML parsing
    github.com/fsnotify/fsnotify v1.6.0   // File watching for config reload
    go.uber.org/zap v1.24.0               // Structured logging
    github.com/prometheus/client_golang v1.15.1  // Metrics
    github.com/stretchr/testify v1.8.4    // Testing
)
```

### Development Priorities

**Must Have (MVP):**
1. Core components (State Manager, Event Bus, HA Client)
2. Plugin framework
3. All 10 plugins with full functionality
4. State synchronization (33 variables)
5. Config loading (4 YAML files)
6. Basic health monitoring

**Should Have:**
7. Comprehensive logging
8. Prometheus metrics
9. Graceful shutdown
10. Hot config reload
11. Auto-reconnect logic

**Nice to Have:**
12. Admin API for runtime inspection
13. Web UI for status/monitoring
14. Automated integration tests
15. Performance benchmarking

### Testing Strategy

**Unit Tests:**
- Each plugin's business logic tested independently
- Mock HA client for service calls
- Mock event bus for event verification

**Integration Tests:**
- Full system tests with mock HA WebSocket server
- Config file parsing tests
- State synchronization tests

**Parallel Testing (with Node-RED):**
- Both systems running simultaneously
- Compare actual outputs (HA service calls, state changes)
- Automated comparison tool

---

## Appendices

### A. State Variable Reference

See `migration_mapping.md` for complete mapping of all 33 variables.

### B. Config File Schemas

See existing YAML files in `configs/` directory:
- `music_config.yaml`
- `hue_config.yaml`
- `schedule_config.yaml`
- `energy_config.yaml`

### C. Home Assistant Service Calls

**Common Services Used:**

| Domain | Service | Purpose |
|--------|---------|---------|
| input_boolean | turn_on | Set boolean to true |
| input_boolean | turn_off | Set boolean to false |
| input_number | set_value | Set number value |
| input_text | set_value | Set text/JSON value |
| media_player | play_media | Play Sonos media |
| media_player | media_play | Resume playback |
| media_player | media_pause | Pause playback |
| media_player | media_stop | Stop playback |
| media_player | volume_set | Set speaker volume |
| media_player | shuffle_set | Enable/disable shuffle |
| media_player | repeat_set | Set repeat mode |
| media_player | join | Join speakers to group |
| media_player | unjoin | Separate speaker from group |
| light | turn_on | Activate light/scene |
| light | turn_off | Deactivate light |
| scene | turn_on | Activate scene |
| tts | google_say | Text-to-speech announcement |
| climate | set_temperature | Set thermostat temp |
| climate | set_hvac_mode | Set HVAC mode |
| lock | lock | Lock doors |
| cover | open_cover | Open garage |

### D. Event Types

**HA State Change Events:**
- Binary sensors (doors, presence, motion)
- Sensors (temperature, energy, etc.)
- Media players (Sonos, Apple TV, TV)
- Lights (Hue, Lutron)
- Input helpers (33 synced variables)

**Internal Events:**
- state.*.changed - Any state variable change
- config.*.reloaded - Config file reloaded
- command.* - Commands to execute actions
- event.* - Custom application events

---

## HomeKit Integration (NRCHKB Replacement)

### Current Node-RED HomeKit Accessories

Node-RED currently exposes **14 HomeKit accessories** via the NRCHKB (Node-RED Contrib HomeKit Bridge) plugin. These appear as switches in Apple Home and Siri:

| Flow | HomeKit Accessory | Purpose |
|------|------------------|---------|
| **State Tracking** | Masters Asleep | Manually set/query master bedroom sleep state |
| **State Tracking** | Guest Asleep | Manually set/query guest sleep state |
| **State Tracking** | Have Guests | Toggle guest presence mode |
| **Configuration** | Reset | Trigger system reset/reload |
| **Lighting Control** | Bright | Force all lights to bright mode |
| **Lighting Control** | Christmas Tree | Control Christmas tree lights |
| **Music** | Airplay | Trigger Airplay music mode |
| **Music** | Sex | Trigger sex music mode |
| **Music** | Volume Restore | Restore volumes to defaults |
| **Vacuum** | Clean Kitchen | Start vacuum in kitchen zone |
| **Vacuum** | Clean Floors | Start vacuum for floor cleaning |
| **Vacuum** | Clean Entryway | Start vacuum in entryway |
| **Vacuum** | Clean Master Bath | Start vacuum in master bath |
| **Vacuum** | Clean Cat Shitolopolis | Start vacuum in cat area |

All accessories are implemented as **Switch** services in HomeKit.

### Migration Strategy: Use Home Assistant's HomeKit Integration

**Recommendation:** Do NOT implement a HomeKit bridge in the Golang system. Instead, create corresponding entities in Home Assistant and use HA's native HomeKit integration.

**Rationale:**

1. **Architectural Consistency**
   - Already using Home Assistant as the state store and control layer
   - Everything flows through HA → simpler architecture
   - One less protocol/bridge to maintain

2. **Proven Reliability**
   - Home Assistant's HomeKit integration is mature and well-maintained
   - Handles pairing, state synchronization, and reconnection automatically
   - Better compatibility with iOS updates

3. **Implementation Simplicity**
   - Golang system only needs to create/update HA entities
   - No need to implement HAP (HomeKit Accessory Protocol)
   - No need to manage HomeKit pairing and persistence

4. **Flexibility**
   - Can use different entity types: `input_boolean` (persistent switches), `input_button` (momentary), or `script` (actions)
   - HA HomeKit integration can expose these with proper characteristics
   - Easier to add new accessories in the future

### Implementation Approach

**Step 1: Create Home Assistant Entities**

Create new input helpers for each HomeKit accessory:

```yaml
# In homeassistant configuration
input_boolean:
  masters_asleep_switch:
    name: Masters Asleep
    icon: mdi:bed
  guest_asleep_switch:
    name: Guest Asleep
    icon: mdi:bed
  have_guests_switch:
    name: Have Guests
    icon: mdi:account-multiple
  bright_mode:
    name: Bright
    icon: mdi:brightness-7
  christmas_tree:
    name: Christmas Tree
    icon: mdi:pine-tree

input_button:
  reset_system:
    name: Reset
    icon: mdi:restart
  airplay_music:
    name: Airplay
    icon: mdi:cast-audio
  sex_music:
    name: Sex
    icon: mdi:music
  volume_restore:
    name: Volume Restore
    icon: mdi:volume-high
  clean_kitchen:
    name: Clean Kitchen
    icon: mdi:robot-vacuum
  clean_floors:
    name: Clean Floors
    icon: mdi:robot-vacuum
  clean_entryway:
    name: Clean Entryway
    icon: mdi:robot-vacuum
  clean_master_bath:
    name: Clean Master Bath
    icon: mdi:robot-vacuum
  clean_cat_area:
    name: Clean Cat Shitolopolis
    icon: mdi:robot-vacuum
```

**Step 2: Configure HA HomeKit Integration**

Include these entities in the HomeKit integration configuration:

```yaml
# In homeassistant configuration
homekit:
  filter:
    include_entities:
      # Sleep state switches
      - input_boolean.masters_asleep_switch
      - input_boolean.guest_asleep_switch
      - input_boolean.have_guests_switch
      # Lighting switches
      - input_boolean.bright_mode
      - input_boolean.christmas_tree
      # Music/action buttons
      - input_button.reset_system
      - input_button.airplay_music
      - input_button.sex_music
      - input_button.volume_restore
      # Vacuum buttons
      - input_button.clean_kitchen
      - input_button.clean_floors
      - input_button.clean_entryway
      - input_button.clean_master_bath
      - input_button.clean_cat_area
  entity_config:
    input_boolean.masters_asleep_switch:
      name: Masters Asleep
      type: switch
    input_button.clean_kitchen:
      name: Clean Kitchen
      type: switch  # Buttons appear as momentary switches in HomeKit
```

**Step 3: Golang System Integration**

The Golang plugins subscribe to these entity state changes and trigger appropriate actions:

```go
// Example: State Tracking Plugin
func (p *StateTrackingPlugin) Initialize(ctx PluginContext) error {
    // Subscribe to the HomeKit switch for master sleep
    ctx.HAClient.SubscribeStateChanges("input_boolean.masters_asleep_switch",
        p.handleMasterAsleepSwitch)

    // Also sync to the shared state variable
    ctx.HAClient.SubscribeStateChanges("input_boolean.masters_asleep_switch",
        func(state State) {
            // Update shared state
            ctx.StateManager.SetBool("isMasterAsleep", state.Value.(bool))
        })

    return nil
}

// Example: Music Plugin
func (p *MusicPlugin) Initialize(ctx PluginContext) error {
    // Subscribe to music mode buttons
    ctx.HAClient.SubscribeStateChanges("input_button.airplay_music",
        func(state State) {
            p.setMusicMode("airplay")
        })

    ctx.HAClient.SubscribeStateChanges("input_button.sex_music",
        func(state State) {
            p.setMusicMode("sex")
        })

    ctx.HAClient.SubscribeStateChanges("input_button.volume_restore",
        func(state State) {
            p.restoreVolumes()
        })

    return nil
}

// Example: Vacuum Plugin
func (p *VacuumPlugin) Initialize(ctx PluginContext) error {
    // Subscribe to vacuum zone buttons
    zones := map[string]string{
        "input_button.clean_kitchen": "kitchen",
        "input_button.clean_floors": "floors",
        "input_button.clean_entryway": "entryway",
        "input_button.clean_master_bath": "master_bath",
        "input_button.clean_cat_area": "cat_area",
    }

    for entity, zone := range zones {
        zone := zone // capture for closure
        ctx.HAClient.SubscribeStateChanges(entity, func(state State) {
            p.startVacuumZone(zone)
        })
    }

    return nil
}
```

### Relationship to Existing State Variables

**Important:** Some HomeKit accessories map to existing synced state variables:

| HomeKit Accessory | Existing State Variable | Notes |
|------------------|------------------------|-------|
| Masters Asleep | `isMasterAsleep` (input_boolean.master_asleep) | **Duplicate!** Should consolidate |
| Guest Asleep | `isGuestAsleep` (input_boolean.guest_asleep) | **Duplicate!** Should consolidate |
| Have Guests | `isHaveGuests` (input_boolean.have_guests) | **Duplicate!** Should consolidate |
| Christmas Tree | N/A (new) | Seasonal control not in current state vars |
| Bright | N/A (trigger) | Temporary override, not persistent state |
| Others | N/A (triggers) | One-time action buttons |

**Consolidation Strategy:**

For the **duplicate** sleep/guest state switches:
- **Do not create new entities** - reuse the existing `input_boolean` entities from the 33 synced variables
- Simply expose those existing entities through HA's HomeKit integration
- The Golang system already syncs these, so no additional work needed

Updated entity list:

```yaml
homekit:
  filter:
    include_entities:
      # Reuse existing synced state variables
      - input_boolean.master_asleep       # Already exists!
      - input_boolean.guest_asleep        # Already exists!
      - input_boolean.have_guests         # Already exists!

      # New entities for HomeKit-only controls
      - input_boolean.bright_mode         # Create new
      - input_boolean.christmas_tree      # Create new
      - input_button.reset_system         # Create new
      - input_button.airplay_music        # Create new
      - input_button.sex_music            # Create new
      - input_button.volume_restore       # Create new
      - input_button.clean_kitchen        # Create new
      - input_button.clean_floors         # Create new
      - input_button.clean_entryway       # Create new
      - input_button.clean_master_bath    # Create new
      - input_button.clean_cat_area       # Create new
```

**Summary:**
- **3 entities** already exist (reuse)
- **11 new entities** to create (2 booleans + 9 buttons)
- **Total exposed to HomeKit:** 14 accessories (matching current NRCHKB count)

### Migration Checklist

- [ ] Create 11 new entities in Home Assistant (2 `input_boolean`, 9 `input_button`)
- [ ] Configure HA HomeKit integration to expose all 14 entities
- [ ] Re-pair HomeKit bridge with Apple Home (entities will have new UUIDs)
- [ ] Update automation shortcuts/scenes in Apple Home
- [ ] Implement event handlers in Golang plugins for new entities
- [ ] Test each HomeKit switch/button triggers correct automation
- [ ] Decommission NRCHKB bridge in Node-RED

### Benefits of This Approach

1. **Zero additional infrastructure** - Uses existing HA HomeKit integration
2. **Better maintainability** - One place to manage HomeKit accessories (HA config)
3. **Easier debugging** - Can test HA entities directly before HomeKit exposure
4. **Flexibility** - Can easily add/remove HomeKit accessories by editing HA config
5. **Performance** - No additional protocol translation in Golang

---

## Resources and References

This design was created by analyzing the following resources from the existing Node-RED implementation. Future design refinement sessions should reference these same resources for consistency.

### Primary Documentation

1. **migration_mapping.md**
   - Complete mapping of 33 state variables from Node-RED to Home Assistant
   - Entity types, names, and data structures
   - Classification of active vs. disabled flow variables
   - Location: `/Users/nborgers/code/node-red/migration_mapping.md`

2. **HA_SYNC_README.md**
   - Documentation of current bidirectional sync between Node-RED and HA
   - Synchronization strategy and implementation details
   - Setup instructions and troubleshooting
   - Location: `/Users/nborgers/code/node-red/HA_SYNC_README.md`

3. **README.md**
   - High-level overview of each Node-RED flow
   - Component architecture diagram
   - Description of automation purposes
   - Location: `/Users/nborgers/code/node-red/README.md`

4. **CREATE_HELPERS_GUI.md**
   - Detailed specifications for the 23 new HA input helpers
   - Entity IDs, types, ranges, and icons
   - Location: `/Users/nborgers/code/node-red/CREATE_HELPERS_GUI.md`

### Node-RED Flows

5. **flows.json**
   - Complete Node-RED flow definitions (634KB+)
   - Contains all node configurations, connections, and logic
   - Location: `/Users/nborgers/code/node-red/flows.json`
   - **Note:** File is large; use targeted searches for specific flows or node types

6. **Live Node-RED Instance**
   - Running system for interactive exploration
   - URL: `https://node-red.featherback-mermaid.ts.net/#flow/90f5fe8cb80ae6a7`
   - Can navigate to different flow tabs to see live configuration

### Flow Screenshots

7. **Automated Screenshots**
   - Visual representation of each Node-RED flow
   - Location: `/Users/nborgers/code/node-red/.automated-rendering/screenshot-capture/screenshots/`
   - Key screenshots examined:
     - `State Tracking.png` - Presence and sleep state tracking logic
     - `Lighting Control.png` - Scene activation and sun event handling
     - `Music.png` - Complex music mode selection and Sonos control
     - `Sleep Hygiene.png` - Wake-up sequences and fade-out logic
     - `Energy State.png` - Battery and solar generation tracking
     - `Security.png` - Lockdown and garage automation
     - `TV Monitoring and Manipulation.png` - TV state detection and control
     - `Load Shedding.png` - Thermostat range adjustment
     - `Configuration.png` - YAML config loading and sun events
     - `Calendar.png` - Meeting reminder logic
     - `Nagging.png` - Weather-based reminders

### Configuration Files

8. **YAML Configuration Files**
   - `configs/music_config.yaml` - Music modes, playlists, volumes, participants
   - `configs/hue_config.yaml` - Lighting scenes and room mappings
   - `configs/schedule_config.yaml` - Time-based schedules and triggers
   - `configs/energy_config.yaml` - Energy level thresholds and behaviors
   - **Note:** These files define behavior and must be loaded by Golang system

### Architecture Context

9. **System Components**
   - Node-RED → Home Assistant → Devices (Sonos, Hue, Apple TV, etc.)
   - WebSocket communication between Node-RED and HA
   - Homekit presence detection
   - See README.md for complete component diagram

### Design Decisions Context

10. **User Requirements Captured:**
    - Architecture: Plugin-based monolith
    - HA Communication: WebSocket preferred
    - Config Files: Reuse existing YAMLs without modification
    - Detail Level: High-level overview
    - Goal: 1:1 functional migration

### How to Use These Resources in Future Sessions

**For Design Refinement:**
```bash
# Read the complete flow configuration for a specific plugin
grep -A 50 '"label":"Music"' flows.json

# Examine specific node types
grep -A 10 '"type":"function"' flows.json | grep -A 10 '"name":"Pick Appropriate Music"'

# View screenshots
open .automated-rendering/screenshot-capture/screenshots/"Music.png"

# Check current state variable usage
grep -r "isNickHome" flows.json
```

You can access the running Node Red with your MCP server at: https://node-red.featherback-mermaid.ts.net/

**For Implementation Details:**
1. Start with the screenshot for the flow you're implementing
2. Cross-reference with flows.json for exact node configurations
3. Check migration_mapping.md for state variables used
4. Review config YAMLs for data structures
5. Test against live Node-RED instance for behavior verification

**Critical Resources for Each Plugin:**

| Plugin | Screenshot | Config File | Key State Variables |
|--------|-----------|-------------|---------------------|
| State Tracking | State Tracking.png | N/A | isNickHome, isCarolineHome, isToriHere, sleep states |
| Lighting | Lighting Control.png | hue_config.yaml | dayPhase, sunevent, isAnyoneHome |
| Music | Music.png | music_config.yaml | musicPlaybackType, currentlyPlayingMusic, sleep states |
| Sleep | Sleep Hygiene.png | schedule_config.yaml | isMasterAsleep, alarmTime, musicPlaybackType |
| Energy | Energy State.png | energy_config.yaml | batteryEnergyLevel, solarProductionEnergyLevel, etc. |
| Load Shedding | Load Shedding.png | N/A | currentEnergyLevel |
| Security | Security.png | N/A | isEveryoneAsleep, isAnyoneHome, isExpectingSomeone |
| TV | TV Monitoring and Manipulation.png | N/A | isTVPlaying, isAppleTVPlaying, isTVOn, dayPhase |
| Calendar | Calendar.png | N/A | isNickHome, isCarolineHome |
| Nagging | Nagging.png | N/A | isAnyoneHome, musicPlaybackType |

---

## Summary

This design provides a complete blueprint for migrating the Node-RED home automation system to a modern, maintainable Golang implementation. The plugin-based architecture ensures modularity while maintaining simplicity through a monolithic deployment. State synchronization via Home Assistant input helpers enables a gradual, risk-free migration with the ability to run both systems in parallel.

The system maintains 100% functional parity with the existing Node-RED flows while providing better performance, type safety, and long-term maintainability.

**Next Steps:**
1. Review and approve this design document
2. Begin Phase 1 implementation (core infrastructure)
3. Set up development environment and CI/CD
4. Start plugin development in Phase 2
