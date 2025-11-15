# Scripts Directory

This directory contains utility scripts for the home-automation project.

## Available Scripts

### `validate-before-push.sh`

**Purpose:** Manually run all pre-push validation checks before committing/pushing code.

**Usage:**
```bash
./scripts/validate-before-push.sh
```

**What it checks:**
- ✅ Code compilation (including all test files)
- ✅ All unit tests (`internal/ha`, `internal/state`)
- ✅ All integration tests (`test/integration`)
- ✅ Race conditions (`-race` flag)
- ✅ Test coverage (minimum 70%)

**When to use:**
- Before committing changes (recommended)
- When debugging test failures
- To verify your changes are ready to push

**Exit codes:**
- `0` - All checks passed
- `1` - One or more checks failed

---

### `setup-git-hooks.sh`

**Purpose:** Install and activate git hooks after cloning the repository.

**Usage:**
```bash
./scripts/setup-git-hooks.sh
```

**What it does:**
- Installs the pre-push hook to `.git/hooks/pre-push`
- Makes the hook executable
- Validates the hook is properly installed

**When to use:**
- After cloning the repository for the first time
- If the pre-push hook gets accidentally deleted
- When updating to a new version of the hook

**Note:** The pre-push hook is already active in this repository. You only need to run this script if you're setting up a fresh clone.

---

## Git Hooks

### Pre-Push Hook (Active)

**Location:** `.git/hooks/pre-push`

**Purpose:** Automatically validate code before every `git push` to prevent pushing broken code.

**What it does:**
The hook runs the same validation as `validate-before-push.sh`:
1. Verifies all code compiles
2. Runs all tests (unit + integration)
3. Checks for race conditions
4. Validates test coverage ≥70%

**Behavior:**
- ✅ If all checks pass → Push proceeds normally
- ❌ If any check fails → Push is BLOCKED

**Bypass (Emergency Only):**
```bash
git push --no-verify
```

**Only bypass if:**
- Pushing documentation-only changes
- Explicitly coordinated with the team
- You understand the risks

See `AGENTS.md` for the full development guide and test requirements.

---

## Adding New Scripts

When adding new scripts to this directory:

1. **Make them executable:**
   ```bash
   chmod +x scripts/your-script.sh
   ```

2. **Add a header comment:**
   ```bash
   #!/bin/bash
   # Description of what the script does
   # Usage: ./scripts/your-script.sh [args]
   ```

3. **Update this README:**
   - Add a section describing the script
   - Include usage examples
   - Document exit codes and error handling

4. **Test the script:**
   - Run it locally
   - Test error cases
   - Verify exit codes

---

## Troubleshooting

### "Permission denied" when running scripts

**Solution:**
```bash
chmod +x scripts/*.sh
```

### Pre-push hook not running

**Check hook is installed:**
```bash
ls -la .git/hooks/pre-push
```

**Should show:** `-rwxr-xr-x` (executable permissions)

**If missing or not executable:**
```bash
./scripts/setup-git-hooks.sh
```

### Tests fail in hook but pass manually

**Possible causes:**
- Different working directory
- Environment variables not set
- Race conditions (timing-dependent)

**Debug:**
1. Check your current directory when hook runs
2. Add `set -x` to the hook for verbose output
3. Run: `cd homeautomation-go && go test ./... -race -v`

---

**Last Updated:** 2025-11-15
