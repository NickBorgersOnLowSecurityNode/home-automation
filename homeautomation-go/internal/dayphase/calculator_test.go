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
