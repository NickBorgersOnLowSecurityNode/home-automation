package dayphase

import (
	"testing"
	"time"

	"homeautomation/internal/config"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCalculator_UpdateSunTimes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Austin, TX coordinates
	calc := NewCalculator(32.85486, -97.50515, logger)

	err := calc.UpdateSunTimes()
	assert.NoError(t, err)

	// Verify sun times are set
	assert.False(t, calc.sunrise.IsZero(), "Sunrise should be set")
	assert.False(t, calc.sunset.IsZero(), "Sunset should be set")
	assert.False(t, calc.dawn.IsZero(), "Dawn should be set")
	assert.False(t, calc.dusk.IsZero(), "Dusk should be set")

	// Verify times are in reasonable order
	assert.True(t, calc.dawn.Before(calc.sunrise), "Dawn should be before sunrise")
	assert.True(t, calc.sunrise.Before(calc.sunset), "Sunrise should be before sunset")
	assert.True(t, calc.sunset.Before(calc.dusk), "Sunset should be before dusk")
}

func TestCalculator_GetSunEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	err := calc.UpdateSunTimes()
	assert.NoError(t, err)

	// Get current sun event
	sunEvent := calc.GetSunEvent()
	assert.NotEmpty(t, sunEvent)

	// Verify it's one of the valid sun events
	validEvents := []SunEvent{SunEventMorning, SunEventDay, SunEventSunset, SunEventDusk, SunEventNight}
	found := false
	for _, valid := range validEvents {
		if sunEvent == valid {
			found = true
			break
		}
	}
	assert.True(t, found, "Sun event should be one of the valid values")
}

func TestCalculator_CalculateDayPhase(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	err := calc.UpdateSunTimes()
	assert.NoError(t, err)

	// Create a sample schedule
	now := time.Now()
	schedule := &config.ParsedSchedule{
		BeginWake:   time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location()),
		Wake:        time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location()),
		Dusk:        time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location()),
		Winddown:    time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location()),
		StopScreens: time.Date(now.Year(), now.Month(), now.Day(), 22, 0, 0, 0, now.Location()),
		GoToBed:     time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 0, now.Location()),
		Night:       time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, now.Location()),
	}

	dayPhase := calc.CalculateDayPhase(schedule)
	assert.NotEmpty(t, dayPhase)

	// Verify it's one of the valid day phases
	validPhases := []DayPhase{
		DayPhaseMorning,
		DayPhaseDay,
		DayPhaseSunset,
		DayPhaseDusk,
		DayPhaseWinddown,
		DayPhaseNight,
	}
	found := false
	for _, valid := range validPhases {
		if dayPhase == valid {
			found = true
			break
		}
	}
	assert.True(t, found, "Day phase should be one of the valid values")
}

func TestCalculator_CalculateDayPhaseWithoutSchedule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	err := calc.UpdateSunTimes()
	assert.NoError(t, err)

	// Calculate without schedule (should use fallback logic)
	dayPhase := calc.CalculateDayPhase(nil)
	assert.NotEmpty(t, dayPhase)
}

func TestCalculator_GetSunEventAllPeriods(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	now := time.Now()

	tests := []struct {
		name        string
		dawn        time.Time
		sunrise     time.Time
		sunriseEnd  time.Time
		sunsetStart time.Time
		sunset      time.Time
		dusk        time.Time
		expected    SunEvent
	}{
		{
			name:        "before dawn - night",
			dawn:        now.Add(2 * time.Hour),
			sunrise:     now.Add(3 * time.Hour),
			sunriseEnd:  now.Add(4 * time.Hour),
			sunsetStart: now.Add(12 * time.Hour),
			sunset:      now.Add(13 * time.Hour),
			dusk:        now.Add(14 * time.Hour),
			expected:    SunEventNight,
		},
		{
			name:        "dawn period - between dawn and sunrise",
			dawn:        now.Add(-30 * time.Minute),
			sunrise:     now.Add(30 * time.Minute),
			sunriseEnd:  now.Add(1 * time.Hour),
			sunsetStart: now.Add(10 * time.Hour),
			sunset:      now.Add(11 * time.Hour),
			dusk:        now.Add(12 * time.Hour),
			expected:    SunEventMorning,
		},
		{
			name:        "sunrise period - between sunrise and sunrise end",
			dawn:        now.Add(-2 * time.Hour),
			sunrise:     now.Add(-1 * time.Hour),
			sunriseEnd:  now.Add(1 * time.Hour),
			sunsetStart: now.Add(10 * time.Hour),
			sunset:      now.Add(11 * time.Hour),
			dusk:        now.Add(12 * time.Hour),
			expected:    SunEventMorning,
		},
		{
			name:        "day - between sunrise end and sunset start",
			dawn:        now.Add(-6 * time.Hour),
			sunrise:     now.Add(-5 * time.Hour),
			sunriseEnd:  now.Add(-4 * time.Hour),
			sunsetStart: now.Add(4 * time.Hour),
			sunset:      now.Add(5 * time.Hour),
			dusk:        now.Add(6 * time.Hour),
			expected:    SunEventDay,
		},
		{
			name:        "sunset - golden hour",
			dawn:        now.Add(-12 * time.Hour),
			sunrise:     now.Add(-11 * time.Hour),
			sunriseEnd:  now.Add(-10 * time.Hour),
			sunsetStart: now.Add(-1 * time.Hour),
			sunset:      now.Add(1 * time.Hour),
			dusk:        now.Add(2 * time.Hour),
			expected:    SunEventSunset,
		},
		{
			name:        "dusk - civil twilight",
			dawn:        now.Add(-14 * time.Hour),
			sunrise:     now.Add(-13 * time.Hour),
			sunriseEnd:  now.Add(-12 * time.Hour),
			sunsetStart: now.Add(-3 * time.Hour),
			sunset:      now.Add(-2 * time.Hour),
			dusk:        now.Add(1 * time.Hour),
			expected:    SunEventDusk,
		},
		{
			name:        "after dusk - night",
			dawn:        now.Add(-16 * time.Hour),
			sunrise:     now.Add(-15 * time.Hour),
			sunriseEnd:  now.Add(-14 * time.Hour),
			sunsetStart: now.Add(-5 * time.Hour),
			sunset:      now.Add(-4 * time.Hour),
			dusk:        now.Add(-3 * time.Hour),
			expected:    SunEventNight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set sun times to create the desired test scenario
			calc.dawn = tt.dawn
			calc.sunrise = tt.sunrise
			calc.sunriseEnd = tt.sunriseEnd
			calc.sunsetStart = tt.sunsetStart
			calc.sunset = tt.sunset
			calc.dusk = tt.dusk
			calc.lastUpdate = now

			sunEvent := calc.GetSunEvent()
			assert.Equal(t, tt.expected, sunEvent, "Expected %s, got %s", tt.expected, sunEvent)
		})
	}
}

func TestCalculator_CalculateDayPhaseAllCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	now := time.Now()

	// Create a schedule for testing
	schedule := &config.ParsedSchedule{
		BeginWake:   time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location()),
		Wake:        time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location()),
		Dusk:        time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location()),
		Winddown:    time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location()),
		StopScreens: time.Date(now.Year(), now.Month(), now.Day(), 22, 0, 0, 0, now.Location()),
		GoToBed:     time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 0, now.Location()),
		Night:       time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, now.Location()),
	}

	tests := []struct {
		name          string
		setupSunTimes func(c *Calculator)
		schedule      *config.ParsedSchedule
		expected      DayPhase
	}{
		{
			name: "morning period",
			setupSunTimes: func(c *Calculator) {
				// Set sun times so current time falls in morning sun event
				c.dawn = now.Add(-2 * time.Hour)
				c.sunrise = now.Add(-1 * time.Hour)
				c.sunriseEnd = now.Add(1 * time.Hour) // Still in morning sun event
				c.sunsetStart = now.Add(10 * time.Hour)
				c.sunset = now.Add(11 * time.Hour)
				c.dusk = now.Add(12 * time.Hour)
				c.lastUpdate = now
			},
			schedule: schedule,
			expected: DayPhaseMorning,
		},
		{
			name: "day phase",
			setupSunTimes: func(c *Calculator) {
				// Set sun times so current time falls in day period
				c.dawn = now.Add(-8 * time.Hour)
				c.sunrise = now.Add(-7 * time.Hour)
				c.sunriseEnd = now.Add(-6 * time.Hour)
				c.sunsetStart = now.Add(6 * time.Hour)
				c.sunset = now.Add(7 * time.Hour)
				c.dusk = now.Add(8 * time.Hour)
				c.lastUpdate = now
			},
			schedule: schedule,
			expected: DayPhaseDay,
		},
		{
			name: "sunset phase",
			setupSunTimes: func(c *Calculator) {
				// Set sun times so current time falls in sunset period
				c.dawn = now.Add(-14 * time.Hour)
				c.sunrise = now.Add(-13 * time.Hour)
				c.sunriseEnd = now.Add(-12 * time.Hour)
				c.sunsetStart = now.Add(-1 * time.Hour)
				c.sunset = now.Add(1 * time.Hour) // In golden hour
				c.dusk = now.Add(2 * time.Hour)
				c.lastUpdate = now
			},
			schedule: schedule,
			expected: DayPhaseSunset,
		},
		{
			name: "dusk phase",
			setupSunTimes: func(c *Calculator) {
				// Set sun times so current time falls in dusk period
				c.dawn = now.Add(-16 * time.Hour)
				c.sunrise = now.Add(-15 * time.Hour)
				c.sunriseEnd = now.Add(-14 * time.Hour)
				c.sunsetStart = now.Add(-3 * time.Hour)
				c.sunset = now.Add(-2 * time.Hour)
				c.dusk = now.Add(1 * time.Hour) // In civil twilight
				c.lastUpdate = now
			},
			schedule: schedule,
			expected: DayPhaseDusk,
		},
		{
			name: "night with schedule - after schedule.Night",
			setupSunTimes: func(c *Calculator) {
				// Set sun times so current time falls in night period
				c.dawn = now.Add(4 * time.Hour)
				c.sunrise = now.Add(5 * time.Hour)
				c.sunriseEnd = now.Add(6 * time.Hour)
				c.sunsetStart = now.Add(14 * time.Hour)
				c.sunset = now.Add(15 * time.Hour)
				c.dusk = now.Add(-1 * time.Hour) // Past dusk - night
				c.lastUpdate = now
			},
			schedule: &config.ParsedSchedule{
				BeginWake: time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location()),
				Wake:      time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location()),
				Night:     now.Add(-2 * time.Hour), // Schedule night time in the past
			},
			expected: DayPhaseNight,
		},
		{
			name: "winddown with schedule - before schedule.Night",
			setupSunTimes: func(c *Calculator) {
				// Set sun times so current time falls in night period
				c.dawn = now.Add(4 * time.Hour)
				c.sunrise = now.Add(5 * time.Hour)
				c.sunriseEnd = now.Add(6 * time.Hour)
				c.sunsetStart = now.Add(14 * time.Hour)
				c.sunset = now.Add(15 * time.Hour)
				c.dusk = now.Add(-1 * time.Hour) // Past dusk
				c.lastUpdate = now
			},
			schedule: &config.ParsedSchedule{
				BeginWake: time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location()),
				Wake:      time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location()),
				Night:     now.Add(2 * time.Hour), // Schedule night time in the future
			},
			// Expected phase depends on current time: Night if hour < 6, otherwise Winddown
			expected: func() DayPhase {
				if now.Hour() < 6 {
					return DayPhaseNight
				}
				return DayPhaseWinddown
			}(),
		},
		{
			name: "night without schedule - late night",
			setupSunTimes: func(c *Calculator) {
				// Set sun times so current time falls in night period
				c.dawn = now.Add(4 * time.Hour)
				c.sunrise = now.Add(5 * time.Hour)
				c.sunriseEnd = now.Add(6 * time.Hour)
				c.sunsetStart = now.Add(14 * time.Hour)
				c.sunset = now.Add(15 * time.Hour)
				c.dusk = now.Add(-1 * time.Hour) // Past dusk
				c.lastUpdate = now
			},
			schedule: nil, // No schedule
			// Expected phase depends on current time: Night if hour >= 23 or < 6, otherwise Winddown
			expected: func() DayPhase {
				if now.Hour() >= 23 || now.Hour() < 6 {
					return DayPhaseNight
				}
				return DayPhaseWinddown
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewCalculator(32.85486, -97.50515, logger)
			tt.setupSunTimes(calc)

			phase := calc.CalculateDayPhase(tt.schedule)
			assert.Equal(t, tt.expected, phase, "Expected %s, got %s", tt.expected, phase)
		})
	}
}

// TestCalculator_CalculateDayPhaseEdgeCases tests edge cases in day phase calculation
func TestCalculator_CalculateDayPhaseEdgeCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	now := time.Now()

	// Test day sun event - sun has already risen (sunriseEnd in past)
	calc.dawn = now.Add(-8 * time.Hour)
	calc.sunrise = now.Add(-7 * time.Hour)
	calc.sunriseEnd = now.Add(-6 * time.Hour)
	calc.sunsetStart = now.Add(6 * time.Hour)
	calc.sunset = now.Add(7 * time.Hour)
	calc.dusk = now.Add(8 * time.Hour)
	calc.lastUpdate = now

	// Since sun times show we're in "day" period (after sunriseEnd), it should be day
	phase := calc.CalculateDayPhase(nil)
	assert.Equal(t, DayPhaseDay, phase)
}

func TestCalculator_AutoUpdateSunTimes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	// Test that GetSunEvent auto-updates if lastUpdate is zero
	assert.True(t, calc.lastUpdate.IsZero())
	sunEvent := calc.GetSunEvent()
	assert.NotEmpty(t, sunEvent)
	assert.False(t, calc.lastUpdate.IsZero(), "GetSunEvent should trigger auto-update")

	// Test that it doesn't update if recent
	lastUpdate := calc.lastUpdate
	_ = calc.GetSunEvent()
	assert.Equal(t, lastUpdate, calc.lastUpdate, "Should not update if recent")
}

func TestCalculator_StartPeriodicUpdate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	// Start periodic updates
	stopChan := calc.StartPeriodicUpdate()
	assert.NotNil(t, stopChan)

	// Verify initial update happened
	assert.False(t, calc.lastUpdate.IsZero())
	assert.False(t, calc.sunrise.IsZero())

	// Stop the periodic updates
	close(stopChan)

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)
}

func TestValidateDayPhase(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  DayPhase
		shouldErr bool
	}{
		{"valid morning", "morning", DayPhaseMorning, false},
		{"valid day", "day", DayPhaseDay, false},
		{"valid sunset", "sunset", DayPhaseSunset, false},
		{"valid dusk", "dusk", DayPhaseDusk, false},
		{"valid winddown", "winddown", DayPhaseWinddown, false},
		{"valid night", "night", DayPhaseNight, false},
		{"invalid", "invalid", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase, err := ValidateDayPhase(tt.input)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, phase)
			}
		})
	}
}

func TestSunEventConstants(t *testing.T) {
	// Verify sun event constants are defined correctly
	assert.Equal(t, SunEvent("morning"), SunEventMorning)
	assert.Equal(t, SunEvent("day"), SunEventDay)
	assert.Equal(t, SunEvent("sunset"), SunEventSunset)
	assert.Equal(t, SunEvent("dusk"), SunEventDusk)
	assert.Equal(t, SunEvent("night"), SunEventNight)
}

func TestDayPhaseConstants(t *testing.T) {
	// Verify day phase constants are defined correctly
	assert.Equal(t, DayPhase("morning"), DayPhaseMorning)
	assert.Equal(t, DayPhase("day"), DayPhaseDay)
	assert.Equal(t, DayPhase("sunset"), DayPhaseSunset)
	assert.Equal(t, DayPhase("dusk"), DayPhaseDusk)
	assert.Equal(t, DayPhase("winddown"), DayPhaseWinddown)
	assert.Equal(t, DayPhase("night"), DayPhaseNight)
}
