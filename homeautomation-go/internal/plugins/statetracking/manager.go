package statetracking

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

// Manager handles automatic computation of derived state variables.
// This plugin implements the logic from Node-RED's "State Tracking" flow.
//
// Derived states computed:
//   - isAnyOwnerHome = isNickHome OR isCarolineHome
//   - isAnyoneHome = isAnyOwnerHome OR isToriHere
//   - isAnyoneAsleep = isMasterAsleep OR isGuestAsleep
//   - isEveryoneAsleep = isMasterAsleep AND isGuestAsleep
//
// Additional features:
//   - Automatic master sleep detection when primary suite lights off for 1 minute
//   - Automatic master wake detection when bedroom door open for 20 seconds
//   - Automatic guest sleep detection when guest bedroom door closes
type Manager struct {
	haClient     ha.HAClient
	stateManager *state.Manager
	logger       *zap.Logger
	readOnly     bool
	helper       *state.DerivedStateHelper

	// Subscriptions for cleanup
	haSubscriptions []ha.Subscription

	// Timers for sleep/wake detection
	masterSleepTimer *time.Timer
	masterWakeTimer  *time.Timer
	timerMutex       sync.Mutex
}

// NewManager creates a new State Tracking manager
func NewManager(haClient ha.HAClient, stateManager *state.Manager, logger *zap.Logger, readOnly bool) *Manager {
	return &Manager{
		haClient:        haClient,
		stateManager:    stateManager,
		logger:          logger.Named("statetracking"),
		readOnly:        readOnly,
		haSubscriptions: make([]ha.Subscription, 0),
	}
}

// Start begins computing and maintaining derived states.
// This must be called before other plugins that depend on derived states (Music, Security).
func (m *Manager) Start() error {
	m.logger.Info("Starting State Tracking Manager")

	// Create and start the derived state helper
	m.helper = state.NewDerivedStateHelper(m.stateManager, m.logger)
	if err := m.helper.Start(); err != nil {
		return fmt.Errorf("failed to start derived state helper: %w", err)
	}

	// Subscribe to primary suite lights for master sleep detection
	lightSub, err := m.haClient.SubscribeStateChanges("light.primary_suite", m.handlePrimarySuiteLightsChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to light.primary_suite: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, lightSub)

	// Subscribe to primary bedroom door for master wake detection
	doorSub, err := m.haClient.SubscribeStateChanges("input_boolean.primary_bedroom_door_open", m.handlePrimaryBedroomDoorChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to input_boolean.primary_bedroom_door_open: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, doorSub)

	// Subscribe to Nick's presence for arrival announcements
	nickSub, err := m.haClient.SubscribeStateChanges("input_boolean.nick_home", m.handleNickHomeChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to input_boolean.nick_home: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, nickSub)

	// Subscribe to Caroline's presence for arrival announcements
	carolineSub, err := m.haClient.SubscribeStateChanges("input_boolean.caroline_home", m.handleCarolineHomeChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to input_boolean.caroline_home: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, carolineSub)

	// Subscribe to Tori's presence for arrival announcements
	toriSub, err := m.haClient.SubscribeStateChanges("input_boolean.tori_here", m.handleToriHereChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to input_boolean.tori_here: %w", err)
	}
	m.haSubscriptions = append(m.haSubscriptions, toriSub)

	m.logger.Info("State Tracking Manager started successfully",
		zap.Strings("derivedStates", []string{
			"isAnyOwnerHome",
			"isAnyoneHome",
			"isAnyoneAsleep",
			"isEveryoneAsleep",
		}),
		zap.Strings("sleepDetection", []string{
			"light.primary_suite (1min off → asleep)",
			"input_boolean.primary_bedroom_door_open (20sec open → awake)",
		}),
		zap.Strings("presenceAnnouncements", []string{
			"input_boolean.nick_home (arrival → TTS)",
			"input_boolean.caroline_home (arrival → TTS)",
			"input_boolean.tori_here (arrival → TTS)",
		}))
	return nil
}

// Stop stops the State Tracking Manager and cleans up subscriptions
func (m *Manager) Stop() {
	m.logger.Info("Stopping State Tracking Manager")

	// Stop any active timers
	m.timerMutex.Lock()
	if m.masterSleepTimer != nil {
		m.masterSleepTimer.Stop()
		m.masterSleepTimer = nil
	}
	if m.masterWakeTimer != nil {
		m.masterWakeTimer.Stop()
		m.masterWakeTimer = nil
	}
	m.timerMutex.Unlock()

	// Unsubscribe from all HA subscriptions
	for _, sub := range m.haSubscriptions {
		sub.Unsubscribe()
	}
	m.haSubscriptions = nil

	if m.helper != nil {
		m.helper.Stop()
	}
	m.logger.Info("State Tracking Manager stopped")
}

// handlePrimarySuiteLightsChange processes primary suite lights state changes
func (m *Manager) handlePrimarySuiteLightsChange(entityID string, oldState, newState *ha.State) {
	if newState == nil {
		return
	}

	lightsOff := newState.State == "off"

	m.logger.Debug("Primary suite lights changed",
		zap.String("entity_id", entityID),
		zap.String("new_state", newState.State),
		zap.Bool("lights_off", lightsOff))

	m.timerMutex.Lock()
	defer m.timerMutex.Unlock()

	// Cancel existing sleep timer if any
	if m.masterSleepTimer != nil {
		m.masterSleepTimer.Stop()
		m.masterSleepTimer = nil
	}

	if lightsOff {
		// Start 1-minute timer for sleep detection
		m.logger.Debug("Primary suite lights turned off, starting 1-minute sleep detection timer")
		m.masterSleepTimer = time.AfterFunc(1*time.Minute, func() {
			m.detectMasterAsleep()
		})
	} else {
		m.logger.Debug("Primary suite lights turned on, canceling sleep detection")
	}
}

// detectMasterAsleep runs after lights have been off for 1 minute
func (m *Manager) detectMasterAsleep() {
	m.logger.Debug("1-minute timer expired, checking if should mark master asleep")

	// Check if anyone is home
	isAnyoneHome, err := m.stateManager.GetBool("isAnyoneHome")
	if err != nil {
		m.logger.Error("Failed to get isAnyoneHome", zap.Error(err))
		return
	}

	if !isAnyoneHome {
		m.logger.Debug("Nobody home, not marking master asleep")
		return
	}

	// Check if master is already asleep
	isMasterAsleep, err := m.stateManager.GetBool("isMasterAsleep")
	if err != nil {
		m.logger.Error("Failed to get isMasterAsleep", zap.Error(err))
		return
	}

	if isMasterAsleep {
		m.logger.Debug("Master already marked asleep, nothing to do")
		return
	}

	// All checks passed, mark master as asleep
	m.logger.Info("Marking master as asleep (lights off for 1 minute)")
	if err := m.stateManager.SetBool("isMasterAsleep", true); err != nil {
		if errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Debug("Skipping isMasterAsleep update in read-only mode")
		} else {
			m.logger.Error("Failed to set isMasterAsleep", zap.Error(err))
		}
	}
}

// handlePrimaryBedroomDoorChange processes primary bedroom door state changes
func (m *Manager) handlePrimaryBedroomDoorChange(entityID string, oldState, newState *ha.State) {
	if newState == nil {
		return
	}

	doorOpen := newState.State == "on"

	m.logger.Debug("Primary bedroom door changed",
		zap.String("entity_id", entityID),
		zap.String("new_state", newState.State),
		zap.Bool("door_open", doorOpen))

	m.timerMutex.Lock()
	defer m.timerMutex.Unlock()

	// Cancel existing wake timer if any
	if m.masterWakeTimer != nil {
		m.masterWakeTimer.Stop()
		m.masterWakeTimer = nil
	}

	if doorOpen {
		// Start 20-second timer for wake detection
		m.logger.Debug("Primary bedroom door opened, starting 20-second wake detection timer")
		m.masterWakeTimer = time.AfterFunc(20*time.Second, func() {
			m.detectMasterAwake()
		})
	} else {
		m.logger.Debug("Primary bedroom door closed, canceling wake detection")
	}
}

// detectMasterAwake runs after door has been open for 20 seconds
func (m *Manager) detectMasterAwake() {
	m.logger.Info("Marking master as awake (bedroom door open for 20 seconds)")

	if err := m.stateManager.SetBool("isMasterAsleep", false); err != nil {
		if errors.Is(err, state.ErrReadOnlyMode) {
			m.logger.Debug("Skipping isMasterAsleep update in read-only mode")
		} else {
			m.logger.Error("Failed to set isMasterAsleep to false", zap.Error(err))
		}
	}
}

// handleNickHomeChange processes Nick's presence state changes for TTS announcements
func (m *Manager) handleNickHomeChange(entityID string, oldState, newState *ha.State) {
	if newState == nil || oldState == nil {
		return
	}

	// Check if Nick just arrived (state changed to "on" from something else)
	if newState.State == "on" && oldState.State != "on" {
		m.logger.Debug("Nick arrived home, checking if should announce",
			zap.String("entity_id", entityID),
			zap.String("old_state", oldState.State),
			zap.String("new_state", newState.State))

		// Check if anyone else was already home (Caroline or Tori)
		// We check the OLD value of isAnyoneHome before Nick arrived
		wasAnyoneHome := false
		if isCarolineHome, err := m.stateManager.GetBool("isCarolineHome"); err == nil && isCarolineHome {
			wasAnyoneHome = true
		}
		if isToriHere, err := m.stateManager.GetBool("isToriHere"); err == nil && isToriHere {
			wasAnyoneHome = true
		}

		if wasAnyoneHome {
			// Run announcement asynchronously to avoid deadlocks
			go m.announceArrivalDirect("Nick", "Nick is home", []string{
				"media_player.kitchen",
				"media_player.dining_room",
				"media_player.soundbar",
				"media_player.kids_bathroom",
			})
		} else {
			m.logger.Debug("Nobody else was home, not announcing Nick's arrival")
		}
	}
}

// handleCarolineHomeChange processes Caroline's presence state changes for TTS announcements
func (m *Manager) handleCarolineHomeChange(entityID string, oldState, newState *ha.State) {
	if newState == nil || oldState == nil {
		return
	}

	// Check if Caroline just arrived (state changed to "on" from something else)
	if newState.State == "on" && oldState.State != "on" {
		m.logger.Debug("Caroline arrived home, checking if should announce",
			zap.String("entity_id", entityID),
			zap.String("old_state", oldState.State),
			zap.String("new_state", newState.State))

		// Check if anyone else was already home (Nick or Tori)
		wasAnyoneHome := false
		if isNickHome, err := m.stateManager.GetBool("isNickHome"); err == nil && isNickHome {
			wasAnyoneHome = true
		}
		if isToriHere, err := m.stateManager.GetBool("isToriHere"); err == nil && isToriHere {
			wasAnyoneHome = true
		}

		if wasAnyoneHome {
			// Run announcement asynchronously to avoid deadlocks
			go m.announceArrivalDirect("Caroline", "Caroline is home", []string{
				"media_player.kitchen",
				"media_player.dining_room",
				"media_player.kids_bathroom",
				"media_player.soundbar",
				"media_player.office",
			})
		} else {
			m.logger.Debug("Nobody else was home, not announcing Caroline's arrival")
		}
	}
}

// handleToriHereChange processes Tori's presence state changes for TTS announcements
func (m *Manager) handleToriHereChange(entityID string, oldState, newState *ha.State) {
	if newState == nil || oldState == nil {
		return
	}

	// Check if Tori just arrived (state changed to "on" from something else)
	if newState.State == "on" && oldState.State != "on" {
		m.logger.Debug("Tori arrived, checking if should announce",
			zap.String("entity_id", entityID),
			zap.String("old_state", oldState.State),
			zap.String("new_state", newState.State))

		// Check if anyone else was already home (Nick or Caroline)
		wasAnyoneHome := false
		if isNickHome, err := m.stateManager.GetBool("isNickHome"); err == nil && isNickHome {
			wasAnyoneHome = true
		}
		if isCarolineHome, err := m.stateManager.GetBool("isCarolineHome"); err == nil && isCarolineHome {
			wasAnyoneHome = true
		}

		if wasAnyoneHome {
			// Run announcement asynchronously to avoid deadlocks
			go m.announceArrivalDirect("Tori", "Tori is here", []string{
				"media_player.kitchen",
				"media_player.dining_room",
				"media_player.kids_bathroom",
				"media_player.soundbar",
				"media_player.office",
			})
		} else {
			m.logger.Debug("Nobody else was home, not announcing Tori's arrival")
		}
	}
}

// announceArrivalDirect makes a TTS announcement (caller has already checked if someone is home)
func (m *Manager) announceArrivalDirect(person, message string, mediaPlayers []string) {
	// Skip TTS in read-only mode
	if m.readOnly {
		m.logger.Info("Would announce arrival (read-only mode)",
			zap.String("person", person),
			zap.String("message", message),
			zap.Strings("media_players", mediaPlayers))
		return
	}

	// Make the TTS announcement
	m.logger.Info("Announcing arrival via TTS",
		zap.String("person", person),
		zap.String("message", message),
		zap.Strings("media_players", mediaPlayers))

	err := m.haClient.CallService("tts", "speak", map[string]interface{}{
		"entity_id":              "tts.google_translate_en_com",
		"message":                message,
		"cache":                  true,
		"media_player_entity_id": mediaPlayers,
	})

	if err != nil {
		m.logger.Error("Failed to announce arrival via TTS",
			zap.String("person", person),
			zap.Error(err))
	}
}

// Reset re-computes all derived states
func (m *Manager) Reset() error {
	m.logger.Info("Resetting State Tracking - re-computing all derived states")

	if m.helper != nil {
		// The helper automatically re-computes all derived states on initialization
		// and whenever source states change, so we just need to trigger a recalculation
		if err := m.helper.Recalculate(); err != nil {
			return fmt.Errorf("failed to recalculate derived states: %w", err)
		}
		m.logger.Info("Successfully re-computed all derived states")
	}

	return nil
}
