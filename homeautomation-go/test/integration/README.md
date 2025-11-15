# Integration Tests

Comprehensive integration tests for the Home Automation Go client with a full mock Home Assistant WebSocket server.

## What This Tests

### 1. Mock Home Assistant Server
- Full WebSocket API implementation
- Simulates authentication flow
- State storage and retrieval
- Service call handling
- Event broadcasting with configurable delays
- Network latency simulation

### 2. Test Scenarios

#### Basic Functionality
- ✅ Connection and authentication
- ✅ Initial state synchronization
- ✅ State updates (Get/Set operations)
- ✅ All state types (bool, number, string, JSON)

#### Concurrency & Race Conditions
- ✅ Concurrent reads (50 goroutines × 100 reads)
- ✅ Concurrent writes (20 goroutines × 50 writes)
- ✅ Mixed read/write operations
- ✅ CompareAndSwap atomic operations

#### Deadlock Detection
- ✅ Subscription callbacks that trigger more state operations
- ✅ Rapid state changes from both server and client
- ✅ Multiple subscribers on same entity
- ✅ Subscription unsubscribe behavior

#### Edge Cases
- ✅ High-frequency state changes (1000+ changes)
- ✅ Multiple subscribers on same entity (tests subscription leak bug)
- ✅ Reconnection after disconnect
- ✅ Network latency handling

## Running the Tests

### Run Locally (Quick)
```bash
cd homeautomation-go
go test -v -race ./test/integration/...
```

### Run with Docker (Isolated Environment)
```bash
docker-compose -f docker-compose.integration.yml up --build
```

### Run Specific Tests
```bash
# Test only deadlock scenarios
go test -v -race -run TestSubscription ./test/integration/

# Test only concurrent operations
go test -v -race -run TestConcurrent ./test/integration/

# Test subscription bug
go test -v -run TestMultipleSubscribers ./test/integration/
```

### Run with Race Detector (Recommended)
```bash
go test -v -race -timeout=5m ./test/integration/...
```

## Expected Failures (Known Bugs)

### 1. TestMultipleSubscribersOnSameEntity - WILL FAIL ❌

**Bug**: Unsubscribe deletes ALL subscribers, not just one

**Location**: `internal/ha/client.go:422-428`

**What happens**:
```go
// BUG: Deletes entire subscriber slice
func (c *Client) unsubscribe(entityID string) error {
    c.subsMu.Lock()
    delete(c.subscribers, entityID)  // ❌ Removes ALL handlers
    c.subsMu.Unlock()
    return nil
}
```

**Expected behavior**:
- Subscribe 3 handlers to same entity
- Unsubscribe 1 handler
- Other 2 should still receive events

**Actual behavior**:
- Unsubscribing 1 handler removes ALL handlers
- No handlers receive subsequent events

**Test output**:
```
After unsubscribe one: count1=1, count2=1, count3=1
Expected count1=2, got 1 (Subscriber 1 should still be called)
Expected count3=2, got 1 (Subscriber 3 should still be called)
```

## What Each Test Does

### TestBasicConnection
- Verifies connection to mock HA server
- Tests initial state sync
- Confirms state updates propagate both ways

### TestStateChangeSubscription
- Subscribes to state changes
- Server triggers state change
- Verifies callback receives correct old/new values

### TestConcurrentReads
- Spawns 50 goroutines
- Each performs 100 concurrent reads
- Ensures no deadlocks on read operations

### TestConcurrentWrites
- Spawns 20 goroutines writing concurrently
- Tests mutex contention
- Ensures no race conditions or deadlocks

### TestConcurrentReadsAndWrites
- Mixed workload: 10 readers + 5 writers
- Runs for 3 seconds continuously
- Tests real-world concurrent access patterns

### TestSubscriptionWithConcurrentWrites ⚠️ **CRITICAL**
- Subscribes to state changes
- Callback triggers MORE state operations (potential deadlock)
- Server and client both trigger rapid changes
- Tests the exact scenario that could cause deadlock

**This test specifically checks for**:
1. Callback acquires lock to read state → ✅ Should work (lock released)
2. Callback calls SetBool → ✅ Should work (new lock acquisition)
3. SetBool triggers HA update → state change event → another callback → potential recursion

### TestMultipleSubscribersOnSameEntity ⚠️ **WILL FAIL**
- Demonstrates the subscription leak bug
- Shows unsubscribe removes ALL handlers

### TestCompareAndSwapRaceCondition
- 20 goroutines compete for CAS operation
- Tests atomicity of CompareAndSwapBool
- Ensures only one succeeds at acquiring lock

### TestReconnection
- Stops server mid-connection
- Verifies client detects disconnect
- Restarts server
- Confirms auto-reconnection with exponential backoff

### TestHighFrequencyStateChanges
- Sends 1000 rapid state changes
- Tests system under high load
- Verifies most events are received (allows ~20% loss)

### TestAllStateTypes
- Comprehensive test of all type operations
- Boolean, Number, String, JSON
- Ensures type safety and conversions work

## Debugging Tips

### If Tests Hang (Deadlock)
```bash
# Run with timeout and race detector
go test -v -race -timeout=30s ./test/integration/

# Get goroutine dump on timeout
GODEBUG=gctrace=1 go test -v -timeout=10s ./test/integration/
```

### View Detailed Logs
```bash
go test -v ./test/integration/ 2>&1 | tee test-output.log
```

### Check for Race Conditions
```bash
# Race detector will print detailed reports
go test -race ./test/integration/
```

## Mock Server Features

### Configurable Event Delay
```go
server := NewMockHAServer(addr, token)
server.SetEventDelay(100 * time.Millisecond) // Simulate network latency
```

### Manual State Changes
```go
server.SetState("input_boolean.test", "on", map[string]interface{}{})
// Automatically broadcasts state_changed event to all clients
```

### State Inspection
```go
state := server.GetState("input_boolean.test")
fmt.Printf("Current state: %s\n", state.State)
```

## Continuous Integration

Add to `.github/workflows/test.yml`:
```yaml
- name: Run Integration Tests
  run: docker-compose -f docker-compose.integration.yml up --abort-on-container-exit
```

## Performance Benchmarks

Expected performance on typical hardware:

- **Connection time**: <100ms
- **State sync (27 vars)**: <200ms
- **State change latency**: <50ms
- **Concurrent operations**: 5000+ ops/sec
- **No deadlocks**: Even under extreme concurrent load

## Known Issues

1. ✅ **Deadlock in SetBool** - FALSE ALARM (code is correct, lock released before client call)
2. ❌ **Subscription memory leak** - CONFIRMED (TestMultipleSubscribersOnSameEntity fails)
3. ⚠️ **Mock client lock bug** - In test code only (`mock.go:263`)

## Next Steps

After fixing the subscription bug:
1. All tests should pass
2. Add benchmarks for performance regression testing
3. Add fuzzing tests for edge cases
4. Test with real Home Assistant instance
