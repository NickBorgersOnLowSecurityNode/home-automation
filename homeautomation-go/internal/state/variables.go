package state

// StateType represents the type of a state variable
type StateType string

const (
	TypeBool   StateType = "bool"
	TypeString StateType = "string"
	TypeNumber StateType = "number"
	TypeJSON   StateType = "json"
)

// StateVariable defines metadata for a state variable
type StateVariable struct {
	Key       string      // Go variable name (e.g., "isNickHome")
	EntityID  string      // HA entity ID (e.g., "input_boolean.nick_home")
	Type      StateType   // bool, string, number, json
	Default   interface{} // Default value
	ReadOnly  bool        // Whether it's read-only from HA
	LocalOnly bool        // If true, only exists in memory, not synced with HA
}

// AllVariables contains all 28 state variables (27 synced with HA + 1 local-only)
var AllVariables = []StateVariable{
	// Booleans (18)
	{Key: "isNickHome", EntityID: "input_boolean.nick_home", Type: TypeBool, Default: false},
	{Key: "isCarolineHome", EntityID: "input_boolean.caroline_home", Type: TypeBool, Default: false},
	{Key: "isToriHere", EntityID: "input_boolean.tori_here", Type: TypeBool, Default: false},
	{Key: "isAnyOwnerHome", EntityID: "input_boolean.any_owner_home", Type: TypeBool, Default: false},
	{Key: "isAnyoneHome", EntityID: "input_boolean.anyone_home", Type: TypeBool, Default: false},
	{Key: "isMasterAsleep", EntityID: "input_boolean.master_asleep", Type: TypeBool, Default: false},
	{Key: "isGuestAsleep", EntityID: "input_boolean.guest_asleep", Type: TypeBool, Default: false},
	{Key: "isAnyoneAsleep", EntityID: "input_boolean.anyone_asleep", Type: TypeBool, Default: false},
	{Key: "isEveryoneAsleep", EntityID: "input_boolean.everyone_asleep", Type: TypeBool, Default: false},
	{Key: "isGuestBedroomDoorOpen", EntityID: "input_boolean.guest_bedroom_door_open", Type: TypeBool, Default: false},
	{Key: "isHaveGuests", EntityID: "input_boolean.have_guests", Type: TypeBool, Default: false},
	{Key: "isAppleTVPlaying", EntityID: "input_boolean.apple_tv_playing", Type: TypeBool, Default: false},
	{Key: "isTVPlaying", EntityID: "input_boolean.tv_playing", Type: TypeBool, Default: false},
	{Key: "isTVon", EntityID: "input_boolean.tv_on", Type: TypeBool, Default: false},
	{Key: "isFadeOutInProgress", EntityID: "input_boolean.fade_out_in_progress", Type: TypeBool, Default: false},
	{Key: "isFreeEnergyAvailable", EntityID: "input_boolean.free_energy_available", Type: TypeBool, Default: false},
	{Key: "isGridAvailable", EntityID: "input_boolean.grid_available", Type: TypeBool, Default: true},
	{Key: "isExpectingSomeone", EntityID: "input_boolean.expecting_someone", Type: TypeBool, Default: false},

	// Numbers (3)
	{Key: "alarmTime", EntityID: "input_number.alarm_time", Type: TypeNumber, Default: 0.0},
	{Key: "remainingSolarGeneration", EntityID: "input_number.remaining_solar_generation", Type: TypeNumber, Default: 0.0},
	{Key: "thisHourSolarGeneration", EntityID: "input_number.this_hour_solar_generation", Type: TypeNumber, Default: 0.0},

	// Text (6)
	{Key: "dayPhase", EntityID: "input_text.day_phase", Type: TypeString, Default: ""},
	{Key: "sunevent", EntityID: "input_text.sun_event", Type: TypeString, Default: ""},
	{Key: "musicPlaybackType", EntityID: "input_text.music_playback_type", Type: TypeString, Default: ""},
	{Key: "batteryEnergyLevel", EntityID: "input_text.battery_energy_level", Type: TypeString, Default: ""},
	{Key: "currentEnergyLevel", EntityID: "input_text.current_energy_level", Type: TypeString, Default: ""},
	{Key: "solarProductionEnergyLevel", EntityID: "input_text.solar_production_energy_level", Type: TypeString, Default: ""},

	// JSON - Local Only (not synced with HA)
	{Key: "currentlyPlayingMusic", EntityID: "", Type: TypeJSON, Default: map[string]interface{}{}, LocalOnly: true},
}

// VariablesByKey creates a map of variables by their key
func VariablesByKey() map[string]StateVariable {
	vars := make(map[string]StateVariable)
	for _, v := range AllVariables {
		vars[v.Key] = v
	}
	return vars
}

// VariablesByEntityID creates a map of variables by their entity ID
func VariablesByEntityID() map[string]StateVariable {
	vars := make(map[string]StateVariable)
	for _, v := range AllVariables {
		vars[v.EntityID] = v
	}
	return vars
}
