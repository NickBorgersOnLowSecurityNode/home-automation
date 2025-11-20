# Node-RED Music Playback Flow Analysis

## Overview

The Node-RED Music flow orchestrates music playback across multiple Sonos speakers with sophisticated logic for:
- Dynamic playlist rotation based on playback type (morning, day, evening, winddown, sleep, wakeup, sex)
- Volume calculation based on base_volume × volume_multiplier
- Speaker group building and management
- Conditional muting based on state variables (sleep, TV playing, etc.)
- Gradual volume ramp-up (fade-in) with 250ms per step
- Rate limiting (max 1 playback per 10 seconds via delay nodes)
- Prevention of double-activation of already-playing music

## Flow Architecture

The Music flow contains:
- **15 Function Nodes** - Core business logic in JavaScript
- **8 Switch Nodes** - Conditional routing
- **25 Delay Nodes** - Rate limiting and fade-in timing
- **154 Total Nodes** - Complete orchestration

### Trigger Points

Music mode selection is triggered by:
1. **Automatic** - "Set music type based on conditions" function based on:
   - Time of day (dayPhase: morning, day, sunset, evening, winddown, night)
   - Presence state (isAnyoneHome)
   - Sleep state (isAnyoneAsleep)
   - Sunday override (no morning music on Sundays)

2. **Manual** - Inject nodes for:
   - Make Morning Music Playback
   - Make Day Music Playback
   - Make Evening Music Playback
   - Make Winddown Playback
   - Make Sleep Playback
   - No Music Playback
   - Shutdown all music
   - Manually reactivate music
   - Force volume reset

## Key Data Structures

### musicPlaybackType (input_text.music_playback_type)
Current playback mode: "morning", "day", "evening", "winddown", "sleep", "wakeup", "sex", or "" (none)

### musicConfig (from YAML)
Loaded from configs/music_config.yaml:
```yaml
music:
  [playback_type]:
    participants:
      - player_name: "Kitchen"        # Speaker name
        base_volume: 9                # Default volume (0-15)
        leave_muted_if:               # Conditional mute criteria
          - variable: [state_var]     # Variable name
            value: true/false         # Value that triggers mute
    playback_options:
      - uri: "spotify:playlist:..."   # Spotify URI or HTTP URL
        media_type: "playlist|music"
        volume_multiplier: 1.0        # Volume multiplier (0.8-1.5)
```

### musicPlaylistNumbers (global state object)
Tracks rotation position for each playback type:
```javascript
{
  "morning": 2,    // Next playlist to play (0-indexed)
  "day": 5,
  "evening": 1,
  "winddown": 0,
  "sleep": 1
}
```

### currentlyPlayingMusic (input_text.currently_playing_music - JSON)
Currently active playback info:
```javascript
{
  type: "day",                  // Current playback type
  uri: "spotify:playlist:...",  // What's playing
  media_type: "playlist",
  leadPlayer: "Kitchen",        // Primary speaker
  participants: [               // Active speakers
    {
      player_name: "Kitchen",
      base_volume: 9,
      volume: 9,                // Current calculated volume
      default_volume: 9,        // Reset to this after mute changes
      leave_muted_if: [...]
    },
    ...
  ]
}
```

## Function Node Details

### 1. **Set music type based on conditions** (ID: e461ac8aeac7cb0c)
**Purpose**: Determines appropriate music type based on time and presence
**Key Logic**:
- Returns "" if nobody is home
- Returns "sleep" if anyone is asleep
- Returns "morning" if it's morning/day and last person woke up (except Sunday)
- Returns "day" if daytime and nobody asleep
- Returns "evening" if sunset/dusk
- Returns "winddown" if evening/night
- Prevents sleep music re-triggering if already playing

### 2. **Build master music message** (ID: 2af7ad02c544ef24)
**Purpose**: Main orchestration node - prepares all playback instructions
**Key Logic**:
```javascript
1. Get musicPlaybackType and musicPlaylistNumbers
2. Increment playlist rotation (wraps around)
3. Calculate volume for each speaker:
   - volume = base_volume × volume_multiplier
   - Math.round() to nearest integer
4. Create comma-separated player list for grouping
5. Get first player as lead
6. Create 5 output messages:
   - playMsg: URI + lead player + group
   - currentlyPlayingMusic: Save state
   - muteMsg: Mute all players
   - incrementMsg: Update playlist numbers
   - stopMsg: Stop existing playback
7. Return all messages for downstream processing
```

### 3. **Prevent re-activation of already active music** (ID: 595573f8f9ee970d)
**Purpose**: Stops double-triggering the same music type
**Logic**:
- Compares incoming musicPlaybackType with currentlyPlayingMusic.type
- Returns null if already playing (blocks flow)
- Returns msg otherwise (allows flow)

### 4. **Check if non-null mode** (ID: 551ddc8702a7c82e)
**Purpose**: Branches flow for start vs. stop
**Logic**:
- If musicPlaybackType is not empty → send to playback path
- If empty → send to stop path

### 5. **Build or parse list of players** (ID: 9af76eda23a1c810)
**Purpose**: Converts player list to individual messages
**Logic**:
- Receives array of player names
- If error, reconstructs from full musicConfig
- Outputs one message per player (forEach)
- Each message: `{playerName: "Kitchen"}` for subsequent processing

### 6. **Create participant-specific message** (ID: d6f678177a40f732)
**Purpose**: Prepares per-speaker volume and mute configuration
**Logic**:
```javascript
1. Default target = all speakers in currentlyPlayingMusic
2. If specific target provided, downscope to one speaker
3. For each target speaker:
   - Extract player_name, desired volume
   - Copy leave_muted_if conditions
   - Create separate message for each
4. Each downstream node handles one speaker
```

### 7. **Apply unmute criteria** (ID: 05e5600da5d5942c)
**Purpose**: Determines if speaker should stay muted
**Logic**:
```javascript
1. Check if speaker has "leave_muted_if" criteria
2. For each criterion:
   - Check if state variable matches condition
   - If yes → msg.payload = "on" (MUTE)
   - Return early
3. If no criteria matched → msg.payload = "off" (UNMUTE)
```

**Example**: Kitchen speaker with:
```yaml
leave_muted_if:
  - variable: isTVPlaying
    value: true
```
If isTVPlaying == true → Kitchen stays muted

### 8. **For all speakers generate volume 0 msg** (ID: 04d8ead9f50cc124)
**Purpose**: Stop playback - sets all speakers to volume 0
**Logic**:
- Iterates all speakers in entire musicConfig
- Outputs: `{playerName: "...", payload: 0}` for each

### 9. **Branch out volume turn ups** (ID: 06b5888b90359103)
**Purpose**: Implements fade-in with 1 step per message
**Logic**:
```javascript
1. Get current_volume and desired_volume
2. If current < desired:
   - Increment by 1
   - Send to two outputs:
     a) To Sonos speaker (volume set)
     b) To delay/repeat loop (continue fade)
3. If current >= desired:
   - Send final message to first output only
   - null to second output (break loop)
```

### 10. **Repeat turn ups until done** (ID: 9637e0e343385196)
**Purpose**: Continues fade-in loop, checks for interference
**Logic**:
```javascript
1. Get current_volume from speaker
2. Check for manual volume changes:
   - If current < last_set → someone fighting system
   - Stop fade-in
3. Check for music type change:
   - If starting_type != current_type
   - Stop fade-in
4. If current < desired:
   - Calculate next volume
   - Calculate delay: (100 - current) × 250ms
   - Return msg for next iteration
5. If current >= desired:
   - Done fading in
```

### 11. **Determine if this variable is relevant** (ID: a78f9276bce4a401)
**Purpose**: Handles real-time mute state updates
**Logic**:
```javascript
1. Get actively playing speakers from currentlyPlayingMusic
2. For each active speaker:
   - Check if it has leave_muted_if criteria
   - Check if changed variable matches any criteria
   - If yes → output message to re-evaluate mute state
3. Purpose: If TV starts playing while music playing
   → Triggers speaker re-mute if configured
```

### 12. **Modify music playback config** (ID: 1400bc1a11781043)
**Purpose**: Custom volume adjustments for specific speakers
**Logic**:
```javascript
// Special handling for Bedroom and Kitchen
var target_speakers = ["Bedroom", "Kitchen"]
participants.forEach(p => {
  if (target_speakers.includes(p.player_name)) {
    p.volume = p.volume + 10  // +10 boost
  }
})
```

### 13. **Compare desired with active groups** (ID: 6c1e2890d2cb3628)
**Purpose**: Verifies speaker group was created correctly
**Logic**:
```javascript
1. Search active Sonos groups for leader with matching name
2. Count matches between expected players and active group
3. If matches != expected count:
   - Group creation failed
   - Output participants to retry grouping
4. If matches == expected count:
   - Group verified correct
   - Continue to playback
```

### 14. **Reset volumes to defaults** (ID: 2c7306185576189a)
**Purpose**: Restores volume to config defaults after changes
**Logic**:
```javascript
1. Get currentlyPlayingMusic config
2. For each participant:
   - If volume != default_volume:
     - Reset volume to default_volume
3. Return updated config
```

### 15. **If sleep music playing** (ID: 16264ecde9053b53)
**Purpose**: Gate for sleep music special handling
**Logic**:
- Only passes message if musicPlaybackType == "sleep"
- Otherwise blocks (returns null)

## Volume Calculation Algorithm

```
final_volume = round(base_volume × volume_multiplier)
```

**Examples from config**:
- Morning Kitchen: 9 × 1.0 = 9
- Evening Bedroom: 10 × 1.5 = 15
- Sleep Bedroom: 16 × 1.1 = 18 → capped at 15

**Base volumes** vary by speaker and context:
- Bedroom: 6-16 (varies by mode)
- Kitchen: 9-10
- Soundbar: 10
- Office: 6-8
- Dining Room: 7-9
- Kids Bathroom: 7-8

## Muting Logic

### Static Mute Conditions (leave_muted_if)
Defined per speaker per music type in config:

**Example - Morning mode**:
- Kitchen: Never muted (leave_muted_if: [])
- Soundbar: Muted if guest asleep
- Kids Bathroom: Muted if TV playing
- Bedroom: Muted if master asleep

### Dynamic Mute Updates
When state variables change during playback:
1. "Determine if this variable is relevant" node checks if variable affects current speakers
2. If affected → re-evaluate "Apply unmute criteria"
3. Updates mute state immediately

### Mute/Unmute Flow
```
musicPlaybackType change
    ↓
Build master music message (mute all)
    ↓
Create participant-specific message
    ↓
Apply unmute criteria (unmute if conditions allow)
    ↓
Set volume on speaker
```

## Playlist Rotation Logic

**Data**: musicPlaylistNumbers object tracks next index for each type
```javascript
// Initialize from config
if (musicType in musicPlaylistNumbers) {
    thisPlaylistNumber = musicPlaylistNumbers[musicType]
} else {
    thisPlaylistNumber = 0
    musicPlaylistNumbers[musicType] = 0
}

// Get playback option
var musicToPlay = musicConfig[musicType]["playback_options"][thisPlaylistNumber]

// Increment for next time (with wraparound)
if ((musicPlaylistNumbers[musicType] + 1) >= musicConfig[musicType]["playback_options"].length) {
    musicPlaylistNumbers[musicType] = 0  // Wrap around
} else {
    musicPlaylistNumbers[musicType] = musicPlaylistNumbers[musicType] + 1
}
```

## Fade-In Implementation

**Gradual volume ramp-up** prevents audio shock:

1. All speakers start at 0 (muted initially)
2. Branch out volume turn ups: increment current by 1
3. Repeat turn ups until done: loop back with delay
4. Delay: `(100 - current_volume) × 250ms`
   - Lower volume = longer between steps (fade smoother)
   - Higher volume = shorter delay (speeds up)
5. Checks every iteration:
   - Has user manually changed volume? (stop if fighting)
   - Has music type changed? (stop if switched music)
   - Have we reached target? (stop if done)

**Example**: Fade 0→9
- Step 1: 0→1 (250×100 = 25s delay)
- Step 2: 1→2 (250×99 = 24.75s delay)
- ...
- Step 8: 8→9 (250×92 = 23s delay)
- Stop at 9

## Rate Limiting

**Max 1 playback per 10 seconds** implemented via Delay nodes:
- Delay nodes with rate: "10" means max 10 messages per second
- Multiple delay nodes in series provide cumulative throttling
- Prevents Sonos speaker overload when manually retriggering

## Entry Points for Playback

1. **Automatic (Timer-based)**
   - "Set music type based on conditions" → evaluates current time + presence
   - Triggers on state changes: dayPhase, isAnyoneHome, isAnyoneAsleep

2. **Manual Inject Nodes**
   - Make Morning/Day/Evening/Winddown/Sleep Playback (direct selection)
   - Manually reactivate music (resume last type)
   - No Music Playback (stop all)
   - Force volume reset (restore defaults after manual changes)

## State Variables Required

**From Home Assistant input_text entities**:
- musicPlaybackType: Current mode (morning|day|evening|winddown|sleep|wakeup|sex|"")
- musicConfig: Loaded from YAML config file
- currentlyPlayingMusic: Currently active playback (JSON)
- musicPlaylistNumbers: Playlist rotation state (JSON)

**From Home Assistant booleans/text**:
- isAnyoneHome, isAnyoneAsleep, isMasterAsleep, isGuestAsleep
- isTVPlaying, isNickOfficeOccupied
- dayPhase: morning|day|sunset|dusk|winddown|night
- Any custom leave_muted_if criteria variables

## Concurrency & Thread Safety Considerations

**For Go Implementation**:
1. All state reads/writes must be protected by sync.RWMutex
2. WebSocket writes must be serialized (use writeMu)
3. Playlist rotation counter must be atomic (CAS or mutex)
4. Mute state changes must not interfere with fade-in
5. Music type changes must gracefully stop existing fade-in
6. Volume adjustment loops must check for user interference
7. Speaker grouping must be atomic (group creation + playback)

## Summary of Key Patterns

| Pattern | Implementation | Go Equivalent |
|---------|-----------------|---------------|
| Async flow | Node-RED wire connections | Goroutines + channels |
| State storage | global.get("state") | sync.RWMutex protected map |
| Conditional logic | Switch nodes | if/switch statements |
| Looping | delay + feedback wire | for loops with time.Ticker |
| Rate limiting | Delay nodes | time.Ticker with max rate |
| Type checking | JavaScript typeof | Go type assertions |
| Array iteration | JavaScript forEach | Go range loops |
| JSON handling | JSON.stringify/parse | encoding/json marshal/unmarshal |

