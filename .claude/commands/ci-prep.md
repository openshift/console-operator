---
description: Format, build, test, fix errors, and generate unit tests for changed code
argument-hint: [base-branch]
---

Prepare your code for CI/PR submission with comprehensive linting, building, testing, error fixing, and unit test generation.

**Arguments:**
- `$1` (optional): Base branch to compare against (auto-detected if not provided)

## Pre-flight Check:

First, determine the base branch:
1. If `$1` is provided, use that as the base branch
2. Otherwise, auto-detect using these steps in order:
   a. Check where the branch was created from using reflog: `git reflog show --format='%gs' $(git rev-parse --abbrev-ref HEAD) | grep -m1 'branch: Created from' | sed 's/.*Created from //'` - Use this value exactly as found
   b. If reflog doesn't have creation info (branch too old, created elsewhere, or reflog pruned), check for common base branches: 'origin/main', 'origin/master', 'main', 'master' (use first that exists)
   c. If none exist, default to 'main'
3. Display: "Base branch for comparison: {detected-branch}"

Current status: !`git status --short`

If there are uncommitted changes, ask the user if they want to:
- Commit them first
- Proceed withouth any action
- Stash them and proceed
- Cancel the operation

---

Execute the build-fix-test cycle in five stages:

## First Stage - Formatting and Linting:
1. Run `go fmt ./...` to format all Go code
2. Run `go vet ./...` to check for common mistakes and issues
3. If `go vet` reports issues, analyze them carefully
4. Fix linting issues by editing the relevant source files
5. Re-run `go vet ./...` after each fix to verify it works
6. Continue until all issues are resolved (max 3 fix attempts)
7. If issues remain after 3 fix attempts, report them to the user and ask how to proceed
8. Once formatting and linting succeed with no errors, prepare to commit:
   - Run `git diff` to review all changes
   - Show a summary of what was fixed
   - Ask user to confirm the commit message
   - Create a git commit with a descriptive message only after user confirmation

## Second Stage - Build:
1. Run `make build` to build the project
2. If the build fails, analyze the errors carefully
3. Fix each error by editing the relevant source files
4. Re-run the build after each fix to verify it works
5. Continue until the build is completely successful (max 3 fix attempts)
6. If build still fails after 3 fix attempts, report the remaining errors to the user and ask how to proceed
7. Once the build succeeds with no errors, prepare to commit:
   - Run `git diff` to review all changes
   - Show a summary of what was fixed
   - Ask user to confirm the commit message
   - Create a git commit with a descriptive message only after user confirmation

## Third Stage - Verify Functionality:
1. Run `make test-unit` to verify if the new code did not break any present functionality
2. If the tests fail, analyze the error carefully
3. Fix each error by editing the relevant source files
4. Re-run only the failing tests by setting the `TESTABLE` envar, which represents the go package the failing test is from
5. Continue until all tests pass successfully (max 3 fix attempts)
6. If tests still fail after 3 fix attempts, report the remaining failures to the user and ask how to proceed
7. Once the tests succeed with no errors, prepare to commit:
   - Run `git diff` to review all changes
   - Show a summary of what was fixed
   - Ask user to confirm the commit message
   - Create a git commit with a descriptive message only after user confirmation

## Fourth Stage - Detect Missing Unit Tests:
1. Find divergence point by running: `git merge-base HEAD <base-branch>` (using the base branch detected in pre-flight check)
2. Get changed Go files (excluding tests) by running: `git diff <merge-base>..HEAD --name-only '*.go' '!*_test.go'`
3. For each changed file, analyze the diff to identify new/modified exported functions and methods
4. Check if corresponding test file exists (e.g., `foo.go` -> `foo_test.go`)
5. For each test file, verify if tests exist for the new/modified functions
6. List all functions missing unit tests
7. Ask the user: "Generate tests for all missing functions at once? (Otherwise approve them separately) (yes/no)"
   - If user answers "yes": Generate unit tests for ALL missing functions automatically
   - If user answers "no": For EACH function separately, ask: "Generate unit test for function X in package Y? (yes/no)"
8. Based on response, generate tests following existing test patterns in the codebase
9. If the test file is missing, create it. If it is present, update it

## Fifth Stage - Re-run Third Stage:
1. Execute the third stage again to verify all new unit tests pass
2. Fix any failing tests (max 3 fix attempts)
3. If tests still fail after 3 fix attempts, report the remaining failures to the user and ask how to proceed
4. Once all tests pass, prepare to commit:
   - Run `git diff` to review all new unit tests
   - Show a summary of tests that were added
   - Ask user to confirm the commit message
   - Create a git commit with a descriptive message only after user confirmation

IMPORTANT: Follow each stage sequentially and complete each one before moving to the next. Use the TodoWrite tool to track progress through all stages and tasks.
