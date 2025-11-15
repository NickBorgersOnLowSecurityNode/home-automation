package music

import (
	"fmt"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Manager handles music mode selection and playback coordination
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	config       *MusicConfig
	logger       *zap.Logger
	readOnly     bool
}

// NewManager creates a new Music manager
func NewManager(haClient ha.HAClient, stateManager *state.Manager, config *MusicConfig, logger *zap.Logger, readOnly bool) *Manager {
	return &Manager{
		haClient:     haClient,
		stateManager: stateManager,
		config:       config,
		logger:       logger.Named("music"),
		readOnly:     readOnly,
	}
}

// Start begins monitoring state changes and managing music playback
func (m *Manager) Start() error {
	m.logger.Info("Starting Music Manager")

	// Subscribe to dayPhase changes
	if _, err := m.stateManager.Subscribe("dayPhase", m.handleStateChange); err != nil {
		return fmt.Errorf("failed to subscribe to dayPhase: %w", err)
	}

	// Subscribe to isAnyoneAsleep changes
	if _, err := m.stateManager.Subscribe("isAnyoneAsleep", m.handleStateChange); err != nil {
		return fmt.Errorf("failed to subscribe to isAnyoneAsleep: %w", err)
	}

	// Subscribe to isAnyoneHome changes
	if _, err := m.stateManager.Subscribe("isAnyoneHome", m.handleStateChange); err != nil {
		return fmt.Errorf("failed to subscribe to isAnyoneHome: %w", err)
	}

	// Perform initial music mode selection
	m.selectAppropriateMusicMode()

	m.logger.Info("Music Manager started successfully")
	return nil
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

// selectAppropriateMusicMode determines which music mode should be active
func (m *Manager) selectAppropriateMusicMode() {
	m.logger.Debug("Selecting appropriate music mode")

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
			m.logger.Error("Failed to set empty music playback type", zap.Error(err))
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
			m.logger.Error("Failed to set sleep music playback type", zap.Error(err))
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

	// Determine music mode based on day phase
	musicMode := m.determineMusicModeFromDayPhase(dayPhase, currentMusicType)

	m.logger.Info("Selected music mode",
		zap.String("day_phase", dayPhase),
		zap.String("current_music_type", currentMusicType),
		zap.String("new_music_mode", musicMode))

	// Set the music playback type
	if err := m.setMusicPlaybackType(musicMode); err != nil {
		m.logger.Error("Failed to set music playback type", zap.Error(err))
	}
}

// determineMusicModeFromDayPhase determines the music mode based on the current day phase
func (m *Manager) determineMusicModeFromDayPhase(dayPhase string, currentMusicType string) string {
	switch dayPhase {
	case "morning":
		// Check if it's Sunday (no morning music on Sundays)
		if time.Now().Weekday() == time.Sunday {
			m.logger.Debug("Sunday detected, using day mode instead of morning")
			return "day"
		}
		return "morning"

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
