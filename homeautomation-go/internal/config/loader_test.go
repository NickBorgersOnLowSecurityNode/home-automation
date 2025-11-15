package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestConfigDir(t *testing.T) string {
	tmpDir := t.TempDir()

	// Create sample music_config.yaml
	musicConfig := `playlists:
  morning:
    - "spotify:playlist:123"
  day:
    - "spotify:playlist:456"
speakers:
  living_room: "192.168.1.100"
  bedroom: "192.168.1.101"
volumes:
  morning: 0.5
  day: 0.7
`
	err := os.WriteFile(filepath.Join(tmpDir, "music_config.yaml"), []byte(musicConfig), 0644)
	require.NoError(t, err)

	// Create sample hue_config.yaml
	hueConfig := `lights:
  living_room:
    - "light.living_room_1"
    - "light.living_room_2"
scenes:
  morning: "bright"
  evening: "dim"
groups:
  all: "group.all_lights"
`
	err = os.WriteFile(filepath.Join(tmpDir, "hue_config.yaml"), []byte(hueConfig), 0644)
	require.NoError(t, err)

	// Create sample schedule_config.yaml
	scheduleConfig := `schedule:
  - begin_wake: "05:00"
    wake: "07:00"
    dusk: "18:00"
    winddown: "21:00"
    stop_screens: "22:00"
    go_to_bed: "22:30"
    night: "23:00"
  - begin_wake: "05:30"
    wake: "07:30"
    dusk: "18:00"
    winddown: "21:00"
    stop_screens: "22:00"
    go_to_bed: "22:30"
    night: "23:00"
  - begin_wake: "05:00"
    wake: "07:00"
    dusk: "18:00"
    winddown: "21:00"
    stop_screens: "22:00"
    go_to_bed: "22:30"
    night: "23:00"
  - begin_wake: "05:00"
    wake: "07:00"
    dusk: "18:00"
    winddown: "21:00"
    stop_screens: "22:00"
    go_to_bed: "22:30"
    night: "23:00"
  - begin_wake: "05:00"
    wake: "07:00"
    dusk: "18:00"
    winddown: "21:00"
    stop_screens: "22:00"
    go_to_bed: "22:30"
    night: "23:00"
  - begin_wake: "06:00"
    wake: "08:00"
    dusk: "18:00"
    winddown: "22:00"
    stop_screens: "23:00"
    go_to_bed: "23:30"
    night: "00:00"
  - begin_wake: "06:00"
    wake: "08:00"
    dusk: "18:00"
    winddown: "22:00"
    stop_screens: "23:00"
    go_to_bed: "23:30"
    night: "00:00"
`
	err = os.WriteFile(filepath.Join(tmpDir, "schedule_config.yaml"), []byte(scheduleConfig), 0644)
	require.NoError(t, err)

	return tmpDir
}

func TestLoader_LoadAll(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	configDir := setupTestConfigDir(t)

	loader := NewLoader(configDir, logger)
	err := loader.LoadAll()
	require.NoError(t, err)

	// Verify music config
	musicConfig := loader.GetMusicConfig()
	assert.NotNil(t, musicConfig)
	assert.NotNil(t, musicConfig.Playlists)
	assert.NotNil(t, musicConfig.Speakers)
	assert.NotNil(t, musicConfig.Volumes)

	// Verify hue config
	hueConfig := loader.GetHueConfig()
	assert.NotNil(t, hueConfig)
	assert.NotNil(t, hueConfig.Lights)
	assert.NotNil(t, hueConfig.Scenes)
	assert.NotNil(t, hueConfig.Groups)

	// Verify schedule config
	scheduleConfig := loader.GetScheduleConfig()
	assert.NotNil(t, scheduleConfig)
	assert.Equal(t, 7, len(scheduleConfig.Schedule), "Should have 7 days of schedule")
}

func TestLoader_LoadMusicConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	configDir := setupTestConfigDir(t)

	loader := NewLoader(configDir, logger)
	err := loader.LoadMusicConfig()
	require.NoError(t, err)

	config := loader.GetMusicConfig()
	assert.NotNil(t, config)
	assert.Contains(t, config.Playlists, "morning")
	assert.Contains(t, config.Speakers, "living_room")
}

func TestLoader_LoadHueConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	configDir := setupTestConfigDir(t)

	loader := NewLoader(configDir, logger)
	err := loader.LoadHueConfig()
	require.NoError(t, err)

	config := loader.GetHueConfig()
	assert.NotNil(t, config)
	assert.Contains(t, config.Lights, "living_room")
	assert.Contains(t, config.Scenes, "morning")
}

func TestLoader_LoadScheduleConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	configDir := setupTestConfigDir(t)

	loader := NewLoader(configDir, logger)
	err := loader.LoadScheduleConfig()
	require.NoError(t, err)

	config := loader.GetScheduleConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 7, len(config.Schedule))

	// Check first entry (Sunday)
	sunday := config.Schedule[0]
	assert.Equal(t, "05:00", sunday.BeginWake)
	assert.Equal(t, "07:00", sunday.Wake)
	assert.Equal(t, "23:00", sunday.Night)
}

func TestLoader_GetTodaysSchedule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	configDir := setupTestConfigDir(t)

	loader := NewLoader(configDir, logger)
	err := loader.LoadScheduleConfig()
	require.NoError(t, err)

	schedule, err := loader.GetTodaysSchedule()
	require.NoError(t, err)
	assert.NotNil(t, schedule)

	// Verify times are parsed correctly
	assert.False(t, schedule.BeginWake.IsZero())
	assert.False(t, schedule.Wake.IsZero())
	assert.False(t, schedule.Dusk.IsZero())
	assert.False(t, schedule.Winddown.IsZero())
	assert.False(t, schedule.Night.IsZero())

	// Verify times are for today
	// The hour will depend on what day of the week it is
	// Sunday (0): 05:00, Monday (1): 05:30, Tue-Fri (2-5): 05:00, Sat-Sun (6): 06:00
	hour := schedule.BeginWake.Hour()
	assert.True(t, hour >= 5 && hour <= 6, "Begin wake hour should be between 5 and 6")
	assert.Equal(t, 0, schedule.BeginWake.Minute())
}

func TestLoader_MissingFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	configDir := t.TempDir() // Empty directory

	loader := NewLoader(configDir, logger)
	err := loader.LoadAll()
	assert.Error(t, err)
}
