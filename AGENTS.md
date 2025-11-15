# Agent Guide - Home Automation Project

This document provides guidance for AI agents and developers working on this home automation project.

## Project Overview

This repository contains a home automation system that is migrating from Node-RED to Golang for improved type safety, testability, and maintainability.

To see the current NodeRed behaviors look in:
* flows.json
* ./automated-rendering/screenshot-capture/screenshots after running `make generate-screenshots`

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
2. **[homeautomation-go/README.md](./homeautomation-go/README.md)** - Go implementation user guide
3. **[HA_SYNC_README.md](./HA_SYNC_README.md)** - Home Assistant synchronization details
4. **[homeautomation-go/test/integration/README.md](./homeautomation-go/test/integration/README.md)** - Integration testing guide
5. **[docs/development/CONCURRENCY_LESSONS.md](./docs/development/CONCURRENCY_LESSONS.md)** - Concurrency patterns and lessons learned
6. **[docs/development/BRANCH_PROTECTION.md](./docs/development/BRANCH_PROTECTION.md)** - PR requirements and branch protection setup (NEW)

### External Documentation
- [Go Documentation](https://go.dev/doc/)
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket)
- [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket)
- [zap Logger](https://pkg.go.dev/go.uber.org/zap)

## Development Standards

---
**⚠️ CI/CD Failure Prevention**

Before EVERY push, run this locally:
```bash
cd homeautomation-go && go test ./...
```

This command runs:
- ✅ Unit tests (`internal/ha`, `internal/state`)
- ✅ Integration tests (`test/integration`)
- ✅ Compilation of all test files

**If it passes locally, CI will likely pass. If it fails, CI WILL fail.**

---

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
- All 12 integration tests pass
- All unit tests pass
- No race conditions detected

See [test/integration/README.md](./homeautomation-go/test/integration/README.md) for detailed test documentation.

### Expected Test Results
- **All unit tests must pass** ✅
- **HA client coverage**: ≥70%
- **State manager coverage**: ≥70%
- **No race conditions** when running with `-race`
- **Integration tests**: 12/12 passing ✅

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
# Install tools
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/lint/golint@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run checks
gofmt -w .                      # Format code
goimports -w .                  # Fix imports
go vet ./...                    # Static analysis
golint ./...                    # Linting
staticcheck ./...               # Advanced static analysis
```

### Pre-commit Checks

**MANDATORY** before every commit. These checks mirror what CI/CD will run:

```bash
# 1. Format code
gofmt -w .

# 2. Static analysis
go vet ./...

# 3. Ensure everything compiles (including tests!)
go build ./...

# 4. Run ALL tests (this is what CI runs!)
go test ./...

# 5. Run with race detector
go test -race ./...

# 6. Run integration tests explicitly (for visibility)
go test -v -race ./test/integration/...
```

**If ANY of these fail, CI will fail. Fix locally first!**

#### Quick Pre-Push Validation
```bash
# One-liner to catch most issues:
cd homeautomation-go && go build ./... && go test ./... && echo "✅ Ready to push"
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
- All tests passing (12/12 integration tests)

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

