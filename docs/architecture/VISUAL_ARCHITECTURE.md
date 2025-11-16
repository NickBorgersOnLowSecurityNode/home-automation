# Visual Architecture Guide

This document provides Mermaid diagrams to visualize the Golang home automation system architecture and logic flows.

## Table of Contents
- [System Architecture](#system-architecture)
- [Plugin System Architecture](#plugin-system-architecture)
- [State Synchronization Flow](#state-synchronization-flow)
- [Music Manager Logic Flow](#music-manager-logic-flow)
- [Lighting Control Logic Flow](#lighting-control-logic-flow)
- [Energy State Logic Flow](#energy-state-logic-flow)
- [State Variable Dependency Graph](#state-variable-dependency-graph)

---

## System Architecture

High-level view of the system components and their interactions.

```mermaid
graph TB
    subgraph "Home Automation Go Application"
        Main[cmd/main.go]

        subgraph "Core Layer"
            HAClient[HA WebSocket Client<br/>internal/ha/client.go]
            StateManager[State Manager<br/>internal/state/manager.go]
            Variables[State Variables<br/>internal/state/variables.go]
        end

        subgraph "Plugin Layer"
            Music[Music Manager<br/>internal/plugins/music/]
            Lighting[Lighting Manager<br/>internal/plugins/lighting/]
            Energy[Energy Manager<br/>internal/plugins/energy/]
            TV[TV Manager<br/>internal/plugins/tv/]
            Sleep[Sleep Hygiene Manager<br/>internal/plugins/sleephygiene/]
            Security[Security Manager<br/>internal/plugins/security/]
            LoadShed[Load Shedding Manager<br/>internal/plugins/loadshedding/]
        end

        subgraph "Configuration"
            ConfigLoader[Config Loader<br/>internal/config/loader.go]
            MusicConfig[music_config.yaml]
            LightConfig[hue_config.yaml]
            EnergyConfig[energy_config.yaml]
        end
    end

    subgraph "External Systems"
        HA[Home Assistant<br/>WebSocket API]
        Sonos[Sonos Speakers]
        Hue[Phillips Hue]
    end

    Main --> HAClient
    Main --> StateManager
    Main --> ConfigLoader

    HAClient <-->|WebSocket<br/>Auth, Commands,<br/>State Changes| HA

    StateManager -->|Read/Write<br/>State Variables| HAClient
    StateManager -.->|Subscribe to<br/>State Changes| Variables

    ConfigLoader -->|Load YAML| MusicConfig
    ConfigLoader -->|Load YAML| LightConfig
    ConfigLoader -->|Load YAML| EnergyConfig

    Music -->|Get/Set State| StateManager
    Music -->|Call Services| HAClient
    Music -.->|Uses Config| MusicConfig

    Lighting -->|Get/Set State| StateManager
    Lighting -->|Call Services| HAClient
    Lighting -.->|Uses Config| LightConfig

    Energy -->|Get/Set State| StateManager
    Energy -.->|Uses Config| EnergyConfig

    TV -->|Get/Set State| StateManager
    TV -->|Subscribe to<br/>Media Players| HAClient

    Sleep -->|Get/Set State| StateManager
    Sleep -->|Trigger Music| Music
    Sleep -->|Trigger Lights| Lighting

    Security -->|Get/Set State| StateManager
    Security -->|Call Services| HAClient

    LoadShed -->|Get/Set State| StateManager
    LoadShed -->|Call Services| HAClient

    HA -->|Control| Sonos
    HA -->|Control| Hue

    style Main fill:#e1f5ff
    style HAClient fill:#fff3e0
    style StateManager fill:#fff3e0
    style Music fill:#e8f5e9
    style Lighting fill:#e8f5e9
    style Energy fill:#e8f5e9
    style TV fill:#e8f5e9
    style Sleep fill:#e8f5e9
    style Security fill:#e8f5e9
    style LoadShed fill:#e8f5e9
```

---

## Plugin System Architecture

How plugins interact with the core state management system.

```mermaid
sequenceDiagram
    participant Main as cmd/main.go
    participant SM as State Manager
    participant HAC as HA Client
    participant Plugin as Plugin Manager
    participant HA as Home Assistant

    Main->>HAC: NewClient(url, token)
    Main->>SM: NewManager(haClient, logger)
    Main->>SM: SyncFromHA()
    SM->>HAC: GetAllStates()
    HAC->>HA: Get All States (WS)
    HA-->>HAC: State Array
    HAC-->>SM: States
    SM->>SM: Parse & Cache All Variables
    SM->>HAC: SubscribeStateChanges(entityID)

    Main->>Plugin: NewManager(haClient, stateManager, config)
    Main->>Plugin: Start()

    Plugin->>SM: Subscribe("dayPhase", handler)
    SM-->>Plugin: Subscription

    Plugin->>SM: Subscribe("isAnyoneHome", handler)
    SM-->>Plugin: Subscription

    Note over Plugin: Plugin is now monitoring state changes

    HA->>HAC: State Change Event (WS)
    HAC->>SM: Handler Callback
    SM->>SM: Update Cache
    SM->>Plugin: Notify Subscribed Handler
    Plugin->>Plugin: Business Logic
    Plugin->>SM: SetBool/SetString/SetNumber
    SM->>HAC: SetInputBoolean/Text/Number
    HAC->>HA: Call Service (WS)
```

---

## State Synchronization Flow

How state changes propagate through the system.

```mermaid
flowchart TD
    Start([State Change in HA]) --> WSEvent[WebSocket Event Received]
    WSEvent --> HAClient[HA Client Receives Event]
    HAClient --> ParseEvent{Parse Event Type}

    ParseEvent -->|state_changed| ExtractState[Extract Entity ID & New State]
    ParseEvent -->|Other| Ignore[Ignore Event]

    ExtractState --> FindSubs{Entity Has<br/>Subscriptions?}
    FindSubs -->|No| End1([End])
    FindSubs -->|Yes| CallHandlers[Call All Subscriber Handlers]

    CallHandlers --> SMHandler[State Manager Handler]
    SMHandler --> ParseValue[Parse State Value by Type]
    ParseValue --> TypeCheck{Variable Type?}

    TypeCheck -->|Boolean| ParseBool["Parse 'on'/'off' → true/false"]
    TypeCheck -->|Number| ParseNum[Parse String → Float64]
    TypeCheck -->|String| UseString[Use String Directly]
    TypeCheck -->|JSON| ParseJSON[Parse JSON String]

    ParseBool --> UpdateCache
    ParseNum --> UpdateCache
    UseString --> UpdateCache
    ParseJSON --> UpdateCache

    UpdateCache[Update State Manager Cache] --> NotifyPlugins{Any Plugin<br/>Subscriptions?}

    NotifyPlugins -->|No| End2([End])
    NotifyPlugins -->|Yes| CallPluginHandlers[Call Plugin Handlers]

    CallPluginHandlers --> PluginLogic[Plugin Business Logic]
    PluginLogic --> Decision{Plugin Needs<br/>to Update State?}

    Decision -->|No| End3([End])
    Decision -->|Yes| SetState[Plugin Calls SetBool/SetString/SetNumber]

    SetState --> CheckReadOnly{Read-Only<br/>Mode?}
    CheckReadOnly -->|Yes| LogOnly[Log Would-Be Change]
    CheckReadOnly -->|No| SyncToHA[Sync to Home Assistant]

    LogOnly --> End4([End])
    SyncToHA --> CallService[Call HA Service via WebSocket]
    CallService --> End5([End])

    style Start fill:#e1f5ff
    style UpdateCache fill:#fff3e0
    style PluginLogic fill:#e8f5e9
    style CallService fill:#ffebee
```

---

## Music Manager Logic Flow

Decision tree for music mode selection (matches Node-RED Music flow).

```mermaid
flowchart TD
    Start([State Change Detected]) --> GetState[Get Current State:<br/>isAnyoneHome<br/>isAnyoneAsleep<br/>dayPhase]

    GetState --> CheckHome{isAnyoneHome?}
    CheckHome -->|No| StopMusic[Set musicPlaybackType = '']
    CheckHome -->|Yes| CheckAsleep{isAnyoneAsleep?}

    CheckAsleep -->|Yes| SetSleep[Set musicPlaybackType = 'sleep']
    CheckAsleep -->|No| CheckDayPhase{dayPhase?}

    CheckDayPhase -->|morning| CheckWakeUp{Is Wake-Up Event?}
    CheckWakeUp -->|Yes| CheckSunday{Is Sunday?}
    CheckSunday -->|Yes| SetDay1[Set musicPlaybackType = 'day']
    CheckSunday -->|No| SetMorning[Set musicPlaybackType = 'morning']
    CheckWakeUp -->|No| SetDay2[Set musicPlaybackType = 'day']

    CheckDayPhase -->|day| SetDay3[Set musicPlaybackType = 'day']
    CheckDayPhase -->|sunset/dusk| SetEvening[Set musicPlaybackType = 'evening']
    CheckDayPhase -->|winddown/night| CheckCurrentSleep{Current Type<br/>= 'sleep'?}

    CheckCurrentSleep -->|Yes| KeepSleep[Keep musicPlaybackType = 'sleep']
    CheckCurrentSleep -->|No| SetWinddown[Set musicPlaybackType = 'winddown']

    StopMusic --> End1([End])
    SetSleep --> End2([End])
    SetMorning --> TriggerPlayback1
    SetDay1 --> TriggerPlayback2
    SetDay2 --> TriggerPlayback3
    SetDay3 --> TriggerPlayback4
    SetEvening --> TriggerPlayback5
    SetWinddown --> TriggerPlayback6
    KeepSleep --> End3([End])

    TriggerPlayback1[State Change Triggers<br/>Playback Handler] --> Orchestrate1
    TriggerPlayback2[State Change Triggers<br/>Playback Handler] --> Orchestrate2
    TriggerPlayback3[State Change Triggers<br/>Playback Handler] --> Orchestrate3
    TriggerPlayback4[State Change Triggers<br/>Playback Handler] --> Orchestrate4
    TriggerPlayback5[State Change Triggers<br/>Playback Handler] --> Orchestrate5
    TriggerPlayback6[State Change Triggers<br/>Playback Handler] --> Orchestrate6

    Orchestrate1[Orchestrate Playback] --> SelectPlaylist
    Orchestrate2[Orchestrate Playback] --> SelectPlaylist
    Orchestrate3[Orchestrate Playback] --> SelectPlaylist
    Orchestrate4[Orchestrate Playback] --> SelectPlaylist
    Orchestrate5[Orchestrate Playback] --> SelectPlaylist
    Orchestrate6[Orchestrate Playback] --> SelectPlaylist

    SelectPlaylist[Select Playlist with Rotation<br/>from music_config.yaml] --> BuildGroup[Build Sonos Speaker Group]
    BuildGroup --> MuteAll[Mute All Speakers to 0]
    MuteAll --> StartPlayback[Start Playback on Lead Player]
    StartPlayback --> EnableShuffle[Enable Shuffle for Playlists]
    EnableShuffle --> EvalConditions[Evaluate Mute Conditions<br/>for Each Speaker]
    EvalConditions --> FadeIn[Fade In Eligible Speakers<br/>Gradually 0→targetVolume]
    FadeIn --> Complete([Playback Complete])

    style Start fill:#e1f5ff
    style CheckHome fill:#fff3e0
    style CheckAsleep fill:#fff3e0
    style CheckDayPhase fill:#fff3e0
    style SelectPlaylist fill:#e8f5e9
    style StartPlayback fill:#e8f5e9
```

**Reference:** See `homeautomation-go/internal/plugins/music/manager.go` for implementation details.

---

## Lighting Control Logic Flow

Scene activation based on day phase and conditional logic (matches Node-RED Lighting Control flow).

```mermaid
flowchart TD
    Start([State Change:<br/>dayPhase or<br/>isAnyoneHome or<br/>isAnyoneAsleep]) --> GetState[Get Current State:<br/>dayPhase<br/>isAnyoneHome<br/>isAnyoneAsleep<br/>isTVPlaying]

    GetState --> LoadConfig[Load hue_config.yaml<br/>Scene Configurations]

    LoadConfig --> IterateRooms[For Each Room in Config]

    IterateRooms --> CheckConditions{Evaluate Room<br/>Conditions}

    CheckConditions -->|on_if_true matched| GetSceneOn1[Get Scene Name<br/>from Config]
    CheckConditions -->|on_if_false matched| GetSceneOn2[Get Scene Name<br/>from Config]
    CheckConditions -->|default| GetSceneDefault[Get Default Scene<br/>for dayPhase]
    CheckConditions -->|off_if_true matched| TurnOff1[Turn Room Off]
    CheckConditions -->|off_if_false matched| TurnOff2[Turn Room Off]

    GetSceneOn1 --> FormatScene1[Format Scene Name:<br/>room/dayPhase]
    GetSceneOn2 --> FormatScene2[Format Scene Name:<br/>room/dayPhase]
    GetSceneDefault --> FormatScene3[Format Scene Name:<br/>room/dayPhase]

    FormatScene1 --> ActivateScene1[Call scene.turn_on<br/>entity_id: scene.ROOM_SCENE]
    FormatScene2 --> ActivateScene2[Call scene.turn_on<br/>entity_id: scene.ROOM_SCENE]
    FormatScene3 --> ActivateScene3[Call scene.turn_on<br/>entity_id: scene.ROOM_SCENE]

    TurnOff1 --> CallLightOff1[Call light.turn_off<br/>for room entities]
    TurnOff2 --> CallLightOff2[Call light.turn_off<br/>for room entities]

    ActivateScene1 --> NextRoom1{More Rooms?}
    ActivateScene2 --> NextRoom2{More Rooms?}
    ActivateScene3 --> NextRoom3{More Rooms?}
    CallLightOff1 --> NextRoom4{More Rooms?}
    CallLightOff2 --> NextRoom5{More Rooms?}

    NextRoom1 -->|Yes| IterateRooms
    NextRoom2 -->|Yes| IterateRooms
    NextRoom3 -->|Yes| IterateRooms
    NextRoom4 -->|Yes| IterateRooms
    NextRoom5 -->|Yes| IterateRooms

    NextRoom1 -->|No| Complete([All Scenes Updated])
    NextRoom2 -->|No| Complete
    NextRoom3 -->|No| Complete
    NextRoom4 -->|No| Complete
    NextRoom5 -->|No| Complete

    style Start fill:#e1f5ff
    style CheckConditions fill:#fff3e0
    style ActivateScene1 fill:#e8f5e9
    style ActivateScene2 fill:#e8f5e9
    style ActivateScene3 fill:#e8f5e9
```

**Reference:** See `homeautomation-go/internal/plugins/lighting/manager.go` for implementation details.

**Condition Evaluation Logic:**
- `on_if_true`: Activate scene if ALL specified state variables are true
- `on_if_false`: Activate scene if ALL specified state variables are false
- `off_if_true`: Turn off room if ALL specified state variables are true
- `off_if_false`: Turn off room if ALL specified state variables are false
- Conditions are evaluated in order of precedence: off conditions → on conditions → default

---

## Energy State Logic Flow

Battery level calculation and energy state management (matches Node-RED Energy State flow).

```mermaid
flowchart TD
    Start([HA Sensor Update:<br/>sensor.span_battery_charge_percent]) --> GetBatteryPercent[Get Battery Charge %]

    GetBatteryPercent --> LoadConfig[Load energy_config.yaml<br/>Battery Level Thresholds]

    LoadConfig --> CheckLevels{Compare Battery %<br/>to Thresholds}

    CheckLevels -->|< critical_threshold| SetCritical[batteryEnergyLevel = 'critical']
    CheckLevels -->|< low_threshold| SetLow[batteryEnergyLevel = 'low']
    CheckLevels -->|< medium_threshold| SetMedium[batteryEnergyLevel = 'medium']
    CheckLevels -->|< high_threshold| SetHigh[batteryEnergyLevel = 'high']
    CheckLevels -->|>= high_threshold| SetFull[batteryEnergyLevel = 'full']

    SetCritical --> SyncToHA1[Sync to HA:<br/>input_text.battery_energy_level]
    SetLow --> SyncToHA2[Sync to HA:<br/>input_text.battery_energy_level]
    SetMedium --> SyncToHA3[Sync to HA:<br/>input_text.battery_energy_level]
    SetHigh --> SyncToHA4[Sync to HA:<br/>input_text.battery_energy_level]
    SetFull --> SyncToHA5[Sync to HA:<br/>input_text.battery_energy_level]

    SyncToHA1 --> CalculateCurrent1
    SyncToHA2 --> CalculateCurrent2
    SyncToHA3 --> CalculateCurrent3
    SyncToHA4 --> CalculateCurrent4
    SyncToHA5 --> CalculateCurrent5

    CalculateCurrent1[Calculate currentEnergyLevel] --> CheckFreeEnergy1
    CalculateCurrent2[Calculate currentEnergyLevel] --> CheckFreeEnergy2
    CalculateCurrent3[Calculate currentEnergyLevel] --> CheckFreeEnergy3
    CalculateCurrent4[Calculate currentEnergyLevel] --> CheckFreeEnergy4
    CalculateCurrent5[Calculate currentEnergyLevel] --> CheckFreeEnergy5

    CheckFreeEnergy1{isFreeEnergyAvailable?} -->|Yes| SetInfinite1[currentEnergyLevel = 'infinite']
    CheckFreeEnergy2{isFreeEnergyAvailable?} -->|Yes| SetInfinite2[currentEnergyLevel = 'infinite']
    CheckFreeEnergy3{isFreeEnergyAvailable?} -->|Yes| SetInfinite3[currentEnergyLevel = 'infinite']
    CheckFreeEnergy4{isFreeEnergyAvailable?} -->|Yes| SetInfinite4[currentEnergyLevel = 'infinite']
    CheckFreeEnergy5{isFreeEnergyAvailable?} -->|Yes| SetInfinite5[currentEnergyLevel = 'infinite']

    CheckFreeEnergy1 -->|No| CheckGrid1{isGridAvailable?}
    CheckFreeEnergy2 -->|No| CheckGrid2{isGridAvailable?}
    CheckFreeEnergy3 -->|No| CheckGrid3{isGridAvailable?}
    CheckFreeEnergy4 -->|No| CheckGrid4{isGridAvailable?}
    CheckFreeEnergy5 -->|No| CheckGrid5{isGridAvailable?}

    CheckGrid1 -->|Yes & battery high/full| SetAbundant1[currentEnergyLevel = 'abundant']
    CheckGrid2 -->|Yes & battery high/full| SetAbundant2[currentEnergyLevel = 'abundant']
    CheckGrid3 -->|Yes & battery medium| SetPlenty1[currentEnergyLevel = 'plenty']
    CheckGrid4 -->|Yes & battery high| SetPlenty2[currentEnergyLevel = 'plenty']
    CheckGrid5 -->|Yes & battery full| SetAbundant3[currentEnergyLevel = 'abundant']

    CheckGrid1 -->|No or battery low/critical| UseBattery1[currentEnergyLevel = batteryEnergyLevel]
    CheckGrid2 -->|No or battery low/critical| UseBattery2[currentEnergyLevel = batteryEnergyLevel]
    CheckGrid3 -->|No| UseBattery3[currentEnergyLevel = batteryEnergyLevel]
    CheckGrid4 -->|No| UseBattery4[currentEnergyLevel = batteryEnergyLevel]
    CheckGrid5 -->|No| UseBattery5[currentEnergyLevel = batteryEnergyLevel]

    SetInfinite1 --> End1([End])
    SetInfinite2 --> End2([End])
    SetInfinite3 --> End3([End])
    SetInfinite4 --> End4([End])
    SetInfinite5 --> End5([End])

    SetAbundant1 --> End6([End])
    SetAbundant2 --> End7([End])
    SetAbundant3 --> End8([End])
    SetPlenty1 --> End9([End])
    SetPlenty2 --> End10([End])

    UseBattery1 --> End11([End])
    UseBattery2 --> End12([End])
    UseBattery3 --> End13([End])
    UseBattery4 --> End14([End])
    UseBattery5 --> End15([End])

    style Start fill:#e1f5ff
    style CheckLevels fill:#fff3e0
    style CheckFreeEnergy1 fill:#fff3e0
    style CheckGrid1 fill:#fff3e0
    style SetInfinite1 fill:#c8e6c9
    style SetAbundant1 fill:#e8f5e9
    style UseBattery1 fill:#ffebee
```

**Reference:** See `homeautomation-go/internal/plugins/energy/manager.go` for implementation details.

---

## State Variable Dependency Graph

Shows which plugins read/write which state variables.

```mermaid
graph LR
    subgraph "State Variables (inputs)"
        NickHome[isNickHome]
        CarolineHome[isCarolineHome]
        ToriHere[isToriHere]
        MasterAsleep[isMasterAsleep]
        GuestAsleep[isGuestAsleep]
        TVPlaying[isTVPlaying]
        AlarmTime[alarmTime]
        BatteryPercent[sensor.span_battery_*]
    end

    subgraph "Computed State Variables"
        AnyOwnerHome[isAnyOwnerHome]
        AnyoneHome[isAnyoneHome]
        AnyoneAsleep[isAnyoneAsleep]
        EveryoneAsleep[isEveryoneAsleep]
        DayPhase[dayPhase]
        BatteryLevel[batteryEnergyLevel]
        CurrentEnergy[currentEnergyLevel]
    end

    subgraph "Output State Variables"
        MusicType[musicPlaybackType]
        MusicURI[currentlyPlayingMusicUri]
        FadeOut[isFadeOutInProgress]
    end

    subgraph "Plugins"
        StateTracking[State Tracking Plugin]
        DayPhasePlugin[Day Phase Plugin]
        Music[Music Plugin]
        Lighting[Lighting Plugin]
        Energy[Energy Plugin]
        SleepHygiene[Sleep Hygiene Plugin]
        TV[TV Plugin]
        Security[Security Plugin]
        LoadShedding[Load Shedding Plugin]
    end

    NickHome --> StateTracking
    CarolineHome --> StateTracking
    ToriHere --> StateTracking
    MasterAsleep --> StateTracking
    GuestAsleep --> StateTracking

    StateTracking --> AnyOwnerHome
    StateTracking --> AnyoneHome
    StateTracking --> AnyoneAsleep
    StateTracking --> EveryoneAsleep

    DayPhasePlugin --> DayPhase

    AnyoneHome --> Music
    AnyoneAsleep --> Music
    DayPhase --> Music
    Music --> MusicType
    Music --> MusicURI

    DayPhase --> Lighting
    AnyoneHome --> Lighting
    AnyoneAsleep --> Lighting
    TVPlaying --> Lighting

    BatteryPercent --> Energy
    Energy --> BatteryLevel
    Energy --> CurrentEnergy

    AlarmTime --> SleepHygiene
    MasterAsleep --> SleepHygiene
    SleepHygiene --> FadeOut
    SleepHygiene -.->|Triggers| Music
    SleepHygiene -.->|Triggers| Lighting

    TV --> TVPlaying

    AnyoneHome --> Security
    EveryoneAsleep --> Security

    CurrentEnergy --> LoadShedding

    style AnyOwnerHome fill:#fff3e0
    style AnyoneHome fill:#fff3e0
    style AnyoneAsleep fill:#fff3e0
    style EveryoneAsleep fill:#fff3e0
    style DayPhase fill:#fff3e0
    style BatteryLevel fill:#fff3e0
    style CurrentEnergy fill:#fff3e0

    style MusicType fill:#e8f5e9
    style MusicURI fill:#e8f5e9
    style FadeOut fill:#e8f5e9
```

---

## How to Use These Diagrams

### Viewing in GitHub
All Mermaid diagrams render automatically in GitHub's markdown viewer.

### Viewing in VS Code
Install the "Markdown Preview Mermaid Support" extension for inline rendering.

### Updating Diagrams
When code changes significantly:
1. Update the relevant diagram(s) in this file
2. Ensure the diagram matches actual implementation
3. Reference file paths and line numbers when helpful
4. Update the "Last Updated" date in git commits

### Creating New Diagrams
Follow these conventions:
- Use consistent colors:
  - Light blue (`#e1f5ff`) for entry points
  - Light orange (`#fff3e0`) for decision/branching logic
  - Light green (`#e8f5e9`) for actions/outputs
  - Light red (`#ffebee`) for error/critical paths
- Include file references for traceability
- Keep diagrams focused on one concept/flow
- Link to implementation code with file paths

---

**Last Updated:** 2025-11-16
**Maintained By:** Development Team
**Related Documentation:**
- [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) - Architecture and design decisions
- [GOLANG_DESIGN.md](./GOLANG_DESIGN.md) - Detailed flow descriptions
- [../migration/migration_mapping.md](../migration/migration_mapping.md) - State variable mapping
