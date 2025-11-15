# Test Failure Prevention Mechanisms

## Problem

Multiple PRs (#23, #24) were pushed with failing tests, causing CI failures and wasting time/resources. Despite guidance in `AGENTS.md` and recommendations for pre-commit checks, agents were still pushing untested code.

## Root Causes

1. **No Enforcement** - Documentation alone wasn't sufficient; agents could skip test validation
2. **No Active Git Hooks** - Only `.sample` hook files existed; no automated checks
3. **Manual Process** - Relied on agents remembering to run tests before pushing
4. **Limited Visibility** - No easy way for agents to validate changes locally

## Solutions Implemented

### 1. Active Pre-Push Git Hook

**File:** `.git/hooks/pre-push`

**What it does:**
- Automatically runs before every `git push`
- Validates all code compiles (including tests)
- Runs all unit and integration tests
- Checks for race conditions with `-race` flag
- Validates test coverage ≥70%
- **BLOCKS the push if any check fails**

**Features:**
- Clear, colored output showing progress
- Detailed error messages on failure
- Instructions for fixing failures
- Option to bypass with `--no-verify` (emergency only)

**Status:** ✅ Active and executable

---

### 2. Validation Script for Manual Checks

**File:** `scripts/validate-before-push.sh`

**What it does:**
- Runs the same checks as the pre-push hook
- Can be invoked manually before committing
- Provides verbose output for debugging
- Exits with proper status codes

**Usage:**
```bash
./scripts/validate-before-push.sh
```

**When to use:**
- Before committing changes (recommended)
- When debugging test failures
- To verify readiness before pushing

**Status:** ✅ Created and executable

---

### 3. Hook Setup Script

**File:** `scripts/setup-git-hooks.sh`

**What it does:**
- Installs pre-push hook for fresh repository clones
- Verifies hook is properly configured
- Provides setup confirmation

**Usage:**
```bash
./scripts/setup-git-hooks.sh
```

**When to use:**
- After cloning the repository
- If the hook gets accidentally deleted
- When updating to new hook versions

**Status:** ✅ Created and executable

---

### 4. Updated AGENTS.md Documentation

**Changes made:**

1. **Added prominent warning section at top:**
   - `## 🚨 CRITICAL: Test Validation Before Every Push 🚨`
   - Impossible to miss when reading the guide
   - References both automated and manual validation

2. **Explained why this matters:**
   - Referenced failed PRs #23 and #24
   - Emphasized time/resource waste from CI failures

3. **Clear DO/DON'T guidance:**
   - ❌ Don't bypass hooks with `--no-verify`
   - ❌ Don't ignore test failures
   - ✅ Do fix tests before pushing
   - ✅ Do run validation script

4. **Updated "Development Standards" section:**
   - Added references to pre-push hook
   - Linked to validation script
   - Made testing requirements more prominent

---

### 5. Scripts Documentation

**File:** `scripts/README.md`

**What it contains:**
- Complete documentation of all scripts
- Usage examples for each script
- Troubleshooting guide
- Best practices

---

## How It Prevents Future Failures

### Before (Prior to PRs #23, #24)

```
Agent makes changes
  ↓
Agent commits
  ↓
Agent pushes (no validation!)
  ↓
CI runs tests
  ↓
❌ CI FAILS
  ↓
Time wasted, PR blocked
```

### After (Current Implementation)

```
Agent makes changes
  ↓
Agent commits
  ↓
Agent tries to push
  ↓
🚨 PRE-PUSH HOOK RUNS AUTOMATICALLY
  ├─ Compiles code ✅
  ├─ Runs all tests ✅
  ├─ Checks for races ✅
  └─ Validates coverage ✅
  ↓
All checks pass?
  ├─ YES → Push proceeds → ✅ CI will likely pass
  └─ NO  → ❌ PUSH BLOCKED → Fix locally first
```

---

## Verification Checklist

Before considering this complete, verify:

- [x] Pre-push hook exists at `.git/hooks/pre-push`
- [x] Pre-push hook is executable (`chmod +x`)
- [x] Validation script exists at `scripts/validate-before-push.sh`
- [x] Validation script is executable
- [x] Setup script exists at `scripts/setup-git-hooks.sh`
- [x] Setup script is executable
- [x] AGENTS.md has prominent warning section
- [x] AGENTS.md references the hook and scripts
- [x] Scripts README.md documents all tools
- [ ] Hook has been tested with actual push (will be tested when changes are pushed)

---

## Testing the Prevention Mechanisms

### Test 1: Pre-Push Hook Activation

```bash
# Verify hook is active
ls -la .git/hooks/pre-push
# Should show: -rwxr-xr-x (executable permissions)
```

### Test 2: Manual Validation

```bash
# Run validation script
./scripts/validate-before-push.sh

# Should output:
# - Step 1: Compiling... ✅
# - Step 2: Running tests... ✅
# - Step 3: Race detector... ✅
# - Step 4: Coverage check... ✅
# - ALL VALIDATION CHECKS PASSED
```

### Test 3: Hook Triggers on Push

```bash
# Make a commit and try to push
git add .
git commit -m "test"
git push

# Should automatically run pre-push hook before pushing
```

### Test 4: Hook Blocks Failing Tests

To test that the hook actually blocks bad code:

1. Intentionally break a test
2. Try to push
3. Hook should detect failure and block the push
4. Fix the test
5. Push should succeed

---

## For AI Agents

**MANDATORY READING:**

1. **Before making ANY code changes**, read the section at the top of `AGENTS.md`:
   - `## 🚨 CRITICAL: Test Validation Before Every Push 🚨`

2. **Before committing**, run:
   ```bash
   ./scripts/validate-before-push.sh
   ```

3. **Before pushing**, the pre-push hook will automatically run. If it fails:
   - **FIX THE TESTS** - Do not try to bypass
   - Run validation script again
   - Only push when all tests pass

4. **NEVER use `git push --no-verify`** unless:
   - You're pushing documentation-only changes
   - You've explicitly coordinated with the team
   - You understand the risks

---

## Maintenance

### Updating the Pre-Push Hook

If the hook needs to be updated:

1. Edit `.git/hooks/pre-push`
2. Test it locally
3. Update this document if behavior changes
4. Notify all developers to re-run `scripts/setup-git-hooks.sh`

### Adding New Validation Checks

To add new checks to the validation process:

1. Add the check to both:
   - `.git/hooks/pre-push`
   - `scripts/validate-before-push.sh`
2. Update this document
3. Update `AGENTS.md` if needed
4. Test thoroughly before committing

---

## Known Limitations

1. **Hook is local** - Must be installed per clone
   - Mitigated by `setup-git-hooks.sh` script
   - Documented in `AGENTS.md` and `scripts/README.md`

2. **Can be bypassed with --no-verify**
   - Documented as emergency-only option
   - Strong warnings in `AGENTS.md`
   - Should only be used with team coordination

3. **Test execution time** - Hook adds ~30-60 seconds to push
   - Acceptable trade-off vs CI failure time
   - Can run validation script before committing to reduce surprise delays

---

## Success Metrics

**How to measure if this is working:**

1. **Zero CI test failures** on new PRs
2. **All agent-submitted PRs pass tests** on first CI run
3. **Reduced time from PR creation to merge** (no fix cycles)
4. **Agents report running validation** before pushing

---

## Related Documentation

- [AGENTS.md](../../AGENTS.md) - Full agent development guide
- [BRANCH_PROTECTION.md](./BRANCH_PROTECTION.md) - PR requirements and CI setup
- [scripts/README.md](../../scripts/README.md) - Scripts documentation

---

**Created:** 2025-11-15
**Updated:** 2025-11-15
**Status:** ✅ Fully Implemented
**Tested:** Partially (manual testing complete, push testing pending)
