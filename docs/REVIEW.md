# Code Review: homeautomation-go initial implementation

## Summary
The new Go client already captures the major architectural pieces from the design doc: it wraps the Home Assistant WebSocket API, keeps a typed cache of input helpers, and exposes a simple demo app that exercises the manager. The code is readable and the test suite covers the happy path for most state operations. While running the test suite succeeds (`go test ./...`), several correctness and design issues should be addressed before building on top of this foundation.

## Key findings

1. **JSON defaults panic for local-only variables**
   `Manager.GetJSON` assumes every JSON default is a string and blindly casts `variable.Default.(string)` when the cache is empty. The only JSON variable (`currentlyPlayingMusic`) is configured with a map default, so the type assertion will panic the first time callers read the value before something is stored in the cache. The fix is to accept `[]byte`, `string`, or any Go value and marshal it accordingly instead of assuming a string literal. 【F:internal/state/manager.go†L402-L429】【F:internal/state/variables.go†L50-L60】

2. **Subscriptions cannot be removed granularly**
   `Manager.Subscribe` returns a `subscription` object, but `subscription.Unsubscribe` unconditionally drops *all* handlers for that key by deleting the entry from `m.subscribers`. Unsubscribing one listener therefore silences every other listener that registered for the same key, which is particularly problematic for plugins that want to share a state topic. The unsubscribe logic should remove only the handler that owns the subscription. 【F:internal/state/manager.go†L524-L545】

3. **Read-only mode is not enforced centrally**
   The metadata (`StateVariable.ReadOnly`) and top-level `READ_ONLY` flag exist, yet the state manager never consults either field. As a result, even when the CLI is started in read-only mode, library consumers (or future plugins) can still mutate Home Assistant state by calling `SetBool`, `SetString`, etc. The manager needs a runtime flag (or per-variable metadata) that prevents writes and returns a deterministic error when read-only is enabled. Right now the flag is only used to skip the demonstration block in `cmd/main.go`, which does not protect the rest of the API. 【F:internal/state/variables.go†L13-L60】【F:internal/state/manager.go†L243-L399】【F:cmd/main.go†L31-L74】

4. **`HAClient` instances cannot reconnect after an intentional disconnect**
   `NewClient` creates a single `context.Context`/`cancel` pair that is stored on the client struct. `Disconnect` calls `c.cancel()` and never recreates the context, yet `Connect` reuses the same `c.ctx`. Any subsequent call to `Connect` (either manual, or via future lifecycle management) spawns a `receiveMessages` goroutine that exits immediately because `c.ctx.Done()` is already closed, and `sendMessage` will also instant-fail through the `case <-c.ctx.Done()` branch. The client should allocate a fresh context inside every successful `Connect`. 【F:internal/ha/client.go†L28-L138】【F:internal/ha/client.go†L141-L235】

5. **`subscribeToEntity` never tears down HA subscriptions**
   The manager tracks HA subscriptions in `m.haSubs`, but `subscription.Unsubscribe` only removes local handlers and never propagates `Unsubscribe` back to the HA client. Over time every entity will remain subscribed at the socket level even if no handlers remain. That can lead to unnecessary event processing (and resource leaks when more entity types are introduced). The state manager should call `ha.Subscription.Unsubscribe()` when the last local handler for a key disappears. 【F:internal/state/manager.go†L120-L205】【F:internal/state/manager.go†L524-L545】

## Tooling / test status
```
go test ./...
```
(Tests pass locally.)
