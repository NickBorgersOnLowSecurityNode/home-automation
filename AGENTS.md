# Agent Guide - Home Automation Project

This document provides guidance for AI agents and developers working on this home automation project.

## Project Overview

This repository contains a home automation system that is migrating from Node-RED to Golang for improved type safety, testability, and maintainability.

## 🚨 CRITICAL: Pre-Push Hook Active

**A pre-push git hook automatically runs all tests before every push and BLOCKS if they fail.**

After PRs #23 and #24 were pushed with failing tests, we added automated enforcement. The hook runs:
- Code compilation + all tests + race detector + coverage check (≥70%)

**NEVER use `git push --no-verify` to bypass the hook.** Fix the tests instead.

---

### Understanding Current Node-RED Behavior

**⚠️ IMPORTANT:** Before implementing any feature, you MUST understand the current Node-RED behavior.

See the **[Understanding the Node-RED Implementation](#understanding-the-node-red-implementation)** section below for:
- How to efficiently read `flows.json` (650KB file - don't read it all at once!)
- How to generate and use flow screenshots
- Recommended workflow for researching flows
- Search patterns and examples

**Quick Start:**
```bash
# Generate flow screenshots
make generate-screenshots

# View screenshots in:
./automated-rendering/screenshot-capture/screenshots/

# Access live Node-RED instance:
# https://node-red.featherback-mermaid.ts.net/
```

### Repository Structure

```
/workspaces/node-red/
├── .github/
│   ├── workflows/
│   │   ├── pr-tests.yml        # PR test requirements
│   │   ├── docker-build-push.yml # Docker build + push (main/master only)
│   │   └── [other workflows]
├── docs/
│   ├── architecture/           # Architecture documentation
│   │   ├── IMPLEMENTATION_PLAN.md
│   │   └── GOLANG_DESIGN.md
│   ├── development/            # Development guides
│   │   ├── BRANCH_PROTECTION.md
│   │   └── CONCURRENCY_LESSONS.md
│   ├── migration/              # Migration documentation
│   │   └── migration_mapping.md
│   ├── deployment/             # Deployment guides
│   │   └── DOCKER.md
│   └── REVIEW.md               # Code review notes
├── homeautomation-go/          # Golang implementation
│   ├── cmd/main.go             # Demo application
│   ├── internal/ha/            # Home Assistant WebSocket client
│   ├── internal/state/         # State management layer
│   ├── test/integration/       # Integration test suite
│   ├── go.mod                  # Go module definition
│   └── README.md               # Go project documentation
├── CLAUDE.md                   # Claude Code project instructions
├── AGENTS.md                   # This file - development guide
└── [Node-RED files]           # Legacy implementation

```

## Key Documentation

### Required Reading
1. **[docs/architecture/IMPLEMENTATION_PLAN.md](./docs/architecture/IMPLEMENTATION_PLAN.md)** - Complete architecture, design decisions, and migration strategy
2. **[docs/architecture/VISUAL_ARCHITECTURE.md](./docs/architecture/VISUAL_ARCHITECTURE.md)** - Mermaid diagrams visualizing system architecture and plugin logic
3. **[docs/architecture/SHADOW_STATE.md](./docs/architecture/SHADOW_STATE.md)** - Shadow state pattern for plugin observability (**READ THIS BEFORE WRITING PLUGINS**)
4. **[docs/DIAGRAM_QUICK_START.md](./docs/DIAGRAM_QUICK_START.md)** - Quick guide to navigating visual documentation
5. **[homeautomation-go/README.md](./homeautomation-go/README.md)** - Go implementation user guide
6. **[HA_SYNC_README.md](./HA_SYNC_README.md)** - Home Assistant synchronization details
7. **[homeautomation-go/test/integration/README.md](./homeautomation-go/test/integration/README.md)** - Integration testing guide
8. **[docs/development/CONCURRENCY_LESSONS.md](./docs/development/CONCURRENCY_LESSONS.md)** - Concurrency patterns and lessons learned
9. **[docs/development/BRANCH_PROTECTION.md](./docs/development/BRANCH_PROTECTION.md)** - PR requirements and branch protection setup

### External Documentation
- [Go Documentation](https://go.dev/doc/)
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket)
- [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket)
- [zap Logger](https://pkg.go.dev/go.uber.org/zap)

## Understanding the Node-RED Implementation

Before implementing features in Go, you need to understand the current Node-RED behavior. This section provides guidance on efficiently reading and analyzing the legacy implementation.

### flows.json Structure

**File:** `flows.json` (~650KB)
**⚠️ WARNING:** This file is very large. Do NOT attempt to read it all at once. Use targeted searches instead.

**Structure Overview:**
```json
[
  {
    "id": "d7a3510d.e93d98",
    "type": "tab",
    "label": "State Tracking",
    "disabled": false
  },
  {
    "type": "function",
    "name": "Pick Appropriate Music",
    "func": "// JavaScript code here",
    "wires": [...]
  }
  // ... thousands more nodes
]
```

**Key Node Types:**
- `type: "tab"` - Represents a flow/tab (e.g., "Music", "Lighting Control")
- `type: "function"` - JavaScript function nodes containing business logic
- `type: "api-call-service"` - Home Assistant service calls
- `type: "server-state-changed"` - HA entity state change triggers
- `type: "switch"` - Routing/conditional logic

### Efficient Search Strategies

**DO NOT** read flows.json directly. Instead, use these targeted search patterns:

#### Find a Specific Flow

```bash
# Find the Music flow definition
grep -A 5 '"label":"Music"' flows.json

# Find all flows (tabs)
grep '"type":"tab"' flows.json
```

#### Find Business Logic Functions

```bash
# Find a specific function node by name
grep -A 20 '"name":"Pick Appropriate Music"' flows.json

# Find all function nodes in the Music flow
# (First get the flow ID, then search for nodes with that flow ID)
grep -A 50 '"label":"Music"' flows.json | grep '"type":"function"'
```

#### Find State Variable Usage

```bash
# Find where isNickHome is used
grep -n "isNickHome" flows.json

# Find all presence variables
grep -n "isNickHome\|isCarolineHome\|isToriHere" flows.json

# Find Home Assistant service calls
grep -n '"type":"api-call-service"' flows.json
```

#### Understand Entity Subscriptions

```bash
# Find what entities a flow listens to
grep -A 5 '"type":"server-state-changed"' flows.json | grep "entityid"

# Find all subscriptions to a specific entity
grep "input_boolean.nick_home" flows.json
```

### Visual Flow Analysis

Screenshots provide the best high-level understanding of each flow.

#### Generate Screenshots

```bash
# From repository root
make generate-screenshots

# Screenshots will be in:
# ./automated-rendering/screenshot-capture/screenshots/
```

**Available Flows (when screenshots are generated):**
- `State Tracking.png` - Presence and sleep state tracking
- `Music.png` - Music mode selection and Sonos control
- `Lighting Control.png` - Scene activation and sun events
- `Sleep Hygiene.png` - Wake-up sequences
- `Energy State.png` - Battery and solar tracking
- `Load Shedding.png` - Thermostat control
- `Security.png` - Lockdown and garage automation
- `TV Monitoring and Manipulation.png` - TV state detection
- `Configuration.png` - Config file loading
- `Calendar.png` - Meeting reminders
- `Nagging.png` - Weather-based reminders

#### View Live Node-RED Instance

You can access the running Node-RED instance with your MCP server:
**URL:** https://node-red.featherback-mermaid.ts.net/

This allows interactive exploration of flows, clicking through nodes, and seeing live configuration.

### Recommended Workflow for Understanding Behavior

When implementing a feature, follow this workflow:

1. **Start with the screenshot** for visual overview
   ```bash
   make generate-screenshots
   # View ./automated-rendering/screenshot-capture/screenshots/Music.png
   ```

2. **Find the flow in flows.json** for detailed configuration
   ```bash
   grep -A 100 '"label":"Music"' flows.json > music_flow.json
   ```

3. **Identify function nodes** containing business logic
   ```bash
   grep -A 50 '"type":"function"' music_flow.json | less
   ```

4. **Check state variables used** in the flow
   ```bash
   # Cross-reference with docs/migration/migration_mapping.md
   grep "input_boolean\|input_number\|input_text" music_flow.json
   ```

5. **Review relevant config files** for data structures
   ```bash
   # For Music flow, check:
   cat configs/music_config.yaml
   ```

6. **Test against live Node-RED** for behavior verification
   - Visit https://node-red.featherback-mermaid.ts.net/
   - Navigate to the flow tab
   - Click "Deploy" and observe behavior

### Quick Reference: Flow to Config Mapping

| Flow | Screenshot | Config File | Key State Variables |
|------|-----------|-------------|---------------------|
| **State Tracking** | State Tracking.png | N/A | isNickHome, isCarolineHome, isToriHere, isMasterAsleep, isGuestAsleep |
| **Lighting Control** | Lighting Control.png | hue_config.yaml | dayPhase, sunevent, isAnyoneHome |
| **Music** | Music.png | music_config.yaml | musicPlaybackType, currentlyPlayingMusic, sleep states |
| **Sleep Hygiene** | Sleep Hygiene.png | schedule_config.yaml | isMasterAsleep, alarmTime, musicPlaybackType |
| **Energy State** | Energy State.png | energy_config.yaml | batteryEnergyLevel, solarProductionEnergyLevel, currentEnergyLevel |
| **Load Shedding** | Load Shedding.png | N/A | currentEnergyLevel |
| **Security** | Security.png | N/A | isEveryoneAsleep, isAnyoneHome, isExpectingSomeone |
| **TV Monitoring** | TV Monitoring and Manipulation.png | N/A | isTVPlaying, isAppleTVPlaying, isTVOn, dayPhase |
| **Calendar** | Calendar.png | N/A | isNickHome, isCarolineHome |
| **Nagging** | Nagging.png | N/A | isAnyoneHome, musicPlaybackType |

### Example: Researching the Music Flow

Here's a complete example of researching how music mode selection works:

```bash
# 1. Generate and view the screenshot
make generate-screenshots
# View ./automated-rendering/screenshot-capture/screenshots/Music.png

# 2. Extract the Music flow to a temporary file
grep -A 1000 '"label":"Music"' flows.json | head -500 > /tmp/music.json

# 3. Find the main decision logic function
grep -B 2 -A 30 '"name":"Pick Appropriate Music"' /tmp/music.json

# 4. Find what triggers music mode changes
grep -A 10 '"type":"server-state-changed"' /tmp/music.json | grep "dayPhase\|isNickHome\|isMasterAsleep"

# 5. Check the config file structure
cat configs/music_config.yaml

# 6. Verify state variable definitions
grep "musicPlaybackType" docs/migration/migration_mapping.md

# 7. Test on live instance
# Visit https://node-red.featherback-mermaid.ts.net/#flow/90f5fe8cb80ae6a7
# (90f5fe8cb80ae6a7 is the Music flow ID from flows.json)
```

### Common Pitfalls

❌ **Don't:** Try to read all of flows.json at once
✅ **Do:** Use grep to extract specific flows or node types

❌ **Don't:** Implement based on assumptions
✅ **Do:** Cross-reference screenshots, flows.json, and the live instance

❌ **Don't:** Ignore disabled flows
✅ **Do:** Check the `"disabled": true/false` field when researching

❌ **Don't:** Miss complex logic in function nodes
✅ **Do:** Extract and read the JavaScript in `"func"` fields carefully

### Additional Resources

- **[README.md](./README.md)** - High-level overview of all flows with visual diagrams
- **[docs/architecture/GOLANG_DESIGN.md](./docs/architecture/GOLANG_DESIGN.md)** - Detailed flow descriptions and migration strategy
- **[docs/migration/migration_mapping.md](./docs/migration/migration_mapping.md)** - Complete state variable mapping
- **[configs/](./configs/)** - YAML configuration files defining behavior

## Development Standards

### Shadow State Pattern (CRITICAL)

**Every plugin MUST properly implement shadow state tracking.** See [docs/architecture/SHADOW_STATE.md](./docs/architecture/SHADOW_STATE.md) for full details.

#### Quick Reference: Required Pattern

```go
// EVERY handler must update shadow inputs at the start
func (m *Manager) handleSomeChange(entityID string, oldState, newState *ha.State) {
    if newState == nil {
        return
    }

    // 1. FIRST: Update shadow state inputs (captures what triggered this)
    m.updateShadowInputs()

    // 2. Then: Process the change
    // 3. Update state variables
    // 4. Update shadow state outputs
}

// This method captures ALL subscribed inputs
func (m *Manager) updateShadowInputs() {
    inputs := make(map[string]interface{})

    // Capture state variables this plugin subscribes to
    if val, err := m.stateManager.GetBool("someInput"); err == nil {
        inputs["someInput"] = val
    }

    // Capture raw HA entity states
    if state, err := m.haClient.GetState("sensor.something"); err == nil && state != nil {
        inputs["sensor.something"] = state.State
    }

    m.shadowTracker.UpdateCurrentInputs(inputs)
}
```

#### Common Bug: Forgetting to Track Inputs

If `/api/shadow/{plugin}` shows `inputs.current: {}` (empty), the plugin is missing `updateShadowInputs()` calls.

**Checklist for new plugins:**
- [ ] Add `shadowTracker` field to Manager struct
- [ ] Implement `updateShadowInputs()` method
- [ ] Call `updateShadowInputs()` at the START of every handler
- [ ] Call output update methods (e.g., `UpdateSomeLevel()`) when computing outputs
- [ ] Test that shadow state is populated after handlers run

### Update docs/architecture/IMPLEMENTATION_PLAN.md
As you complete tasks, update the implementation plan with progress, and add additional work items as additional problems to solve are identified.

### Go Code Standards

#### Style Guidelines
- Follow standard Go formatting (`gofmt`)
- Use `golint` and `go vet` standards
- Maximum line length: 120 characters
- Use descriptive variable names (no single-letter variables except for loops)
- Add godoc comments to all exported functions, types, and packages

#### Testing Requirements
- **Minimum test coverage**: 70% for all packages
- All public functions must have tests
- Use table-driven tests where appropriate
- Tests must pass with race detector: `go test -race`
- Mock external dependencies (Home Assistant client)
- **Run integration tests** before major changes

#### Error Handling
- Always check and handle errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Log errors with appropriate levels (Error, Warn, Info)
- Never panic in production code (except for initialization failures)

#### Concurrency
- All shared state must be protected with mutexes
- Use `sync.RWMutex` for read-heavy operations
- Document goroutine lifecycles
- Always test concurrent code with `-race` flag
- **WebSocket writes must be serialized** (use writeMu)

### File Organization
```
internal/
├── ha/                    # Home Assistant client package
│   ├── client.go         # WebSocket client implementation
│   ├── types.go          # Message types and structs
│   ├── mock.go           # Mock client for testing
│   └── client_test.go    # Client tests
└── state/                # State management package
    ├── manager.go        # State manager implementation
    ├── variables.go      # Variable definitions
    └── manager_test.go   # Manager tests

test/
└── integration/          # Integration tests (NEW)
    ├── mock_ha_server.go      # Mock Home Assistant server
    ├── integration_test.go    # Test scenarios
    ├── Dockerfile             # Container for tests
    └── README.md              # Testing guide
```

## Running Tests

### Unit Tests

```bash
# Navigate to Go project directory
cd homeautomation-go

# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run tests with race detection
go test ./... -race

# Run specific package tests
go test ./internal/ha -v
go test ./internal/state -v

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests (NEW)

Integration tests use a full mock Home Assistant WebSocket server to validate the system under realistic concurrent load.

#### Quick Run
```bash
cd homeautomation-go

# Run all integration tests with race detector (recommended)
go test -v -race ./test/integration/...

# Run specific test scenarios
go test -v -race -run TestConcurrent ./test/integration/
go test -v -race -run TestSubscription ./test/integration/
```

#### Docker Run (Isolated)
```bash
# From repository root
docker-compose -f homeautomation-go/docker-compose.integration.yml up --build
```

#### What Integration Tests Validate

✅ **Concurrency & Race Conditions**
- 50 goroutines × 100 concurrent reads
- 20 goroutines × 50 concurrent writes
- Mixed read/write workloads
- CompareAndSwap atomic operations

✅ **Deadlock Detection**
- Subscription callbacks triggering more state operations
- Rapid state changes from both server and client
- Multiple subscribers on same entity

✅ **Edge Cases**
- High-frequency state changes (1000+ events)
- Reconnection after disconnect
- Network latency simulation

✅ **All State Types**
- Boolean, Number, String, JSON operations

#### Test Status

✅ **All tests passing** - No known failures
- All 11 integration tests pass
- All unit tests pass
- No race conditions detected

See [test/integration/README.md](./homeautomation-go/test/integration/README.md) for detailed test documentation.

### Expected Test Results
- **All unit tests must pass** ✅
- **HA client coverage**: ≥70%
- **State manager coverage**: ≥70%
- **No race conditions** when running with `-race`
- **Integration tests**: 11/11 passing ✅

### Test Execution Time
- HA client tests: ~10 seconds (includes reconnection testing)
- State manager tests: <1 second
- Integration tests: ~20-30 seconds
- Total test suite: ~30-40 seconds

## Building and Running

### Build the Application
```bash
cd homeautomation-go
go build -o homeautomation ./cmd/main.go
```

### Run the Application
```bash
# Using go run
go run cmd/main.go

# Using compiled binary
./homeautomation
```

### Environment Configuration
Create a `.env` file in `homeautomation-go/`:
```env
HA_URL=wss://your-homeassistant/api/websocket
HA_TOKEN=your_long_lived_access_token
READ_ONLY=true
```

See `.env.example` for template.

## Code Quality Tools

### Required Tools
```bash
# Install tools (or run 'make pre-commit' which auto-installs them)
go install golang.org/x/tools/cmd/goimports@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Format code automatically
make format-go
# OR manually:
gofmt -w .                      # Format code
goimports -w .                  # Fix imports

# Run linters
make lint-go
# OR manually:
go vet ./...                    # Static analysis
staticcheck ./...               # Advanced linting
```

**Note**: `golint` is deprecated. Use `staticcheck` for comprehensive linting.

### Git Hooks Strategy

This repository uses a two-tier validation strategy optimized for developer experience:

#### Pre-commit Hook (Fast - ~5-10 seconds)

**Runs automatically on every commit.** Checks code quality without running tests:

```bash
# Automatically runs via git hook, or manually:
make pre-commit

# This runs fast checks only:
# 1. gofmt formatting check
# 2. goimports formatting check
# 3. go vet static analysis
# 4. staticcheck linting
# 5. Build check (go build ./...)
```

**Why no tests?** Commits should be fast. Tests run on push (see below).

#### Pre-push Hook (Comprehensive - ~30-40 seconds)

**Runs automatically before every push.** This is the main quality gate:

```bash
# Automatically runs via git hook, or manually:
make pre-push

# This runs comprehensive validation (matches CI exactly):
# 1. Build check (go build ./...)
# 2. All tests with race detector (go test -race ./...)
# 3. Coverage check (≥70%)
```

**Your push will be blocked if tests fail.**

**The `make pre-push` target uses the exact same coverage check as CI**, ensuring local validation matches what will run in GitHub Actions. This prevents surprises when your PR is tested.

#### Manual Testing

If you want to run individual test commands:

```bash
cd homeautomation-go
go test ./...                    # Run all tests
go test -race ./...              # Run with race detector
go test -v -race ./test/integration/...  # Integration tests explicitly
```

**If pre-push fails, CI will also fail. Fix locally first!**

#### Quick Commands

```bash
# Format code automatically:
make format-go

# Run linters only:
make lint-go

# Run all pre-commit checks:
make pre-commit

# Run all pre-push checks (same as CI):
make pre-push

# One-liner to catch most issues:
make pre-push
```

#### Required Tools

Install these tools if not already present (the Makefile will auto-install them):

```bash
go install golang.org/x/tools/cmd/goimports@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## Critical Bugs Found by Integration Tests

The integration test suite discovered and helped fix production-critical bugs. See [docs/development/CONCURRENCY_LESSONS.md](./docs/development/CONCURRENCY_LESSONS.md) for concurrency patterns and lessons learned.

### Fixed Bugs ✅
1. **Concurrent WebSocket Writes** - Would cause panics under load
   - **Severity**: CRITICAL
   - **Fix**: Added `writeMu` mutex in `internal/ha/client.go`
   - **Tests**: TestConcurrentWrites, TestConcurrentReadsAndWrites

2. **Subscription Memory Leak** - Unsubscribe removed all handlers
   - **Severity**: HIGH
   - **Fix**: Fixed subscription handler tracking in `internal/ha/client.go`
   - **Test**: TestMultipleSubscribersOnSameEntity

### Active Bugs ❌
None - all known bugs have been fixed! ✅

**Always run integration tests after making changes to concurrency-sensitive code.**

## API Change Protocol

When modifying function signatures, types, or interfaces:

### CRITICAL: Search for All Call Sites
```bash
# Before changing a function signature, find ALL usages
cd homeautomation-go
grep -r "FunctionName" .
# OR use Grep tool to find all call sites

# Example: When changing NewManager signature
grep -r "NewManager" .
```

### Required Steps for API Changes
1. ✅ **Search** for all call sites using grep/ripgrep
2. ✅ **Update** ALL call sites (code + tests + docs)
3. ✅ **Compile check**: `go build ./...`
4. ✅ **Run ALL tests**: `go test ./...` (includes integration)
5. ✅ **Verify CI will pass** locally before pushing

### Common Files to Check
- [ ] All `*_test.go` files (especially integration tests)
- [ ] `cmd/main.go` (application entry point)
- [ ] `README.md` (code examples)
- [ ] Other documentation with code samples

**⚠️ Breaking Change Alert**: Compilation errors in tests are easy to miss if you don't run `go test ./...`

## Development Workflow

### Making Changes

1. **Create feature branch** from main
2. **Make code changes** following standards above
3. **If changing function signatures**:
   - Search for ALL usages: `grep -r "FunctionName" .`
   - Update code, tests, AND documentation
   - Verify compilation: `go build ./...`
4. **Write/update tests** (both unit and integration if needed)
5. **Run ALL tests** (exactly what CI runs):
   ```bash
   # This single command runs EVERYTHING (unit + integration)
   go test ./...

   # Add race detection (CI requirement)
   go test -race ./...

   # ⚠️ Common mistake: forgetting integration tests exist in ./test/integration
   # The command above INCLUDES them, so watch for failures there!
   ```
6. **Format and lint** code
7. **Commit with descriptive message**
8. **Push and create PR**

### Pull Request Requirements

**⚠️ IMPORTANT: All PRs require passing tests before merge**

This repository enforces test requirements through GitHub branch protection rules and automated CI checks.

#### Automated PR Testing

Every pull request automatically runs:
- ✅ **Go unit tests** with race detector
- ✅ **Test coverage check** (minimum 70%)
- ✅ **Integration tests** (concurrent load, deadlocks, race conditions)
- ✅ **Config validation** (YAML files, Spotify URIs)

**The PR merge button will be blocked until all required tests pass.**

See [.github/BRANCH_PROTECTION.md](./.github/BRANCH_PROTECTION.md) for details on:
- How to configure branch protection rules
- What tests are required
- Troubleshooting test failures

#### CI Workflow

When you create or update a PR:
1. **GitHub Actions automatically triggers** the `PR Tests` workflow
2. **Tests run in parallel** (Go tests + Config validation)
3. **Status checks appear** on your PR:
   - 🟡 Yellow circle: Tests running
   - 🟢 Green checkmark: All tests passed - **ready to merge**
   - 🔴 Red X: Tests failed - **merge blocked**
4. **Review workflow logs** in the Actions tab if tests fail

### Pull Request Checklist

Before creating a PR, verify locally:
- [ ] All tests passing (unit + integration)
- [ ] No race conditions (`-race` flag)
- [ ] Code coverage ≥70%
- [ ] Follows Go style guidelines
- [ ] Has comprehensive tests
- [ ] Handles errors properly
- [ ] Thread-safe if concurrent
- [ ] Documented (godoc comments)
- [ ] No performance regressions
- [ ] Backward compatible if possible

**Note**: The first 4 items are automatically verified by CI, but running locally first saves time.

### Communication

When reporting issues or making decisions:
- **Reference file:line** for code locations
- **Include error messages** verbatim
- **Show test output** when relevant
- **Explain reasoning** for design choices
- **Link to documentation** for context

## Common CI Failures & Prevention

### "not enough arguments in call to X"
**Cause**: Function signature changed but not all call sites updated

**Prevention**:
- Run `grep -r "FunctionName" .` before changing signatures
- Always run `go test ./...` (not just `go build`)
- Check README.md and test files

**Fix**: Search for all usages, update them, verify with `go test ./...`

### "undefined: X" or import errors
**Cause**: Missing dependency or module not updated

**Prevention**: Run `go mod tidy` before committing

**Fix**: `go mod tidy && go mod download`

### Test timeout or deadlock
**Cause**: Concurrent code issue, missing mutex

**Prevention**: Always run tests with `-race` flag

**Fix**: Review integration test output, check for missing locks

### "No tests run" but expecting tests
**Cause**: Test files not updated, syntax errors in test files

**Prevention**: Run `go test ./... -v` to see which tests run

**Fix**: Check test file syntax, ensure `_test.go` suffix

### Tests pass locally but fail in CI
**Cause**:
- Environment differences
- Race conditions only visible under CI load
- Missing dependencies in CI environment

**Prevention**:
- Always run `go test -race ./...` locally
- Check CI logs for environment-specific errors
- Ensure `go.mod` and `go.sum` are committed

**Fix**: Review CI logs, reproduce locally with same Go version

## Useful Commands Reference

```bash
# Development
go run cmd/main.go              # Run application
go build -o homeautomation ./cmd/main.go  # Build binary

# Testing
go test ./...                   # Run all unit tests
go test ./... -v                # Verbose output
go test ./... -race             # Race detection
go test ./... -cover            # Coverage summary
go test ./... -coverprofile=coverage.out  # Coverage report

# Integration Testing (NEW)
go test -v -race ./test/integration/...   # All integration tests
go test -v -race -run TestConcurrent ./test/integration/  # Specific test
docker-compose -f docker-compose.integration.yml up --build  # Docker run

# Code Quality
gofmt -w .                      # Format code
go vet ./...                    # Static analysis
go mod tidy                     # Clean dependencies
go mod download                 # Download dependencies

# Debugging
go test -v ./internal/ha -run TestSpecific  # Run specific test
go build -race                  # Build with race detector
dlv debug ./cmd/main.go         # Debug with delve (if installed)
```

## Related Projects and Migration

### Node-RED Implementation (Legacy)
- Located in repository root
- Being phased out in favor of Go implementation
- Consult for business logic reference
- **Do not add new features to Node-RED**

### Migration Strategy
See docs/architecture/IMPLEMENTATION_PLAN.md for complete migration roadmap.

**Current Phase**: MVP Complete + Integration Testing ✅
- Go implementation is ready for parallel testing
- Running in READ_ONLY mode alongside Node-RED
- All 28 state variables supported
- Comprehensive integration test suite validates correctness
- All critical bugs fixed (concurrent writes, subscription leak)
- All tests passing (11/11 integration tests)

**Next Steps**:
1. Validate behavior matches Node-RED
2. Migrate helper functions
3. Switch to read-write mode
4. Deprecate Node-RED implementation

## Getting Help

### Internal Resources
- [docs/architecture/IMPLEMENTATION_PLAN.md](./docs/architecture/IMPLEMENTATION_PLAN.md) - Architecture decisions
- [homeautomation-go/README.md](./homeautomation-go/README.md) - User guide
- [HA_SYNC_README.md](./HA_SYNC_README.md) - Sync details
- [test/integration/README.md](./homeautomation-go/test/integration/README.md) - Integration testing
- [docs/development/CONCURRENCY_LESSONS.md](./docs/development/CONCURRENCY_LESSONS.md) - Concurrency patterns and lessons

### External Resources
- [Go Documentation](https://go.dev/doc/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Home Assistant Developer Docs](https://developers.home-assistant.io/)

### Common Questions

**Q: Why Go instead of Node-RED?**
A: Type safety, better testing, easier maintenance, no NPM dependency hell. See docs/architecture/IMPLEMENTATION_PLAN.md.

**Q: Can I run both implementations simultaneously?**
A: Yes! Use READ_ONLY=true in Go implementation to safely run in parallel.

**Q: How do I add a new state variable?**
A: Update `internal/state/variables.go`, create HA entity, use getter/setter methods.

**Q: Tests are failing with "setup failed"**
A: Use full package path: `go test homeautomation/internal/state`

**Q: How do I test against real Home Assistant?**
A: Update `.env` with real credentials, run `go run cmd/main.go`, watch logs.

**Q: Should I use a real HA instance for testing or the mock?**
A: Use the mock for automated testing (faster, more reliable). Use real HA for final validation.

**Q: How do I run tests in Docker?**
A: `docker-compose -f homeautomation-go/docker-compose.integration.yml up --build`

**Q: What if I'm adding concurrent code?**
A: MUST test with `-race` flag and run integration tests. Protect WebSocket writes with mutex.

---

**Last Updated**: 2025-11-15
**Go Version**: 1.23
**Project Status**: MVP Complete, Integration Testing Added, Parallel Testing Phase

