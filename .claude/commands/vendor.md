---
description: Vendor openshift/api at a specific commit SHA
---

You are helping vendor the openshift/api repository at a specific commit SHA.

# Usage

The user can invoke this command with a commit SHA and optional JIRA ticket:
- `/vendor <SHA>` - Vendor openshift/api at the specified commit SHA
  - JIRA ID will be auto-detected from branch name if it matches `CONSOLE-<number>` or `OCPBUGS-<number>` pattern
- `/vendor <SHA> <JIRA-ID>` - Vendor with explicit JIRA ticket for commit messages (e.g., `/vendor abc123 CONSOLE-1234` or `/vendor abc123 OCPBUGS-5678`)
  - Explicit JIRA ID overrides branch name detection
- `/vendor` - If no SHA provided, ask the user for it

# Task

Follow these steps to vendor openshift/api at the specified commit:

## 1. Parse Arguments and Detect JIRA ID
- Extract the SHA from the first argument after /vendor
- Extract optional JIRA ticket ID from second argument (e.g., "CONSOLE-1234" or "OCPBUGS-5678")
- If no SHA is provided, ask: "Please provide the commit SHA from openshift/api to vendor (e.g., `/vendor abc123` or `/vendor abc123 CONSOLE-1234`)"
- The SHA can be either full (40 characters) or short (7+ characters)

**Auto-detect JIRA ID from branch name:**
- Run: `git branch --show-current` to get the current branch name
- Check if branch name matches pattern `CONSOLE-<number>` or `OCPBUGS-<number>` (case-insensitive)
- If it matches and no JIRA ID was provided as argument, extract and use it
  - Examples:
    - Branch `CONSOLE-1234-feature` → JIRA ID `CONSOLE-1234`
    - Branch `ocpbugs-5678-fix-bug` → JIRA ID `OCPBUGS-5678`
- If JIRA ID was provided as argument, it takes precedence over branch name
- Display which JIRA ID will be used (or "none" if not applicable)

## 2. Check Git Status
- Run: `git status --porcelain` to check for uncommitted changes
- If there are uncommitted changes in files other than `.claude/commands/vendor.md`, warn the user and ask if they want to proceed

## 3. Show Current Version
- Run: `grep 'github.com/openshift/api' go.mod` to show the current version
- Display it to the user so they know what will change

## 4. Update Dependency
- Run: `go get github.com/openshift/api@<SHA>`
- If this fails (invalid SHA or network error):
  - Display the error message
  - Ask the user to verify the SHA is valid at https://github.com/openshift/api/commits

## 5. Update Modules and Vendor
- Run: `go mod tidy && go mod vendor`
- This tidies go.mod and updates the vendor directory
- Note: Removed `go clean -modcache` as it's unnecessarily aggressive (clears cache system-wide)

## 6. Verify Changes
- Run: `go mod verify`
- If verification fails:
  - Display the error
  - Suggest running `go clean -modcache && go mod download` to re-download modules
  - Ask user how to proceed

## 7. Run Tests
- Run: `make test-unit` to verify the vendored code didn't break existing functionality
- If tests fail:
  - Analyze the error output carefully
  - Identify which Go package the test belongs to (e.g., `github.com/openshift/console-operator/pkg/console`)
  - Fix errors by editing the relevant source files
  - Re-run only the failing tests: `TESTABLE=<package-path> make test-unit`
    - Example: `TESTABLE=./pkg/console make test-unit`
  - Continue until all tests pass (maximum 3 fix attempts)
  - If tests still fail after 3 attempts, report the failures and ask how to proceed
  - Once fixed, create a brief summary of the fix nature for the commit message (store in `FIX` variable)

## 8. Commit Changes
Create one or two commits depending on whether code fixes were needed:

**Commit 1 - Dependency bump (always created):**
- Files: `go.mod`, `go.sum`, and `vendor/` directory
- Format: `<JIRA>: Bump openshift/api to <SHORT-SHA>` (if JIRA provided)
- Format: `Bump openshift/api to <SHORT-SHA>` (if no JIRA)
- Use first 7-8 characters of SHA for SHORT-SHA

**Commit 2 - Code fixes (only if fixes were needed):**
- Files: All other modified files
- Format: `<JIRA>: <FIX>` (if JIRA provided)
- Format: `<FIX>` (if no JIRA)
- Example FIX: "Update API types and fix test assertions"

Commands:
```bash
# Add and commit vendor changes
git add go.mod go.sum vendor/
git commit -m "<commit-message-1>"

# If fixes exist, commit them separately
git add .
git commit -m "<commit-message-2>"
```

## 9. Show Summary
- Run: `grep 'github.com/openshift/api' go.mod` to show the new version
- Display what changed: old version → new version
- List commits created: `git log --oneline -2`
- Confirm successful vendoring

# Important Notes

- The console-operator depends on openshift/api for OpenShift resource types
- Vendoring stores all dependencies in the vendor/ directory for reproducible builds
- The SHA must be a valid commit from https://github.com/openshift/api
- JIRA IDs are auto-detected from branch names matching `CONSOLE-<number>` or `OCPBUGS-<number>` patterns
- The two-commit approach separates mechanical changes (vendor bump) from logical changes (code fixes)
- If something goes wrong, you can rollback with: `git reset --hard HEAD~1` (or `HEAD~2` if both commits were made)

# Error Recovery

If the vendoring process fails at any step:
1. Identify the failing step and error message
2. For go.mod issues: Consider running `go mod tidy` again
3. For test failures: Analyze and fix up to 3 times, then ask user
4. For git issues: Check if files are in the correct state
5. User can always rollback: `git checkout go.mod go.sum && rm -rf vendor/ && go mod vendor`

# Output Format

Provide clear, step-by-step updates so the user knows what's happening at each stage. Use the TodoWrite tool to track progress through the steps.
