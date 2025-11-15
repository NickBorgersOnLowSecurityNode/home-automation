package lighting

import (
	"os"

	"gopkg.in/yaml.v3"
)

// RoomConfig represents the configuration for a single room/area
type RoomConfig struct {
	HueGroup                  string      `yaml:"hue_group"`
	HASSAreaID                string      `yaml:"hass_area_id"`
	OnIfTrue                  interface{} `yaml:"on_if_true"`  // Can be string or []string
	OnIfFalse                 interface{} `yaml:"on_if_false"` // Can be string or []string
	OffIfTrue                 interface{} `yaml:"off_if_true"` // Can be string or []string
	OffIfFalse                interface{} `yaml:"off_if_false"` // Can be string or []string
	IncreaseBrightnessIfTrue  interface{} `yaml:"increase_brightness_if_true"` // Can be string or []string
	TransitionSeconds         *int        `yaml:"transition_seconds"` // Pointer to handle nil/~ values
}

// GetOnIfTrueConditions returns the list of on_if_true conditions
func (r *RoomConfig) GetOnIfTrueConditions() []string {
	return interfaceToStringSlice(r.OnIfTrue)
}

// GetOnIfFalseConditions returns the list of on_if_false conditions
func (r *RoomConfig) GetOnIfFalseConditions() []string {
	return interfaceToStringSlice(r.OnIfFalse)
}

// GetOffIfTrueConditions returns the list of off_if_true conditions
func (r *RoomConfig) GetOffIfTrueConditions() []string {
	return interfaceToStringSlice(r.OffIfTrue)
}

// GetOffIfFalseConditions returns the list of off_if_false conditions
func (r *RoomConfig) GetOffIfFalseConditions() []string {
	return interfaceToStringSlice(r.OffIfFalse)
}

// GetIncreaseBrightnessIfTrueConditions returns the list of increase_brightness_if_true conditions
func (r *RoomConfig) GetIncreaseBrightnessIfTrueConditions() []string {
	return interfaceToStringSlice(r.IncreaseBrightnessIfTrue)
}

// interfaceToStringSlice converts an interface{} that can be string, []string, or nil to []string
func interfaceToStringSlice(val interface{}) []string {
	if val == nil {
		return []string{}
	}

	switch v := val.(type) {
	case string:
		if v == "" {
			return []string{}
		}
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	default:
		return []string{}
	}
}

// HueConfig represents the Hue lighting configuration
type HueConfig struct {
	Rooms []RoomConfig `yaml:"rooms"`
}

// LoadConfig loads the Hue configuration from a YAML file
func LoadConfig(path string) (*HueConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config HueConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
