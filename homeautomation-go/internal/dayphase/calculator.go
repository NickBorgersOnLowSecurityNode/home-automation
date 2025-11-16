package dayphase

import (
	"fmt"
	"time"

	"homeautomation/internal/config"

	"github.com/nathan-osman/go-sunrise"
	"go.uber.org/zap"
)

// SunEvent represents the simplified sun event state
type SunEvent string

const (
	SunEventMorning SunEvent = "morning"
	SunEventDay     SunEvent = "day"
	SunEventSunset  SunEvent = "sunset"
	SunEventDusk    SunEvent = "dusk"
	SunEventNight   SunEvent = "night"
)

// DayPhase represents the current day phase
type DayPhase string

const (
	DayPhaseMorning  DayPhase = "morning"
	DayPhaseDay      DayPhase = "day"
	DayPhaseSunset   DayPhase = "sunset"
	DayPhaseDusk     DayPhase = "dusk"
	DayPhaseWinddown DayPhase = "winddown"
	DayPhaseNight    DayPhase = "night"
)

// Calculator manages sun event tracking and day phase calculation
type Calculator struct {
	latitude  float64
	longitude float64
	logger    *zap.Logger

	// Cached sun times (updated every 6 hours)
	sunrise     time.Time
	sunset      time.Time
	sunriseEnd  time.Time
	sunsetStart time.Time
	dusk        time.Time
	dawn        time.Time
	lastUpdate  time.Time
}

// NewCalculator creates a new day phase calculator
// Default coordinates are for Austin, TX area (32.85486, -97.50515)
func NewCalculator(latitude, longitude float64, logger *zap.Logger) *Calculator {
	return &Calculator{
		latitude:  latitude,
		longitude: longitude,
		logger:    logger,
	}
}

// UpdateSunTimes calculates sun event times for today
func (c *Calculator) UpdateSunTimes() error {
	now := time.Now()

	// Calculate sunrise and sunset
	sunrise, sunset := sunrise.SunriseSunset(
		c.latitude, c.longitude,
		now.Year(), now.Month(), now.Day(),
	)

	c.sunrise = sunrise
	c.sunset = sunset

	// Calculate civil twilight (approximately)
	// Civil twilight is about 30 minutes before sunrise and after sunset
	c.dawn = sunrise.Add(-30 * time.Minute)
	c.dusk = sunset.Add(30 * time.Minute)

	// Golden hour is approximately 1 hour before sunset
	c.sunsetStart = sunset.Add(-60 * time.Minute)

	// Sunrise end is approximately 30 minutes after sunrise
	c.sunriseEnd = sunrise.Add(30 * time.Minute)

	c.lastUpdate = now

	c.logger.Info("Sun times updated",
		zap.Time("sunrise", c.sunrise),
		zap.Time("sunset", c.sunset),
		zap.Time("dawn", c.dawn),
		zap.Time("dusk", c.dusk))

	return nil
}

// GetSunEvent returns the current simplified sun event state
// Maps detailed sun events to: morning, day, sunset, dusk, night
func (c *Calculator) GetSunEvent() SunEvent {
	now := time.Now()

	// Ensure we have recent sun times
	if c.lastUpdate.IsZero() || time.Since(c.lastUpdate) > 6*time.Hour {
		c.UpdateSunTimes()
	}

	// Determine current sun event based on time of day
	switch {
	case now.Before(c.dawn):
		return SunEventNight
	case now.Before(c.sunrise):
		return SunEventMorning // Dawn period
	case now.Before(c.sunriseEnd):
		return SunEventMorning // Sunrise
	case now.Before(c.sunsetStart):
		return SunEventDay
	case now.Before(c.sunset):
		return SunEventSunset // Golden hour / sunset start
	case now.Before(c.dusk):
		return SunEventDusk // Civil twilight
	default:
		return SunEventNight
	}
}

// CalculateDayPhase determines the current day phase based on sun event and schedule
// This implements the logic from Node-RED's Configuration tab
func (c *Calculator) CalculateDayPhase(schedule *config.ParsedSchedule) DayPhase {
	sunEvent := c.GetSunEvent()
	now := time.Now()

	c.logger.Debug("Calculating day phase",
		zap.String("sun_event", string(sunEvent)),
		zap.Time("now", now))

	switch sunEvent {
	case SunEventMorning:
		// Stay in morning until noon, then switch to day
		if now.Hour() >= 12 {
			return DayPhaseDay
		}
		return DayPhaseMorning

	case SunEventDay:
		return DayPhaseDay

	case SunEventSunset:
		return DayPhaseSunset

	case SunEventDusk:
		return DayPhaseDusk

	case SunEventNight:
		// Check if we're past the scheduled "night" time
		if schedule != nil {
			if now.After(schedule.Night) || now.Hour() < 6 {
				return DayPhaseNight
			}
			return DayPhaseWinddown
		}
		// No schedule available, use simple logic
		if now.Hour() >= 23 || now.Hour() < 6 {
			return DayPhaseNight
		}
		return DayPhaseWinddown

	default:
		return DayPhaseDay
	}
}

// StartPeriodicUpdate starts a goroutine that updates sun times every 6 hours
func (c *Calculator) StartPeriodicUpdate() chan struct{} {
	stopChan := make(chan struct{})

	c.logger.Info("Starting periodic sun time updates (every 6 hours)")

	// Initial update
	c.UpdateSunTimes()

	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.logger.Debug("Periodic sun time update")
				if err := c.UpdateSunTimes(); err != nil {
					c.logger.Error("Failed to update sun times", zap.Error(err))
				}

			case <-stopChan:
				c.logger.Info("Stopping periodic sun time updates")
				return
			}
		}
	}()

	return stopChan
}

// ValidateDayPhase checks if a string is a valid day phase
func ValidateDayPhase(phase string) (DayPhase, error) {
	switch phase {
	case string(DayPhaseMorning):
		return DayPhaseMorning, nil
	case string(DayPhaseDay):
		return DayPhaseDay, nil
	case string(DayPhaseSunset):
		return DayPhaseSunset, nil
	case string(DayPhaseDusk):
		return DayPhaseDusk, nil
	case string(DayPhaseWinddown):
		return DayPhaseWinddown, nil
	case string(DayPhaseNight):
		return DayPhaseNight, nil
	default:
		return "", fmt.Errorf("invalid day phase: %s", phase)
	}
}
