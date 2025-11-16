package sleephygiene

import (
	"fmt"
	"time"

	"homeautomation/internal/config"
	"homeautomation/internal/ha"
	"homeautomation/internal/state"

	"go.uber.org/zap"
)

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

// Manager handles sleep hygiene automations including wake-up sequences
type Manager struct {
	haClient      ha.HAClient
	stateManager  *state.Manager
	configLoader  *config.Loader
	logger        *zap.Logger
	readOnly      bool
	timeProvider  TimeProvider
	stopChan      chan struct{}
	ticker        *time.Ticker
	subscriptions []state.Subscription

	// Track which triggers have been fired today
	triggeredToday map[string]time.Time
}

// NewManager creates a new Sleep Hygiene manager
// If timeProvider is nil, it defaults to RealTimeProvider
func NewManager(haClient ha.HAClient, stateManager *state.Manager, configLoader *config.Loader, logger *zap.Logger, readOnly bool, timeProvider TimeProvider) *Manager {
	if timeProvider == nil {
		timeProvider = RealTimeProvider{}
	}
	return &Manager{
		haClient:       haClient,
		stateManager:   stateManager,
		configLoader:   configLoader,
		logger:         logger.Named("sleephygiene"),
		readOnly:       readOnly,
		timeProvider:   timeProvider,
		stopChan:       make(chan struct{}),
		subscriptions:  make([]state.Subscription, 0),
		triggeredToday: make(map[string]time.Time),
	}
}

// Start begins monitoring state changes and managing sleep hygiene
func (m *Manager) Start() error {
	m.logger.Info("Starting Sleep Hygiene Manager")

	// Subscribe to alarmTime changes
	sub, err := m.stateManager.Subscribe("alarmTime", m.handleAlarmTimeChange)
	if err != nil {
		return fmt.Errorf("failed to subscribe to alarmTime: %w", err)
	}
	m.subscriptions = append(m.subscriptions, sub)

	// Start ticker to check time triggers every minute
	m.ticker = time.NewTicker(1 * time.Minute)
	go m.runTimerLoop()

	// Perform initial check
	m.checkTimeTriggers()

	m.logger.Info("Sleep Hygiene Manager started successfully")
	return nil
}

// Stop stops the Sleep Hygiene Manager and cleans up resources
func (m *Manager) Stop() {
	m.logger.Info("Stopping Sleep Hygiene Manager")

	// Stop ticker
	if m.ticker != nil {
		m.ticker.Stop()
	}

	// Signal stop
	close(m.stopChan)

	// Unsubscribe from all subscriptions
	for _, sub := range m.subscriptions {
		sub.Unsubscribe()
	}
	m.subscriptions = nil

	m.logger.Info("Sleep Hygiene Manager stopped")
}

// handleAlarmTimeChange processes alarm time changes
func (m *Manager) handleAlarmTimeChange(key string, oldValue, newValue interface{}) {
	m.logger.Debug("Alarm time changed",
		zap.Any("old", oldValue),
		zap.Any("new", newValue))

	// When alarm time changes, reset the triggered flags for wake-related triggers
	delete(m.triggeredToday, "begin_wake")
	delete(m.triggeredToday, "wake")

	// Check if we should trigger now
	m.checkTimeTriggers()
}

// runTimerLoop runs the main timer loop that checks for time triggers
func (m *Manager) runTimerLoop() {
	for {
		select {
		case <-m.ticker.C:
			// Check if we crossed midnight - reset triggers
			now := m.timeProvider.Now()
			if len(m.triggeredToday) > 0 {
				// Check if any triggered time is from a previous day
				for trigger, triggerTime := range m.triggeredToday {
					if !isSameDay(now, triggerTime) {
						m.logger.Debug("Resetting trigger for new day",
							zap.String("trigger", trigger))
						delete(m.triggeredToday, trigger)
					}
				}
			}

			// Check time triggers
			m.checkTimeTriggers()

		case <-m.stopChan:
			return
		}
	}
}

// checkTimeTriggers checks all time-based triggers and fires them if conditions are met
func (m *Manager) checkTimeTriggers() {
	now := m.timeProvider.Now()

	// Get alarm time from state
	alarmTimeMs, err := m.stateManager.GetNumber("alarmTime")
	if err != nil {
		m.logger.Debug("Failed to get alarmTime", zap.Error(err))
		return
	}

	// Convert milliseconds timestamp to time.Time
	alarmTime := time.Unix(int64(alarmTimeMs)/1000, 0)

	// Calculate wake time (25 minutes after alarm time)
	wakeTime := alarmTime.Add(25 * time.Minute)

	// Get today's schedule
	schedule, err := m.configLoader.GetTodaysSchedule()
	if err != nil {
		m.logger.Error("Failed to get today's schedule", zap.Error(err))
		return
	}

	const ONE_HOUR = time.Hour

	// Check begin_wake trigger
	if now.After(alarmTime) && now.Before(alarmTime.Add(ONE_HOUR)) {
		if _, triggered := m.triggeredToday["begin_wake"]; !triggered {
			m.logger.Info("Triggering begin_wake",
				zap.Time("alarm_time", alarmTime),
				zap.Time("now", now))
			m.triggeredToday["begin_wake"] = now
			m.handleBeginWake()
		}
	}

	// Check wake trigger
	if now.After(wakeTime) && now.Before(wakeTime.Add(ONE_HOUR)) {
		if _, triggered := m.triggeredToday["wake"]; !triggered {
			m.logger.Info("Triggering wake",
				zap.Time("wake_time", wakeTime),
				zap.Time("now", now))
			m.triggeredToday["wake"] = now
			m.handleWake()
		}
	}

	// Check stop_screens trigger
	if now.After(schedule.StopScreens) && now.Before(schedule.StopScreens.Add(ONE_HOUR)) {
		if _, triggered := m.triggeredToday["stop_screens"]; !triggered {
			m.logger.Info("Triggering stop_screens",
				zap.Time("stop_screens_time", schedule.StopScreens),
				zap.Time("now", now))
			m.triggeredToday["stop_screens"] = now
			m.handleStopScreens()
		}
	}

	// Check go_to_bed trigger
	if now.After(schedule.GoToBed) && now.Before(schedule.GoToBed.Add(ONE_HOUR)) {
		if _, triggered := m.triggeredToday["go_to_bed"]; !triggered {
			m.logger.Info("Triggering go_to_bed",
				zap.Time("go_to_bed_time", schedule.GoToBed),
				zap.Time("now", now))
			m.triggeredToday["go_to_bed"] = now
			m.handleGoToBed()
		}
	}
}

// handleBeginWake handles the begin_wake trigger (start fading out sleep music)
func (m *Manager) handleBeginWake() {
	m.logger.Info("Handling begin_wake trigger")

	// Check conditions: anyone home, master asleep, sleep music playing
	isAnyoneHome, err := m.stateManager.GetBool("isAnyoneHome")
	if err != nil || !isAnyoneHome {
		m.logger.Debug("Skipping begin_wake: no one home")
		return
	}

	isMasterAsleep, err := m.stateManager.GetBool("isMasterAsleep")
	if err != nil || !isMasterAsleep {
		m.logger.Debug("Skipping begin_wake: master not asleep")
		return
	}

	musicPlaybackType, err := m.stateManager.GetString("musicPlaybackType")
	if err != nil || musicPlaybackType != "sleep" {
		m.logger.Debug("Skipping begin_wake: not playing sleep music",
			zap.String("music_type", musicPlaybackType))
		return
	}

	// All conditions met - start fade out
	m.logger.Info("Conditions met for begin_wake, starting fade out")

	if !m.readOnly {
		// Set fade out in progress flag
		if err := m.stateManager.SetBool("isFadeOutInProgress", true); err != nil {
			m.logger.Error("Failed to set isFadeOutInProgress", zap.Error(err))
		}

		// TODO: Implement actual fade out logic
		// This would involve gradually reducing volume on bedroom speakers
		// For now, we just set the flag which other systems can monitor
	} else {
		m.logger.Info("[READ-ONLY] Would start fade out")
	}
}

// handleWake handles the wake trigger (turn on lights, flash, cuddle announcement)
func (m *Manager) handleWake() {
	m.logger.Info("Handling wake trigger")

	// Check conditions: anyone home, master asleep, fade out in progress
	isAnyoneHome, err := m.stateManager.GetBool("isAnyoneHome")
	if err != nil || !isAnyoneHome {
		m.logger.Debug("Skipping wake: no one home")
		return
	}

	isMasterAsleep, err := m.stateManager.GetBool("isMasterAsleep")
	if err != nil || !isMasterAsleep {
		m.logger.Debug("Skipping wake: master not asleep")
		return
	}

	isFadeOutInProgress, err := m.stateManager.GetBool("isFadeOutInProgress")
	if err != nil || !isFadeOutInProgress {
		m.logger.Debug("Skipping wake: fade out not in progress")
		return
	}

	// All conditions met - execute wake sequence
	m.logger.Info("Conditions met for wake, executing wake sequence")

	if !m.readOnly {
		// 1. Turn on master bedroom lights slowly (30 minute transition)
		m.turnOnMasterBedroomLights()

		// 2. Flash lights in common areas
		m.flashCommonAreaLights()

		// 3. Check if both owners can cuddle and announce
		m.checkAndAnnounceCuddle()
	} else {
		m.logger.Info("[READ-ONLY] Would execute wake sequence (lights + cuddle)")
	}
}

// handleStopScreens handles the stop_screens trigger (flash lights as reminder)
func (m *Manager) handleStopScreens() {
	m.logger.Info("Handling stop_screens trigger")

	// Check conditions: anyone home and not everyone asleep
	isAnyoneHome, err := m.stateManager.GetBool("isAnyoneHome")
	if err != nil || !isAnyoneHome {
		m.logger.Debug("Skipping stop_screens: no one home")
		return
	}

	isEveryoneAsleep, err := m.stateManager.GetBool("isEveryoneAsleep")
	if err != nil || isEveryoneAsleep {
		m.logger.Debug("Skipping stop_screens: everyone is asleep")
		return
	}

	// Conditions met - flash lights
	m.logger.Info("Conditions met for stop_screens, flashing lights")

	if !m.readOnly {
		m.flashCommonAreaLights()
	} else {
		m.logger.Info("[READ-ONLY] Would flash common area lights")
	}
}

// handleGoToBed handles the go_to_bed trigger
func (m *Manager) handleGoToBed() {
	m.logger.Info("Handling go_to_bed trigger")
	// Note: The Node-RED implementation doesn't have logic for this trigger yet
	// This is a placeholder for future implementation
}

// turnOnMasterBedroomLights turns on master bedroom lights with a slow 30-minute transition
func (m *Manager) turnOnMasterBedroomLights() {
	m.logger.Info("Turning on master bedroom lights slowly")

	// First, ensure lights start dim and white
	if err := m.haClient.CallService("light", "turn_on", map[string]interface{}{
		"entity_id":      "light.master_bedroom",
		"transition":     0,
		"color_temp":     290,
		"brightness_pct": 1,
	}); err != nil {
		m.logger.Error("Failed to set initial bedroom light state", zap.Error(err))
		return
	}

	// Then start slow transition to full brightness over 30 minutes
	if err := m.haClient.CallService("light", "turn_on", map[string]interface{}{
		"entity_id":      "light.master_bedroom",
		"transition":     1800, // 30 minutes in seconds
		"color_temp":     290,
		"brightness_pct": 100,
	}); err != nil {
		m.logger.Error("Failed to start bedroom light transition", zap.Error(err))
	}
}

// flashCommonAreaLights flashes lights in common areas as a notification
func (m *Manager) flashCommonAreaLights() {
	m.logger.Info("Flashing common area lights")

	commonAreaLights := []string{
		"light.living_room",
		"light.kitchen",
	}

	for _, lightEntity := range commonAreaLights {
		if err := m.haClient.CallService("light", "turn_on", map[string]interface{}{
			"entity_id": lightEntity,
			"flash":     "short",
		}); err != nil {
			m.logger.Error("Failed to flash light",
				zap.String("entity", lightEntity),
				zap.Error(err))
		}
	}
}

// checkAndAnnounceCuddle checks if both owners are home and announces cuddle time via TTS
func (m *Manager) checkAndAnnounceCuddle() {
	m.logger.Info("Checking if cuddle announcement should be made")

	isNickHome, err := m.stateManager.GetBool("isNickHome")
	if err != nil {
		m.logger.Error("Failed to get isNickHome", zap.Error(err))
		return
	}

	isCarolineHome, err := m.stateManager.GetBool("isCarolineHome")
	if err != nil {
		m.logger.Error("Failed to get isCarolineHome", zap.Error(err))
		return
	}

	if isNickHome && isCarolineHome {
		m.logger.Info("Both owners home, announcing cuddle time")

		if err := m.haClient.CallService("tts", "speak", map[string]interface{}{
			"cache":                  true,
			"media_player_entity_id": []string{"media_player.bedroom"},
			"message":                "Time to cuddle",
		}); err != nil {
			m.logger.Error("Failed to announce cuddle time", zap.Error(err))
		}
	} else {
		m.logger.Debug("Only one owner home, skipping cuddle announcement",
			zap.Bool("nick_home", isNickHome),
			zap.Bool("caroline_home", isCarolineHome))
	}
}

// isSameDay checks if two times are on the same day
func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
