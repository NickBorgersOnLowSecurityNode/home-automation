package sleephygiene

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ScheduleConfig represents the schedule configuration structure
type ScheduleConfig struct {
	Schedule []DaySchedule `yaml:"schedule"`
}

// DaySchedule represents the schedule for a specific day of the week
type DaySchedule struct {
	Name        string `yaml:"name"`
	BeginWake   string `yaml:"begin_wake"`
	Wake        string `yaml:"wake"`
	StopScreens string `yaml:"stop_screens"`
	GoToBed     string `yaml:"go_to_bed"`
	Winddown    string `yaml:"winddown"`
	Night       string `yaml:"night"`
	Dusk        string `yaml:"dusk"`
}

// GetScheduleForDay returns the schedule for a specific weekday
func (sc *ScheduleConfig) GetScheduleForDay(weekday time.Weekday) (*DaySchedule, error) {
	// Map weekday to day name
	dayNames := map[time.Weekday]string{
		time.Sunday:    "Sunday",
		time.Monday:    "Monday",
		time.Tuesday:   "Tuesday",
		time.Wednesday: "Wednesday",
		time.Thursday:  "Thursday",
		time.Friday:    "Friday",
		time.Saturday:  "Saturday",
	}

	dayName := dayNames[weekday]

	// Find the schedule for this day
	for _, day := range sc.Schedule {
		if day.Name == dayName {
			return &day, nil
		}
	}

	return nil, fmt.Errorf("no schedule found for %s", dayName)
}

// ParseTimeToday parses a time string (HH:MM) and returns a time.Time for today
func ParseTimeToday(timeStr string) (time.Time, error) {
	now := time.Now()

	// Parse the time string
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse time %s: %w", timeStr, err)
	}

	// Combine with today's date
	return time.Date(
		now.Year(), now.Month(), now.Day(),
		t.Hour(), t.Minute(), 0, 0,
		now.Location(),
	), nil
}

// LoadScheduleConfig loads the schedule configuration from a YAML file
func LoadScheduleConfig(configPath string) (*ScheduleConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schedule config file: %w", err)
	}

	var config ScheduleConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse schedule config: %w", err)
	}

	// Validate that we have all 7 days
	if len(config.Schedule) != 7 {
		return nil, fmt.Errorf("expected 7 days in schedule, got %d", len(config.Schedule))
	}

	// Validate that all required days are present
	expectedDays := map[string]bool{
		"Sunday": false, "Monday": false, "Tuesday": false, "Wednesday": false,
		"Thursday": false, "Friday": false, "Saturday": false,
	}

	for _, day := range config.Schedule {
		if _, ok := expectedDays[day.Name]; !ok {
			return nil, fmt.Errorf("unexpected day name: %s", day.Name)
		}
		expectedDays[day.Name] = true
	}

	for day, found := range expectedDays {
		if !found {
			return nil, fmt.Errorf("missing required day: %s", day)
		}
	}

	return &config, nil
}
