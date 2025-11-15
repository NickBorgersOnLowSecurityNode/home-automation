# Agent Guide - Home Automation Project

This document provides guidance for AI agents and developers working on this home automation project.

## Project Overview

This repository contains a home automation system that is migrating from Node-RED to Golang for improved type safety, testability, and maintainability.

### Repository Structure

```
/workspaces/node-red/
├── homeautomation-go/          # Golang implementation (NEW)
│   ├── cmd/main.go             # Demo application
│   ├── internal/ha/            # Home Assistant WebSocket client
│   ├── internal/state/         # State management layer
│   ├── go.mod                  # Go module definition
│   └── README.md               # Go project documentation
├── IMPLEMENTATION_PLAN.md      # Architecture and design decisions
├── HA_SYNC_README.md          # HA synchronization documentation
├── AGENTS.md                   # This file
└── [Node-RED files]           # Legacy implementation

```

## Key Documentation

### Required Reading
1. **[IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md)** - Complete architecture, design decisions, and migration strategy
2. **[homeautomation-go/README.md](./homeautomation-go/README.md)** - Go implementation user guide
3. **[HA_SYNC_README.md](./HA_SYNC_README.md)** - Home Assistant synchronization details

### External Documentation
- [Go Documentation](https://go.dev/doc/)
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket)
- [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket)
- [zap Logger](https://pkg.go.dev/go.uber.org/zap)

## Development Standards

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
```

## Running Tests

### Quick Test Commands

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

### Expected Test Results
- **All tests must pass** ✅
- **HA client coverage**: ≥70%
- **State manager coverage**: ≥70%
- **No race conditions** when running with `-race`

### Test Execution Time
- HA client tests: ~10 seconds (includes reconnection testing)
- State manager tests: <1 second
- Total test suite: ~10-11 seconds

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
# Install Go tools
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/lint/golint@latest

# Format code
gofmt -w .

# Vet code
go vet ./...

# Tidy dependencies
go mod tidy
```

### Pre-commit Checklist
- [ ] Code is formatted with `gofmt`
- [ ] No warnings from `go vet`
- [ ] All tests pass: `go test ./...`
- [ ] No race conditions: `go test ./... -race`
- [ ] Test coverage ≥70%
- [ ] Documentation updated if API changed
- [ ] Commit message follows convention (see below)

## Git Workflow

### Commit Message Format
```
<type>: <short description>

<detailed description>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types**: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`

### Example Commits
```
feat: Add support for JSON state variables

Implemented GetJSON/SetJSON methods with local-only variable support
for data that is too large to sync with Home Assistant.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## Common Tasks

### Adding a New State Variable

1. **Update `internal/state/variables.go`:**
```go
{Key: "myNewVar", EntityID: "input_boolean.my_new_var", Type: TypeBool, Default: false}
```

2. **Create corresponding entity in Home Assistant**

3. **Use the variable:**
```go
value, _ := manager.GetBool("myNewVar")
manager.SetBool("myNewVar", true)
```

4. **Add tests** in `manager_test.go`

### Debugging

#### Enable Debug Logging
Change logger creation in `cmd/main.go`:
```go
// From:
logger, err := zap.NewProduction()

// To:
logger, err := zap.NewDevelopment()
```

#### Common Issues

**Connection Refused**
- Check `HA_URL` in `.env`
- Verify Home Assistant is running
- Check WebSocket API is enabled

**Authentication Failed**
- Verify `HA_TOKEN` is valid
- Check token hasn't expired
- Ensure token has proper permissions

**State Not Syncing**
- Verify entity exists in Home Assistant
- Check entity name matches in `variables.go`
- Look for warnings in logs

## Architecture Patterns

### WebSocket Client Pattern
```go
// Connect and handle reconnection
client := ha.NewClient(url, token, logger)
client.Connect()
defer client.Disconnect()
```

### State Management Pattern
```go
// Create manager and sync
manager := state.NewManager(client, logger)
manager.SyncFromHA()

// Subscribe to changes
manager.Subscribe("key", func(key string, old, new interface{}) {
    // Handle change
})
```

### Testing Pattern
```go
func TestFeature(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    mockClient := ha.NewMockClient()
    mockClient.SetState("entity_id", "value", map[string]interface{}{})
    mockClient.Connect()

    // Test code here

    assert.Equal(t, expected, actual)
}
```

## Performance Standards

### Expected Metrics
- **Sync Time**: 100-200ms for all 27 variables
- **State Change Latency**: <100ms from HA event to callback
- **Memory Usage**: 10-20MB typical
- **Goroutines**: <50 under normal operation

### Optimization Guidelines
- Use `sync.RWMutex` for read-heavy caches
- Avoid allocations in hot paths
- Pool WebSocket messages if needed
- Profile before optimizing: `go test -cpuprofile=cpu.prof`

## Security Considerations

### Secrets Management
- **NEVER** commit `.env` files
- **NEVER** commit tokens or credentials
- Use environment variables for all secrets
- `.gitignore` must include `.env`

### Network Security
- Use `wss://` (WebSocket Secure) for production
- Validate all inputs from Home Assistant
- Use timeouts for all network operations
- Implement exponential backoff for retries

## Thread Safety Rules

### Always Thread-Safe
1. **State cache** - protected by `cacheMu`
2. **Subscribers map** - protected by `subsMu`
3. **WebSocket connection** - protected by `connMu`

### Deadlock Prevention
- **Never** hold locks while calling external code
- **Always** release locks before making HA API calls
- Use `defer` for lock cleanup
- Test with `-race` flag regularly

## Troubleshooting Guide

### Test Failures

**Setup failed: directory not found**
```bash
# Use full package path
go test homeautomation/internal/ha
go test homeautomation/internal/state
```

**Timeout in tests**
- Check for deadlocks
- Verify mock client is connected
- Increase timeout if testing reconnection

**Race detector warnings**
- Fix immediately - indicates real bug
- Use mutexes to protect shared state
- Never ignore race warnings

### Build Failures

**Cannot find package**
```bash
go mod tidy
go mod download
```

**Import cycle**
- Reorganize packages
- Extract shared types to separate package
- Review dependency graph

## Agent-Specific Guidelines

### When Making Changes

1. **Read relevant documentation first**
   - IMPLEMENTATION_PLAN.md for architecture
   - README.md for usage patterns
   - Existing code for style

2. **Run tests before and after changes**
   ```bash
   go test ./... -race
   ```

3. **Check test coverage**
   ```bash
   go test ./... -cover
   ```

4. **Update documentation** if APIs change

5. **Commit with descriptive messages**

### Code Review Checklist

- [ ] Follows Go style guidelines
- [ ] Has comprehensive tests
- [ ] Handles errors properly
- [ ] Thread-safe if concurrent
- [ ] Documented (godoc comments)
- [ ] No performance regressions
- [ ] Backward compatible if possible

### Communication

When reporting issues or making decisions:
- **Reference file:line** for code locations
- **Include error messages** verbatim
- **Show test output** when relevant
- **Explain reasoning** for design choices
- **Link to documentation** for context

## Useful Commands Reference

```bash
# Development
go run cmd/main.go              # Run application
go build -o homeautomation ./cmd/main.go  # Build binary

# Testing
go test ./...                   # Run all tests
go test ./... -v                # Verbose output
go test ./... -race             # Race detection
go test ./... -cover            # Coverage summary
go test ./... -coverprofile=coverage.out  # Coverage report

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
See IMPLEMENTATION_PLAN.md for complete migration roadmap.

**Current Phase**: MVP Complete ✅
- Go implementation is ready for parallel testing
- Running in READ_ONLY mode alongside Node-RED
- All 28 state variables supported

**Next Steps**:
1. Validate behavior matches Node-RED
2. Migrate helper functions
3. Switch to read-write mode
4. Deprecate Node-RED implementation

## Getting Help

### Internal Resources
- [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) - Architecture decisions
- [homeautomation-go/README.md](./homeautomation-go/README.md) - User guide
- [HA_SYNC_README.md](./HA_SYNC_README.md) - Sync details

### External Resources
- [Go Documentation](https://go.dev/doc/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Home Assistant Developer Docs](https://developers.home-assistant.io/)

### Common Questions

**Q: Why Go instead of Node-RED?**
A: Type safety, better testing, easier maintenance, no NPM dependency hell. See IMPLEMENTATION_PLAN.md.

**Q: Can I run both implementations simultaneously?**
A: Yes! Use READ_ONLY=true in Go implementation to safely run in parallel.

**Q: How do I add a new state variable?**
A: Update `internal/state/variables.go`, create HA entity, use getter/setter methods.

**Q: Tests are failing with "setup failed"**
A: Use full package path: `go test homeautomation/internal/state`

**Q: How do I test against real Home Assistant?**
A: Update `.env` with real credentials, run `go run cmd/main.go`, watch logs.

---

**Last Updated**: 2025-11-15
**Go Version**: 1.25.3
**Project Status**: MVP Complete, Parallel Testing Phase
