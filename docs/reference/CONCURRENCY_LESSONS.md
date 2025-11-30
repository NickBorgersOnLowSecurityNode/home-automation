# Concurrency Lessons - WebSocket & State Management

## Overview

This document explains critical concurrency patterns used in the Go implementation. These patterns emerged from bugs discovered during integration testing and represent important lessons for working with WebSockets and concurrent state management.

---

## Lesson 1: WebSocket Writes Must Be Serialized

**Pattern**: All WebSocket writes must be protected by a mutex.

**Why**: The `gorilla/websocket` library is **NOT thread-safe for writes**. Multiple goroutines writing to the same connection will cause:
```
panic: concurrent write to websocket connection
```

**Implementation**:
```go
type Client struct {
    conn    *websocket.Conn
    writeMu sync.Mutex  // Protects all writes to conn
    // ...
}

func (c *Client) sendMessage(msg interface{}) error {
    c.writeMu.Lock()
    err := c.conn.WriteJSON(msg)
    c.writeMu.Unlock()
    return err
}
```

**Where to Apply**:
- `internal/ha/client.go` - All WebSocket write operations
- Any future WebSocket client implementations
- Test mock servers that broadcast to multiple connections

**Tests That Validate This**:
- `TestConcurrentWrites` - 20 goroutines writing simultaneously
- `TestConcurrentReadsAndWrites` - Mixed read/write workload

---

## Lesson 2: Subscription Tracking Requires Per-Handler IDs

**Pattern**: Track individual subscription handlers, not just by entity ID.

**Why**: Multiple handlers can subscribe to the same entity. Unsubscribing one must not affect others.

**Wrong Approach** (causes memory leak):
```go
// ❌ BAD: Deletes ALL handlers for entity
func (c *Client) unsubscribe(entityID string) {
    delete(c.subscribers, entityID)  // Removes all handlers!
}
```

**Correct Approach**:
```go
// ✅ GOOD: Track individual subscriptions
type subscription struct {
    id       string
    entityID string
    handler  func(state)
}

// Store by subscription ID, not entity ID
subscribers map[string]*subscription

func (c *Client) unsubscribe(subID string) {
    delete(c.subscribers, subID)  // Removes only this handler
}
```

**Where to Apply**:
- `internal/ha/client.go` - Subscription management
- `internal/state/manager.go` - State change notifications
- Any pub/sub or event handler system

**Test That Validates This**:
- `TestMultipleSubscribersOnSameEntity` - 3 handlers on same entity, unsubscribe one

---

## Lesson 3: Use RWMutex for Read-Heavy Workloads

**Pattern**: Use `sync.RWMutex` when reads vastly outnumber writes.

**Why**: Allows multiple concurrent readers while still protecting against concurrent writes.

**Implementation**:
```go
type Manager struct {
    cacheMu sync.RWMutex
    cache   map[string]interface{}
}

func (m *Manager) Get(key string) interface{} {
    m.cacheMu.RLock()         // Multiple readers OK
    defer m.cacheMu.RUnlock()
    return m.cache[key]
}

func (m *Manager) Set(key string, val interface{}) {
    m.cacheMu.Lock()          // Exclusive write lock
    defer m.cacheMu.Unlock()
    m.cache[key] = val
}
```

**Performance Impact**:
- 50 goroutines × 100 concurrent reads = 5,000 operations with no contention
- Read latency: ~1-2µs
- Write latency: ~5-10µs

**Where to Apply**:
- `internal/state/manager.go` - State cache access
- Any shared cache or lookup table

---

## Lesson 4: Mock External Services for Concurrency Testing

**Pattern**: Use mock servers instead of real external services for integration tests.

**Why Mock HA Server vs Real Home Assistant**:

| Aspect | Mock Server | Real HA |
|--------|-------------|---------|
| **Isolation** | ✅ No external deps | ❌ Requires infrastructure |
| **Speed** | ✅ <30 seconds | ❌ Minutes + network latency |
| **Repeatability** | ✅ Exact same conditions | ❌ Variable state |
| **Concurrency Testing** | ✅ Can simulate 1000s of ops | ❌ Rate limited |
| **Race Detection** | ✅ `-race` flag works | ❌ Harder to reproduce |
| **CI/CD** | ✅ Runs in Docker | ❌ Needs HA instance |

**When to Use Real HA**:
- Final end-to-end validation
- Compatibility testing with specific HA versions
- Real-world performance benchmarking

**Implementation**:
- See `test/integration/mock_ha_server.go` for reference implementation
- Implements full WebSocket protocol with auth, state, subscriptions
- Can simulate disconnects, delays, and error conditions

---

## Lesson 5: Event Handlers Must Not Block Message Processing

**Pattern**: Call event handlers asynchronously in separate goroutines.

**Why**: Event handlers are invoked from the `receiveMessages()` goroutine. If a handler tries to send a message to HA and wait for a response, it will deadlock because `receiveMessages()` is blocked waiting for the handler to return.

**Deadlock Scenario** (CRITICAL BUG - Fixed in PR #XX):
```go
// ❌ BAD: Synchronous handler call causes deadlock
func (c *Client) handleEvent(msg *Message) {
    for _, entry := range entries {
        entry.handler(...)  // BLOCKS receiveMessages goroutine
        // Handler tries to send message → waits for response
        // But response can never be received because receiveMessages is blocked!
        // → 10 second timeout → error
    }
}
```

**Error Symptoms**:
```
Failed to set solarProductionEnergyLevel: failed to set HA value: timeout waiting for response
```

**Call Chain Leading to Deadlock**:
```
receiveMessages() goroutine:
  → ReadJSON() - reads state change event
  → handleEvent() - processes event
  → handler() - energy manager handler called SYNCHRONOUSLY
    → recalculateSolarProductionLevel()
      → stateManager.SetString()
        → haClient.SetInputText()
          → haClient.CallService()
            → haClient.sendMessage()
              → Waits for response with 10 second timeout
              → BUT receiveMessages() is BLOCKED waiting for handler to return
              → Response can never be received
              → DEADLOCK → timeout → error
```

**Correct Approach**:
```go
// ✅ GOOD: Async handler calls prevent deadlock
func (c *Client) handleEvent(msg *Message) {
    for _, entry := range entries {
        // Run each handler in its own goroutine
        // This allows receiveMessages to continue processing
        go entry.handler(entityID, oldState, newState)
    }
}
```

**Benefits**:
- No blocking of message processing
- Handlers can safely send messages back to HA
- Better performance (parallel handler execution)
- Prevents cascading delays when handlers are slow

**Where to Apply**:
- `internal/ha/client.go` - Event handler invocation
- Any callback/handler system invoked from a message processing loop
- Plugin systems where plugins might need to communicate back

**Test That Validates This**:
- `TestClient_HandleEventBackpressuresHandlers` - Verifies handlers don't block message processing
- Production logs showing timeout errors when handlers were synchronous

**Production Impact**:
- **Before Fix**: Frequent 10-second timeout errors in energy manager
- **After Fix**: No timeouts, all HA updates succeed immediately

---

## Lesson 6: Always Test with Race Detector

**Command**: `go test -race ./...`

**Why**: Race conditions are timing-dependent and may not manifest in normal runs.

**What It Catches**:
- Concurrent map access without locks
- Concurrent writes to shared variables
- Channel races and deadlocks

**Cost**: ~10x slower test execution, but catches critical bugs.

**CI Requirement**: All tests must pass with `-race` flag before merging.

---

## Common Pitfalls to Avoid

### 1. Forgetting to Lock Before Map Access
```go
// ❌ BAD: Race condition
m.cache[key] = value

// ✅ GOOD: Protected access
m.mu.Lock()
m.cache[key] = value
m.mu.Unlock()
```

### 2. Holding Locks Across Network Calls
```go
// ❌ BAD: Lock held during slow I/O
m.mu.Lock()
result := callAPI()  // May take seconds!
m.cache[key] = result
m.mu.Unlock()

// ✅ GOOD: Release lock before I/O
result := callAPI()
m.mu.Lock()
m.cache[key] = result
m.mu.Unlock()
```

### 3. Not Closing Channels in Cleanup
```go
// ❌ BAD: Goroutine leak
func subscribe() chan Event {
    ch := make(chan Event)
    go sendEvents(ch)  // Never stops!
    return ch
}

// ✅ GOOD: Proper cleanup
type Subscription struct {
    events chan Event
    done   chan struct{}
}

func (s *Subscription) Close() {
    close(s.done)  // Signals goroutine to stop
}
```

---

## Key Takeaways

1. **WebSocket writes need mutex protection** - gorilla/websocket is not thread-safe
2. **Track subscriptions individually** - Multiple handlers per entity must be supported
3. **Use RWMutex for caches** - Better concurrency for read-heavy workloads
4. **Mock external services** - Faster, more reliable concurrency testing
5. **Event handlers must be async** - Prevents deadlocks when handlers send messages
6. **Always test with -race** - Catches bugs you can't see in normal runs

---

## References

- Integration tests: `test/integration/integration_test.go`
- HA client: `internal/ha/client.go`
- State manager: `internal/state/manager.go`
- Mock server: `test/integration/mock_ha_server.go`

**Last Updated**: 2025-11-16
**Test Status**: All 11/11 integration tests passing with `-race` flag

---

## Change Log

### 2025-11-16
- **Added Lesson 5**: Event Handlers Must Not Block Message Processing
  - Fixed critical deadlock bug causing 10-second timeouts in production
  - Changed handlers from synchronous to asynchronous execution
  - Updated `TestClient_HandleEventBackpressuresHandlers` to verify async behavior
  - See `internal/ha/client.go:356-360` for implementation
