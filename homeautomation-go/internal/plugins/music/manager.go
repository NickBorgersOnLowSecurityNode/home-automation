package music

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// CurrentlyPlayingMusic represents the currently active music playback
type CurrentlyPlayingMusic struct {
	Type         string                  `json:"type"`
	URI          string                  `json:"uri"`
	MediaType    string                  `json:"media_type"`
	LeadPlayer   string                  `json:"leadPlayer"`
	Participants []ParticipantWithVolume `json:"participants"`
}

// ParticipantWithVolume represents a speaker with calculated volume
type ParticipantWithVolume struct {
	PlayerName    string          `json:"player_name"`
	BaseVolume    int             `json:"base_volume"`
	Volume        int             `json:"volume"`
	DefaultVolume int             `json:"default_volume"`
	LeaveMutedIf  []MuteCondition `json:"leave_muted_if"`
}

// TimeProvider is an interface for getting the current time
// This allows tests to inject a fixed time instead of using time.Now()
type TimeProvider interface {
	Now() time.Time
}

// RealTimeProvider returns the actual current time
type RealTimeProvider struct{}

func (r RealTimeProvider) Now() time.Time {
	return time.Now()
}

// FixedTimeProvider returns a fixed time (for testing)
type FixedTimeProvider struct {
	FixedTime time.Time
}

func (f FixedTimeProvider) Now() time.Time {
	return f.FixedTime
}

// Manager handles music mode selection and playback coordination
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	config       *MusicConfig
	logger       *zap.Logger
	readOnly     bool
	timeProvider TimeProvider

	// Playback state
	playlistNumbers    map[string]int // Tracks playlist rotation per music type
	currentlyPlaying   *CurrentlyPlayingMusic
	lastPlaybackTime   time.Time
	playbackInProgress bool
	mu                 sync.RWMutex // Protects playback state

	// Subscriptions for cleanup
	subscriptions []state.Subscription
}

// NewManager creates a new Music manager
// If timeProvider is nil, it defaults to RealTimeProvider
func NewManager(haClient ha.HAClient, stateManager *state.Manager, config *MusicConfig, logger *zap.Logger, readOnly bool, timeProvider TimeProvider) *Manager {
	if timeProvider == nil {
		timeProvider = RealTimeProvider{}
	}
	return &Manager{
		haClient:           haClient,
		stateManager:       stateManager,
		config:             config,
		logger:             logger.Named("music"),
		readOnly:           readOnly,
		timeProvider:       timeProvider,
		playlistNumbers:    make(map[string]int),
		subscriptions:      make([]state.Subscription, 0),
		playbackInProgress: false,
	}
}

// Start begins monitoring state changes and managing music playback
func (m *Manager) Start() error {
	m.logger.Info("Starting Music Manager")

	// Subscribe to dayPhase changes
	sub, err := m.stateManager.Subscribe("dayPhase", m.handleStateChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to dayPhase: %w", err)
	}
	m.subscriptions = append(m.subscriptions, sub)

	// Subscribe to isAnyoneAsleep changes
	sub, err = m.stateManager.Subscribe("isAnyoneAsleep", m.handleStateChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to isAnyoneAsleep: %w", err)
	}
	m.subscriptions = append(m.subscriptions, sub)

	// Subscribe to isAnyoneHome changes
	sub, err = m.stateManager.Subscribe("isAnyoneHome", m.handleStateChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to isAnyoneHome: %w", err)
	}
	m.subscriptions = append(m.subscriptions, sub)

	// Subscribe to musicPlaybackType changes to trigger actual playback
	sub, err = m.stateManager.Subscribe("musicPlaybackType", m.handleMusicPlaybackTypeChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to musicPlaybackType: %w", err)
	}
	m.subscriptions = append(m.subscriptions, sub)

	// Perform initial music mode selection
	m.selectAppropriateMusicMode()

	m.logger.Info("Music Manager started successfully")
	return nil
}

// Stop stops the Music Manager and cleans up subscriptions
func (m *Manager) Stop() {
	m.logger.Info("Stopping Music Manager")

	// Unsubscribe from all subscriptions
	for _, sub := range m.subscriptions {
		sub.Unsubscribe()
	}
	m.subscriptions = nil

	m.logger.Info("Music Manager stopped")
}

// handleStateChange processes state changes that should trigger music mode re-evaluation
func (m *Manager) handleStateChange(key string, oldValue, newValue interface{}) {
	m.logger.Debug("State change detected",
		zap.String("key", key),
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	// Re-evaluate music mode
	m.selectAppropriateMusicMode()
}

// selectAppropriateMusicMode determines which music mode should be active (without trigger context)
func (m *Manager) selectAppropriateMusicMode() {
	m.selectAppropriateMusicModeWithContext("", false)
}

// selectAppropriateMusicModeWithContext determines which music mode should be active with trigger context
func (m *Manager) selectAppropriateMusicModeWithContext(triggerKey string, isWakeUpEvent bool) {
	m.logger.Debug("Selecting appropriate music mode",
		zap.String("trigger_key", triggerKey),
		zap.Bool("is_wake_up_event", isWakeUpEvent))

	// Get current state
	isAnyoneHome, err := m.stateManager.GetBool("isAnyoneHome")
	if err != nil {
		m.logger.Error("Failed to get isAnyoneHome", zap.Error(err))
		return
	}

	// If no one is home, stop music
	if !isAnyoneHome {
		m.logger.Info("No one is home, stopping music")
		if err := m.setMusicPlaybackType(""); err != nil {
			if errors.Is(err, state.ErrReadOnlyMode) {
				m.logger.Debug("Skipping music playback type update in read-only mode",
					zap.String("music_type", ""))
			} else {
				m.logger.Error("Failed to set empty music playback type", zap.Error(err))
			}
		}
		return
	}

	// Check if anyone is asleep - sleep mode has highest priority
	isAnyoneAsleep, err := m.stateManager.GetBool("isAnyoneAsleep")
	if err != nil {
		m.logger.Error("Failed to get isAnyoneAsleep", zap.Error(err))
		return
	}

	if isAnyoneAsleep {
		m.logger.Info("Someone is asleep, selecting sleep mode")
		if err := m.setMusicPlaybackType("sleep"); err != nil {
			if errors.Is(err, state.ErrReadOnlyMode) {
				m.logger.Debug("Skipping music playback type update in read-only mode",
					zap.String("music_type", "sleep"))
			} else {
				m.logger.Error("Failed to set sleep music playback type", zap.Error(err))
			}
		}
		return
	}

	// Get current day phase
	dayPhase, err := m.stateManager.GetString("dayPhase")
	if err != nil {
		m.logger.Error("Failed to get dayPhase", zap.Error(err))
		return
	}

	// Get current music playback type to check for persistence
	currentMusicType, err := m.stateManager.GetString("musicPlaybackType")
	if err != nil {
		m.logger.Error("Failed to get musicPlaybackType", zap.Error(err))
		return
	}

	// Determine music mode based on day phase and trigger context
	musicMode := m.determineMusicModeFromDayPhase(dayPhase, currentMusicType, triggerKey, isWakeUpEvent)

	m.logger.Info("Selected music mode",
		zap.String("day_phase", dayPhase),
		zap.String("current_music_type", currentMusicType),
		zap.String("trigger_key", triggerKey),
		zap.Bool("is_wake_up_event", isWakeUpEvent),
		zap.String("new_music_mode", musicMode))

	// Set the music playback type
	if err := m.setMusicPlaybackType(musicMode); err != nil {
		if errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Debug("Skipping music playback type update in read-only mode",
				zap.String("music_type", musicMode))
		} else {
			m.logger.Error("Failed to set music playback type", zap.Error(err))
		}
	}
}

// determineMusicModeFromDayPhase determines the music mode based on the current day phase
// Matches Node-RED behavior: morning music only plays on wake-up events
func (m *Manager) determineMusicModeFromDayPhase(dayPhase string, currentMusicType string, triggerKey string, isWakeUpEvent bool) string {
	switch dayPhase {
	case "morning":
		// Morning music ONLY plays when someone wakes up (matches Node-RED)
		// Otherwise, fall back to day music during morning phase
		if isWakeUpEvent {
			// Check if it's Sunday (no morning music on Sundays)
			if m.timeProvider.Now().Weekday() == time.Sunday {
				m.logger.Debug("Sunday detected, using day mode instead of morning")
				return "day"
			}
			m.logger.Info("Wake-up event during morning phase, playing morning music")
			return "morning"
		}
		// During morning phase but not a wake-up event - use day music
		m.logger.Debug("Morning phase but not a wake-up event, using day music")
		return "day"

	case "day":
		return "day"

	case "sunset", "dusk":
		return "evening"

	case "winddown", "night":
		// Don't override sleep music with winddown
		if currentMusicType == "sleep" {
			m.logger.Debug("Sleep music already playing, not changing to winddown")
			return "sleep"
		}
		return "winddown"

	default:
		m.logger.Warn("Unknown day phase, defaulting to day mode",
			zap.String("day_phase", dayPhase))
		return "day"
	}
}

// setMusicPlaybackType updates the musicPlaybackType state variable
func (m *Manager) setMusicPlaybackType(musicType string) error {
	// Get current type to check if it's actually changing
	currentType, err := m.stateManager.GetString("musicPlaybackType")
	if err != nil {
		return fmt.Errorf("failed to get current music playback type: %w", err)
	}

	// Only update if it's different
	if currentType == musicType {
		m.logger.Debug("Music playback type unchanged",
			zap.String("type", musicType))
		return nil
	}

	m.logger.Info("Changing music playback type",
		zap.String("from", currentType),
		zap.String("to", musicType))

	// Update the state variable
	if err := m.stateManager.SetString("musicPlaybackType", musicType); err != nil {
		return fmt.Errorf("failed to set music playback type: %w", err)
	}

	return nil
}

// handleMusicPlaybackTypeChange is called when musicPlaybackType changes
// This triggers actual music playback orchestration
func (m *Manager) handleMusicPlaybackTypeChange(key string, oldValue, newValue interface{}) {
	m.logger.Info("Music playback type changed, initiating playback",
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	newType, ok := newValue.(string)
	if !ok {
		m.logger.Error("Invalid musicPlaybackType value type")
		return
	}

	// Check rate limiting (max 1 playback per 10 seconds)
	m.mu.Lock()
	timeSinceLastPlayback := m.timeProvider.Now().Sub(m.lastPlaybackTime)
	if timeSinceLastPlayback < 10*time.Second && !m.lastPlaybackTime.IsZero() {
		m.mu.Unlock()
		m.logger.Warn("Rate limiting: playback too soon after last playback",
			zap.Duration("time_since_last", timeSinceLastPlayback))
		return
	}
	m.lastPlaybackTime = m.timeProvider.Now()
	m.mu.Unlock()

	// Prevent re-activation of already playing music
	m.mu.RLock()
	if m.currentlyPlaying != nil && m.currentlyPlaying.Type == newType && newType != "" {
		m.mu.RUnlock()
		m.logger.Debug("Double activation of already-playing musicType, ignoring",
			zap.String("type", newType))
		return
	}
	m.mu.RUnlock()

	// If empty string, stop playback
	if newType == "" {
		m.logger.Info("Stopping music playback")
		m.stopPlayback()
		return
	}

	// Start playback orchestration
	if err := m.orchestratePlayback(newType); err != nil {
		m.logger.Error("Failed to orchestrate playback",
			zap.String("type", newType),
			zap.Error(err))
	}
}

// stopPlayback stops all music playback
func (m *Manager) stopPlayback() {
	m.mu.Lock()
	m.currentlyPlaying = nil
	m.mu.Unlock()

	// Clear the currently playing music URI in Home Assistant
	if err := m.stateManager.SetString("currentlyPlayingMusicUri", ""); err != nil {
		if !errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Error("Failed to clear currently playing music URI", zap.Error(err))
		}
	}

	if m.readOnly {
		m.logger.Debug("Skipping playback stop in read-only mode")
		return
	}

	// Set all speakers to volume 0
	for _, mode := range m.config.Music {
		for _, participant := range mode.Participants {
			entityID := m.getSpeakerEntityID(participant.PlayerName)
			if err := m.callService("media_player", "volume_set", map[string]interface{}{
				"entity_id":    entityID,
				"volume_level": 0,
			}); err != nil {
				m.logger.Error("Failed to set speaker volume to 0",
					zap.String("speaker", participant.PlayerName),
					zap.Error(err))
			}
		}
	}

	m.logger.Info("Music playback stopped")
}

// orchestratePlayback coordinates the complete playback flow
func (m *Manager) orchestratePlayback(musicType string) error {
	m.logger.Info("Orchestrating playback", zap.String("type", musicType))

	// Get the music mode configuration
	mode, ok := m.config.Music[musicType]
	if !ok {
		return fmt.Errorf("unknown music type: %s", musicType)
	}

	// Select playlist with rotation
	playlistIndex := m.getNextPlaylistIndex(musicType, len(mode.PlaybackOptions))
	playbackOption := mode.PlaybackOptions[playlistIndex]

	m.logger.Info("Selected playlist",
		zap.String("type", musicType),
		zap.Int("playlist_index", playlistIndex),
		zap.String("uri", playbackOption.URI),
		zap.Float64("volume_multiplier", playbackOption.VolumeMultiplier))

	// Set the currently playing music URI in Home Assistant
	if err := m.stateManager.SetString("currentlyPlayingMusicUri", playbackOption.URI); err != nil {
		if errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Debug("Skipping URI update in read-only mode",
				zap.String("uri", playbackOption.URI))
		} else {
			m.logger.Error("Failed to set currently playing music URI",
				zap.String("uri", playbackOption.URI),
				zap.Error(err))
		}
	}

	// Build participants with calculated volumes
	participants := make([]ParticipantWithVolume, 0, len(mode.Participants))
	for _, p := range mode.Participants {
		volume := m.calculateVolume(p.BaseVolume, playbackOption.VolumeMultiplier)
		participants = append(participants, ParticipantWithVolume{
			PlayerName:    p.PlayerName,
			BaseVolume:    p.BaseVolume,
			Volume:        volume,
			DefaultVolume: volume,
			LeaveMutedIf:  p.LeaveMutedIf,
		})
	}

	// Get lead player (first participant)
	if len(participants) == 0 {
		return fmt.Errorf("no participants for music type: %s", musicType)
	}
	leadPlayer := participants[0].PlayerName

	// Update currently playing state
	m.mu.Lock()
	m.currentlyPlaying = &CurrentlyPlayingMusic{
		Type:         musicType,
		URI:          playbackOption.URI,
		MediaType:    playbackOption.MediaType,
		LeadPlayer:   leadPlayer,
		Participants: participants,
	}
	m.mu.Unlock()

	if m.readOnly {
		m.logger.Info("Read-only mode: would start playback",
			zap.String("type", musicType),
			zap.String("lead_player", leadPlayer),
			zap.Int("participant_count", len(participants)))
		return nil
	}

	// Execute playback sequence
	if err := m.executePlayback(musicType, playbackOption, participants, leadPlayer); err != nil {
		return fmt.Errorf("failed to execute playback: %w", err)
	}

	return nil
}

// getNextPlaylistIndex returns the next playlist index with rotation
func (m *Manager) getNextPlaylistIndex(musicType string, optionsCount int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get current index or initialize to 0
	currentIndex, exists := m.playlistNumbers[musicType]
	if !exists {
		currentIndex = 0
	}

	// Save the index to use
	indexToUse := currentIndex

	// Increment for next time (with wraparound)
	nextIndex := currentIndex + 1
	if nextIndex >= optionsCount {
		nextIndex = 0
	}
	m.playlistNumbers[musicType] = nextIndex

	return indexToUse
}

// calculateVolume calculates final volume from base and multiplier
func (m *Manager) calculateVolume(baseVolume int, multiplier float64) int {
	volume := math.Round(float64(baseVolume) * multiplier)
	// Cap at 15 (Sonos max for Spotify playback scale)
	if volume > 15 {
		volume = 15
	}
	if volume < 0 {
		volume = 0
	}
	return int(volume)
}

// executePlayback executes the actual playback sequence
func (m *Manager) executePlayback(musicType string, option PlaybackOption, participants []ParticipantWithVolume, leadPlayer string) error {
	m.logger.Info("Executing playback sequence",
		zap.String("type", musicType),
		zap.String("lead_player", leadPlayer),
		zap.Int("participant_count", len(participants)))

	leadEntityID := m.getSpeakerEntityID(leadPlayer)

	// Step 1: Build speaker group if multiple participants
	if len(participants) > 1 {
		if err := m.buildSpeakerGroup(participants, leadEntityID); err != nil {
			return fmt.Errorf("failed to build speaker group: %w", err)
		}
	}

	// Step 2: Mute all speakers initially
	for _, p := range participants {
		entityID := m.getSpeakerEntityID(p.PlayerName)
		if err := m.callService("media_player", "volume_set", map[string]interface{}{
			"entity_id":    entityID,
			"volume_level": 0,
		}); err != nil {
			m.logger.Error("Failed to mute speaker",
				zap.String("speaker", p.PlayerName),
				zap.Error(err))
		}
	}

	// Step 3: Start playback on lead player
	if err := m.callService("media_player", "play_media", map[string]interface{}{
		"entity_id":          leadEntityID,
		"media_content_id":   option.URI,
		"media_content_type": option.MediaType,
	}); err != nil {
		return fmt.Errorf("failed to start playback: %w", err)
	}

	// Step 4: Enable shuffle for Spotify playlists
	if option.MediaType == "playlist" {
		if err := m.callService("media_player", "shuffle_set", map[string]interface{}{
			"entity_id": leadEntityID,
			"shuffle":   true,
		}); err != nil {
			m.logger.Warn("Failed to enable shuffle",
				zap.String("speaker", leadPlayer),
				zap.Error(err))
		}
	}

	// Step 5: Evaluate mute conditions and unmute eligible speakers
	for _, p := range participants {
		if m.shouldUnmuteSpeaker(p) {
			m.logger.Info("Unmuting speaker",
				zap.String("speaker", p.PlayerName),
				zap.Int("target_volume", p.Volume))

			// Start fade-in in goroutine
			go m.fadeInSpeaker(p.PlayerName, p.Volume, musicType)
		} else {
			m.logger.Info("Keeping speaker muted due to conditions",
				zap.String("speaker", p.PlayerName))
		}
	}

	m.logger.Info("Playback sequence completed successfully",
		zap.String("type", musicType))

	return nil
}

// buildSpeakerGroup creates a Sonos speaker group
func (m *Manager) buildSpeakerGroup(participants []ParticipantWithVolume, leadEntityID string) error {
	m.logger.Info("Building speaker group", zap.Int("count", len(participants)))

	// Join all other speakers to the lead
	for i, p := range participants {
		if i == 0 {
			// Skip lead player
			continue
		}

		entityID := m.getSpeakerEntityID(p.PlayerName)
		if err := m.callService("media_player", "join", map[string]interface{}{
			"entity_id":     entityID,
			"group_members": []string{leadEntityID},
		}); err != nil {
			m.logger.Error("Failed to join speaker to group",
				zap.String("speaker", p.PlayerName),
				zap.Error(err))
		}
	}

	// Wait for group to stabilize
	time.Sleep(500 * time.Millisecond)

	return nil
}

// shouldUnmuteSpeaker determines if a speaker should be unmuted based on conditions
func (m *Manager) shouldUnmuteSpeaker(participant ParticipantWithVolume) bool {
	// If no mute conditions, always unmute
	if len(participant.LeaveMutedIf) == 0 {
		return true
	}

	// Check each mute condition
	for _, condition := range participant.LeaveMutedIf {
		// Get the state variable value
		value, err := m.getStateValue(condition.Variable)
		if err != nil {
			m.logger.Error("Failed to get state variable for mute condition",
				zap.String("variable", condition.Variable),
				zap.Error(err))
			continue
		}

		// Check if condition matches (should stay muted)
		if m.valuesMatch(value, condition.Value) {
			m.logger.Debug("Mute condition matched",
				zap.String("variable", condition.Variable),
				zap.Any("value", value),
				zap.Any("condition", condition.Value))
			return false // Stay muted
		}
	}

	// No conditions matched, unmute
	return true
}

// fadeInSpeaker gradually increases speaker volume
func (m *Manager) fadeInSpeaker(speakerName string, targetVolume int, startingMusicType string) {
	m.logger.Debug("Starting fade-in",
		zap.String("speaker", speakerName),
		zap.Int("target_volume", targetVolume))

	entityID := m.getSpeakerEntityID(speakerName)

	// Gradual fade-in: 0 → targetVolume
	for currentVolume := 0; currentVolume <= targetVolume; currentVolume++ {
		// Check if music type changed (stop fade if switched)
		musicType, err := m.stateManager.GetString("musicPlaybackType")
		if err == nil && musicType != startingMusicType {
			m.logger.Info("Music type changed during fade-in, stopping",
				zap.String("speaker", speakerName),
				zap.String("starting_type", startingMusicType),
				zap.String("current_type", musicType))
			return
		}

		// Set volume
		if err := m.callService("media_player", "volume_set", map[string]interface{}{
			"entity_id":    entityID,
			"volume_level": float64(currentVolume) / 15.0, // Normalize to 0.0-1.0
		}); err != nil {
			m.logger.Error("Failed to set volume during fade-in",
				zap.String("speaker", speakerName),
				zap.Int("volume", currentVolume),
				zap.Error(err))
		}

		// Adaptive delay: slower at start, faster as volume increases
		// Matches Node-RED: (100 - current) * 250ms, but scaled for our 0-15 range
		delayMs := (100 - (currentVolume * 100 / 15)) * 2 // ~2ms per point
		if delayMs < 100 {
			delayMs = 100 // Minimum 100ms between steps
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	m.logger.Info("Fade-in completed",
		zap.String("speaker", speakerName),
		zap.Int("final_volume", targetVolume))
}

// getSpeakerEntityID converts speaker name to Home Assistant entity ID
func (m *Manager) getSpeakerEntityID(speakerName string) string {
	// Convert "Kitchen" to "media_player.kitchen"
	// Simple conversion - assumes lowercase, spaces to underscores
	entityName := ""
	for _, char := range speakerName {
		if char == ' ' {
			entityName += "_"
		} else {
			entityName += string(char)
		}
	}
	// Convert to lowercase
	entityName = toLower(entityName)
	return "media_player." + entityName
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := ""
	for _, char := range s {
		if char >= 'A' && char <= 'Z' {
			result += string(char + 32)
		} else {
			result += string(char)
		}
	}
	return result
}

// getStateValue gets a state variable value by key
func (m *Manager) getStateValue(key string) (interface{}, error) {
	// Try as boolean first
	if val, err := m.stateManager.GetBool(key); err == nil {
		return val, nil
	}

	// Try as string
	if val, err := m.stateManager.GetString(key); err == nil {
		return val, nil
	}

	// Try as number
	if val, err := m.stateManager.GetNumber(key); err == nil {
		return val, nil
	}

	return nil, fmt.Errorf("failed to get state variable: %s", key)
}

// valuesMatch checks if two values match
func (m *Manager) valuesMatch(a, b interface{}) bool {
	// Simple equality check
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// callService calls a Home Assistant service
func (m *Manager) callService(domain, service string, serviceData map[string]interface{}) error {
	if m.readOnly {
		m.logger.Debug("Read-only mode: would call service",
			zap.String("domain", domain),
			zap.String("service", service),
			zap.Any("service_data", serviceData))
		return nil
	}

	m.logger.Debug("Calling HA service",
		zap.String("domain", domain),
		zap.String("service", service),
		zap.Any("service_data", serviceData))

	// Call the service via HA client
	if err := m.haClient.CallService(domain, service, serviceData); err != nil {
		return fmt.Errorf("service call failed: %w", err)
	}

	return nil
}
