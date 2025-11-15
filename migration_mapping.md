# Node Red to Home Assistant Variable Mapping

This document maps Node Red global state variables to their Home Assistant entity equivalents.

## Migration Summary

- **Total Node Red state variables**: 58
- **Variables in disabled flows (SKIP)**: 25
- **Active variables to migrate**: 33
- **Already exist in Home Assistant**: 10
- **Need to create**: 23

---

## ✅ ALREADY EXISTS - Will Sync With Existing Entities (10)

These entities already exist in Home Assistant and will be synchronized with Node Red.

| Node Red Variable | Home Assistant Entity | Type | Action |
|------------------|----------------------|------|--------|
| isCarolineHome | input_boolean.caroline_home | Boolean | Sync only |
| isExpectingSomeone | input_boolean.expecting_someone | Boolean | Sync only |
| isGridAvailable | input_boolean.grid_available | Boolean | Sync only |
| isNickHome | input_boolean.nick_home | Boolean | Sync only |
| isToriHere | input_boolean.tori_here | Boolean | Sync only |
| batteryEnergyLevel | input_text.battery_energy_level | String | Sync only |
| currentEnergyLevel | input_text.current_energy_level | String | Sync only |
| dayPhase | input_text.day_phase | String | Sync only |
| musicPlaybackType | input_text.music_playback_type | String | Sync only |
| solarProductionEnergyLevel | input_text.solar_production_energy_level | String | Sync only |

---

## 🆕 NEED TO CREATE - Boolean Variables (13)

| Node Red Variable | Home Assistant Entity | Description | Action |
|------------------|----------------------|-------------|--------|
| isAnyOwnerHome | input_boolean.any_owner_home | Whether any owner is home | Create & sync |
| isAnyoneAsleep | input_boolean.anyone_asleep | Whether anyone is asleep | Create & sync |
| isAnyoneHome | input_boolean.anyone_home | Whether anyone is home | Create & sync |
| isAppleTVPlaying | input_boolean.apple_tv_playing | Apple TV playback status | Create & sync |
| isEveryoneAsleep | input_boolean.everyone_asleep | Everyone asleep status | Create & sync |
| isFadeOutInProgress | input_boolean.fade_out_in_progress | Music fade out status | Create & sync |
| isFreeEnergyAvailable | input_boolean.free_energy_available | Free energy availability | Create & sync |
| isGuestAsleep | input_boolean.guest_asleep | Guest sleep status | Create & sync |
| isGuestBedroomDoorOpen | input_boolean.guest_bedroom_door_open | Guest bedroom door state | Create & sync |
| isHaveGuests | input_boolean.have_guests | Guest presence | Create & sync |
| isMasterAsleep | input_boolean.master_asleep | Master bedroom sleep status | Create & sync |
| isTVPlaying | input_boolean.tv_playing | TV playback status | Create & sync |
| isTVon | input_boolean.tv_on | TV power status | Create & sync |

---

## 🆕 NEED TO CREATE - Numeric Variables (3)

| Node Red Variable | Home Assistant Entity | Type | Min | Max | Step | Unit | Action |
|------------------|----------------------|------|-----|-----|------|------|--------|
| alarmTime | input_number.alarm_time | Number (timestamp) | 0 | 2147483647 | 1 | ms | Create & sync |
| remainingSolarGeneration | input_number.remaining_solar_generation | Number | 0 | 100000 | 0.1 | kWh | Create & sync |
| thisHourSolarGeneration | input_number.this_hour_solar_generation | Number | 0 | 100000 | 0.1 | kW | Create & sync |

---

## 🆕 NEED TO CREATE - Text Variables (1)

| Node Red Variable | Home Assistant Entity | Description | Example Values | Action |
|------------------|----------------------|-------------|----------------|--------|
| sunevent | input_text.sunevent | Current sun event | "morning", "day", "sunset", "dusk", "night" | Create & sync |

---

## 🆕 NEED TO CREATE - JSON Object Variables (1)

These are complex objects stored as JSON strings in input_text entities.

| Node Red Variable | Home Assistant Entity | Max Length | Description | Action |
|------------------|----------------------|------------|-------------|--------|
| currentlyPlayingMusic | input_text.currently_playing_music | 4096 | Current music playback info (JSON) | Create & sync |

---

## ⏭️ SKIPPED - Variables Only in Disabled Flows (25)

These variables are only referenced in disabled Node Red flows and will NOT be migrated.

| Node Red Variable | Disabled Flow | Reason |
|------------------|---------------|--------|
| currentClimate | Air Condition | Flow disabled |
| desiredHumidityOfMasterBedroom | Air Condition | Flow disabled |
| formaldehydeOfBedroom | Air Condition | Flow disabled |
| formaldehydeOfLivingRoom | Air Condition | Flow disabled |
| formaldehydeOfMasterBedroom | Air Condition | Flow disabled |
| humidityOfBedroom | Air Condition | Flow disabled |
| humidityOfLivingRoomCenter | Air Condition | Flow disabled |
| humidityOfLivingRoomWindow | Air Condition | Flow disabled |
| humidityOfMasterBedroom | Air Condition | Flow disabled |
| humidityOfOutside | Air Condition | Flow disabled |
| isHumidifierOn | Air Condition | Flow disabled |
| keepPoolPumpOnFor24Hours | Pool Pump | Flow disabled |
| lastVacuumingTimestamp | Vacuum | Flow disabled |
| outdoorTemperature | Air Condition | Flow disabled |
| pm25OfBedroom | Air Condition | Flow disabled |
| pm25OfLivingRoom | Air Condition | Flow disabled |
| pm25OfMasterBedroom | Air Condition | Flow disabled |
| temperatureOfBedroom | Air Condition | Flow disabled |
| temperatureOfLivingRoomCenter | Air Condition | Flow disabled |
| temperatureOfLivingRoomWindow | Air Condition | Flow disabled |
| temperatureOfMasterBedroom | Air Condition | Flow disabled |
| temperatureOfOutside | Air Condition | Flow disabled |
| vocOfBedroom | Air Condition | Flow disabled |
| vocOfLivingRoom | Air Condition | Flow disabled |
| vocOfMasterBedroom | Air Condition | Flow disabled |

---

## Implementation Notes

### Entity Creation
- Entities will be created via Home Assistant REST API
- input_boolean: Simple on/off entities
- input_number: Numeric entities with appropriate min/max/step values
- input_text: Text entities for strings and JSON-serialized objects

### Synchronization Strategy
1. **On Node Red startup**: Read all 33 variables from Home Assistant → initialize Node Red state
2. **On Node Red variable change**: Write value to corresponding Home Assistant entity
3. **During migration**: Both systems share state via Home Assistant entities
