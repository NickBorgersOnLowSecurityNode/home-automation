package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homeautomation/internal/ha"
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
	stateManager := state.NewManager(client, logger)

	// Sync all state from HA
	if err := stateManager.SyncFromHA(); err != nil {
		logger.Fatal("Failed to sync state from HA", zap.Error(err))
	}

	// Display current state
	displayState(stateManager, logger)

	// Subscribe to interesting state changes
	subscribeToChanges(stateManager, logger)

	// Demonstrate setting values (only in read-write mode)
	if !readOnly {
		demonstrateStateChanges(stateManager, logger)
	} else {
		logger.Info("Running in READ-ONLY mode - no changes will be made to Home Assistant")
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
	// Subscribe to home presence changes
	manager.Subscribe("isNickHome", func(key string, oldValue, newValue interface{}) {
		logger.Info("State changed",
			zap.String("key", key),
			zap.Any("old", oldValue),
			zap.Any("new", newValue))
	})

	manager.Subscribe("isCarolineHome", func(key string, oldValue, newValue interface{}) {
		logger.Info("State changed",
			zap.String("key", key),
			zap.Any("old", oldValue),
			zap.Any("new", newValue))
	})

	manager.Subscribe("isAnyoneHome", func(key string, oldValue, newValue interface{}) {
		logger.Info("State changed",
			zap.String("key", key),
			zap.Any("old", oldValue),
			zap.Any("new", newValue))
	})

	// Subscribe to sleep status
	manager.Subscribe("isEveryoneAsleep", func(key string, oldValue, newValue interface{}) {
		logger.Info("State changed",
			zap.String("key", key),
			zap.Any("old", oldValue),
			zap.Any("new", newValue))
	})

	// Subscribe to day phase
	manager.Subscribe("dayPhase", func(key string, oldValue, newValue interface{}) {
		logger.Info("State changed",
			zap.String("key", key),
			zap.Any("old", oldValue),
			zap.Any("new", newValue))
	})

	// Subscribe to energy availability
	manager.Subscribe("isFreeEnergyAvailable", func(key string, oldValue, newValue interface{}) {
		logger.Info("State changed",
			zap.String("key", key),
			zap.Any("old", oldValue),
			zap.Any("new", newValue))
	})

	logger.Info("Subscribed to state change notifications")
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
