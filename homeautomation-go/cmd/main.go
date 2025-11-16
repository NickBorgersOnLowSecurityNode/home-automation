package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"homeautomation/internal/ha"
	"homeautomation/internal/loadshedding"
	"homeautomation/internal/plugins/energy"
	"homeautomation/internal/plugins/lighting"
	"homeautomation/internal/plugins/music"
	"homeautomation/internal/state"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using environment variables")
	}

	haURL := os.Getenv("HA_URL")
	haToken := os.Getenv("HA_TOKEN")
	readOnly := os.Getenv("READ_ONLY") == "true"

	if haURL == "" || haToken == "" {
		logger.Fatal("HA_URL and HA_TOKEN environment variables must be set")
	}

	// Determine config directory path
	// Priority: CONFIG_DIR env var > ./configs (container) > ../configs (local dev)
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		// Auto-detect: prefer ./configs if it exists (container), otherwise ../configs (local dev)
		if _, err := os.Stat("./configs"); err == nil {
			configDir = "./configs"
		} else {
			configDir = "../configs"
		}
	}
	logger.Info("Using config directory", zap.String("path", configDir))

	logger.Info("Starting Home Automation Client",
		zap.String("url", haURL),
		zap.Bool("read_only", readOnly))

	// Create HA client
	client := ha.NewClient(haURL, haToken, logger)

	// Connect to Home Assistant
	if err := client.Connect(); err != nil {
		logger.Fatal("Failed to connect to Home Assistant", zap.Error(err))
	}
	defer client.Disconnect()

	logger.Info("Connected to Home Assistant")

	// Create State Manager
	stateManager := state.NewManager(client, logger, readOnly)

	// Sync all state from HA
	if err := stateManager.SyncFromHA(); err != nil {
		logger.Fatal("Failed to sync state from HA", zap.Error(err))
	}

	// Display current state
	displayState(stateManager, logger)

	// Subscribe to interesting state changes
	subscribeToChanges(stateManager, logger)

	// Start Energy State Manager
	energyManager, err := startEnergyManager(client, stateManager, logger, readOnly, configDir)
	if err != nil {
		logger.Fatal("Failed to start Energy State Manager", zap.Error(err))
	}
	defer energyManager.Stop()

	// Start Music Manager
	musicManager, err := startMusicManager(client, stateManager, logger, readOnly, configDir)
	if err != nil {
		logger.Fatal("Failed to start Music Manager", zap.Error(err))
	}
	defer musicManager.Stop()

	// Start Lighting Manager
	lightingManager, err := startLightingManager(client, stateManager, logger, readOnly, configDir)
	if err != nil {
		logger.Fatal("Failed to start Lighting Manager", zap.Error(err))
	}
	defer lightingManager.Stop()

	// Start Load Shedding controller
	loadSheddingController := loadshedding.New(stateManager, client, logger)
	if err := loadSheddingController.Start(); err != nil {
		logger.Fatal("Failed to start Load Shedding controller", zap.Error(err))
	}
	defer loadSheddingController.Stop()
	logger.Info("Load Shedding controller started")

	// Demonstrate setting values (only in read-write mode)
	if !readOnly {
		demonstrateStateChanges(stateManager, logger)
	} else {
		logger.Info("Running in READ-ONLY mode - state monitoring active")
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Application running. Press Ctrl+C to exit.")
	if readOnly {
		logger.Info("Monitoring state changes in READ-ONLY mode...")
	} else {
		logger.Info("Monitoring state changes...")
	}

	// Wait for shutdown signal
	<-sigChan

	logger.Info("Shutting down gracefully...")
}

func displayState(manager *state.Manager, logger *zap.Logger) {
	logger.Info("=== Current State ===")

	// Display booleans
	logger.Info("--- Boolean Variables ---")
	boolVars := []string{
		"isNickHome", "isCarolineHome", "isToriHere",
		"isAnyOwnerHome", "isAnyoneHome",
		"isMasterAsleep", "isGuestAsleep", "isAnyoneAsleep", "isEveryoneAsleep",
		"isGuestBedroomDoorOpen", "isHaveGuests",
		"isAppleTVPlaying", "isTVPlaying", "isTVon",
		"isFadeOutInProgress", "isFreeEnergyAvailable", "isGridAvailable",
		"isExpectingSomeone",
	}

	for _, key := range boolVars {
		value, err := manager.GetBool(key)
		if err != nil {
			logger.Error("Failed to get bool", zap.String("key", key), zap.Error(err))
			continue
		}
		logger.Info(fmt.Sprintf("  %s: %v", key, value))
	}

	// Display numbers
	logger.Info("--- Number Variables ---")
	numVars := []string{"alarmTime", "remainingSolarGeneration", "thisHourSolarGeneration"}

	for _, key := range numVars {
		value, err := manager.GetNumber(key)
		if err != nil {
			logger.Error("Failed to get number", zap.String("key", key), zap.Error(err))
			continue
		}
		logger.Info(fmt.Sprintf("  %s: %.2f", key, value))
	}

	// Display strings
	logger.Info("--- String Variables ---")
	strVars := []string{
		"dayPhase", "sunevent", "musicPlaybackType",
		"batteryEnergyLevel", "currentEnergyLevel", "solarProductionEnergyLevel",
	}

	for _, key := range strVars {
		value, err := manager.GetString(key)
		if err != nil {
			logger.Error("Failed to get string", zap.String("key", key), zap.Error(err))
			continue
		}
		logger.Info(fmt.Sprintf("  %s: %s", key, value))
	}

	logger.Info("======================")
}

func subscribeToChanges(manager *state.Manager, logger *zap.Logger) {
	// Subscribe to all state variables
	for _, variable := range state.AllVariables {
		key := variable.Key
		manager.Subscribe(key, func(varKey string, oldValue, newValue interface{}) {
			logger.Info("State changed",
				zap.String("key", varKey),
				zap.Any("old", oldValue),
				zap.Any("new", newValue))
		})
	}

	logger.Info("Subscribed to all state change notifications",
		zap.Int("variable_count", len(state.AllVariables)))
}

func demonstrateStateChanges(manager *state.Manager, logger *zap.Logger) {
	logger.Info("=== Demonstrating State Changes ===")

	// Example 1: Toggle a boolean
	logger.Info("Example 1: Toggling isExpectingSomeone")
	currentValue, _ := manager.GetBool("isExpectingSomeone")
	logger.Info(fmt.Sprintf("  Current value: %v", currentValue))

	newValue := !currentValue
	if err := manager.SetBool("isExpectingSomeone", newValue); err != nil {
		logger.Error("Failed to set isExpectingSomeone", zap.Error(err))
	} else {
		logger.Info(fmt.Sprintf("  Set to: %v", newValue))
	}

	time.Sleep(1 * time.Second)

	// Set it back
	if err := manager.SetBool("isExpectingSomeone", currentValue); err != nil {
		logger.Error("Failed to restore isExpectingSomeone", zap.Error(err))
	} else {
		logger.Info(fmt.Sprintf("  Restored to: %v", currentValue))
	}

	// Example 2: Update a text value
	logger.Info("Example 2: Reading dayPhase")
	dayPhase, _ := manager.GetString("dayPhase")
	logger.Info(fmt.Sprintf("  Current dayPhase: %s", dayPhase))

	// Example 3: CompareAndSwap
	logger.Info("Example 3: Using CompareAndSwapBool")
	currentFade, _ := manager.GetBool("isFadeOutInProgress")
	swapped, err := manager.CompareAndSwapBool("isFadeOutInProgress", currentFade, !currentFade)
	if err != nil {
		logger.Error("Failed to CompareAndSwap", zap.Error(err))
	} else {
		logger.Info(fmt.Sprintf("  Swapped: %v (from %v to %v)", swapped, currentFade, !currentFade))
		// Restore
		manager.SetBool("isFadeOutInProgress", currentFade)
	}

	logger.Info("===================================")
}

func startEnergyManager(client ha.HAClient, stateManager *state.Manager, logger *zap.Logger, readOnly bool, configDir string) (*energy.Manager, error) {
	// Load energy configuration
	configPath := filepath.Join(configDir, "energy_config.yaml")
	energyConfig, err := energy.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load energy config: %w", err)
	}

	logger.Info("Loaded energy configuration",
		zap.Int("energy_states", len(energyConfig.Energy.EnergyStates)),
		zap.String("free_energy_start", energyConfig.Energy.FreeEnergyTime.Start),
		zap.String("free_energy_end", energyConfig.Energy.FreeEnergyTime.End))

	// Create and start energy manager
	energyManager := energy.NewManager(client, stateManager, energyConfig, logger, readOnly)
	if err := energyManager.Start(); err != nil {
		return nil, fmt.Errorf("failed to start energy manager: %w", err)
	}

	return energyManager, nil
}

func startMusicManager(client ha.HAClient, stateManager *state.Manager, logger *zap.Logger, readOnly bool, configDir string) (*music.Manager, error) {
	// Load music configuration
	configPath := filepath.Join(configDir, "music_config.yaml")
	musicConfig, err := music.LoadMusicConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load music config: %w", err)
	}

	// Count total music modes
	logger.Info("Loaded music configuration",
		zap.Int("music_modes", len(musicConfig.Music)))

	// Create and start music manager
	musicManager := music.NewManager(client, stateManager, musicConfig, logger, readOnly)
	if err := musicManager.Start(); err != nil {
		return nil, fmt.Errorf("failed to start music manager: %w", err)
	}

	logger.Info("Music Manager started successfully")
	return musicManager, nil
}

func startLightingManager(client ha.HAClient, stateManager *state.Manager, logger *zap.Logger, readOnly bool, configDir string) (*lighting.Manager, error) {
	// Load lighting configuration
	configPath := filepath.Join(configDir, "hue_config.yaml")
	lightingConfig, err := lighting.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lighting config: %w", err)
	}

	logger.Info("Loaded lighting configuration",
		zap.Int("rooms", len(lightingConfig.Rooms)))

	// Create and start lighting manager
	lightingManager := lighting.NewManager(client, stateManager, lightingConfig, logger, readOnly)
	if err := lightingManager.Start(); err != nil {
		return nil, fmt.Errorf("failed to start lighting manager: %w", err)
	}

	logger.Info("Lighting Manager started successfully")
	return lightingManager, nil
}

