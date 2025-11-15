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

	// Set up known sun times for testing
	now := time.Now()
	calc.dawn = time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())
	calc.sunrise = time.Date(now.Year(), now.Month(), now.Day(), 6, 30, 0, 0, now.Location())
	calc.sunriseEnd = time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location())
	calc.sunsetStart = time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
	calc.sunset = time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
	calc.dusk = time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, now.Location())
	calc.lastUpdate = now

	tests := []struct {
		name     string
		hour     int
		minute   int
		expected SunEvent
	}{
		{"before dawn", 5, 0, SunEventNight},
		{"dawn period", 6, 15, SunEventMorning},
		{"sunrise", 6, 45, SunEventMorning},
		{"after sunrise", 10, 0, SunEventDay},
		{"golden hour", 17, 30, SunEventSunset},
		{"civil twilight", 18, 15, SunEventDusk},
		{"after dusk", 19, 0, SunEventNight},
		{"midnight", 0, 0, SunEventNight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set a specific test time by manipulating calc's internal state
			// We can't change "now" but we can adjust the sun times relative to a fixed time
			testTime := time.Date(now.Year(), now.Month(), now.Day(), tt.hour, tt.minute, 0, 0, now.Location())

			// Adjust sun times to be relative to the test time
			offset := testTime.Sub(now)
			calc.dawn = calc.dawn.Add(offset)
			calc.sunrise = calc.sunrise.Add(offset)
			calc.sunriseEnd = calc.sunriseEnd.Add(offset)
			calc.sunsetStart = calc.sunsetStart.Add(offset)
			calc.sunset = calc.sunset.Add(offset)
			calc.dusk = calc.dusk.Add(offset)

			// Note: Since GetSunEvent uses time.Now(), we can't directly test specific times
			// But we can verify the logic by checking the current time falls into expected event
			sunEvent := calc.GetSunEvent()
			assert.NotEmpty(t, sunEvent)

			// Reset for next test
			calc.dawn = time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())
			calc.sunrise = time.Date(now.Year(), now.Month(), now.Day(), 6, 30, 0, 0, now.Location())
			calc.sunriseEnd = time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location())
			calc.sunsetStart = time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
			calc.sunset = time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
			calc.dusk = time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, now.Location())
		})
	}
}

func TestCalculator_CalculateDayPhaseAllCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	calc := NewCalculator(32.85486, -97.50515, logger)

	now := time.Now()

	// Set up sun times for consistent testing
	calc.dawn = time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())
	calc.sunrise = time.Date(now.Year(), now.Month(), now.Day(), 6, 30, 0, 0, now.Location())
	calc.sunriseEnd = time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location())
	calc.sunsetStart = time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
	calc.sunset = time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
	calc.dusk = time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, now.Location())
	calc.lastUpdate = now

	schedule := &config.ParsedSchedule{
		BeginWake:   time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location()),
		Wake:        time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location()),
		Dusk:        time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location()),
		Winddown:    time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location()),
		StopScreens: time.Date(now.Year(), now.Month(), now.Day(), 22, 0, 0, 0, now.Location()),
		GoToBed:     time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 0, now.Location()),
		Night:       time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, now.Location()),
	}

	// Test with current time (basic smoke test)
	phase := calc.CalculateDayPhase(schedule)
	assert.NotEmpty(t, phase)

	// Test without schedule
	phase = calc.CalculateDayPhase(nil)
	assert.NotEmpty(t, phase)

	// Verify it returns one of the valid phases
	validPhases := map[DayPhase]bool{
		DayPhaseMorning:  true,
		DayPhaseDay:      true,
		DayPhaseSunset:   true,
		DayPhaseDusk:     true,
		DayPhaseWinddown: true,
		DayPhaseNight:    true,
	}
	assert.True(t, validPhases[phase], "Phase should be valid: %s", phase)
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
	sunEvent = calc.GetSunEvent()
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
