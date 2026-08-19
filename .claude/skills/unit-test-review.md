---
name: unit-test-review
description: Review Go unit tests for best practices, table-driven patterns, and proper assertions
tags: [review, testing, unit, go]
---

# Unit Test Review Skill

Review unit tests in `pkg/**/*_test.go` files for correctness and best practices.

## What to Check

### 1. Table-Driven Test Pattern
Most unit tests should use table-driven pattern for clarity and completeness:
```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected OutputType
        wantErr  bool
    }{
        {
            name:     "happy path",
            input:    validInput,
            expected: expectedOutput,
            wantErr:  false,
        },
        {
            name:     "error case",
            input:    invalidInput,
            expected: OutputType{},
            wantErr:  true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionUnderTest(tt.input)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
            }
            
            if diff := deep.Equal(result, tt.expected); diff != nil {
                t.Error(diff)
            }
        })
    }
}
```

### 2. Test Naming
**Good test names:**
- `TestGetNodeComputeEnvironments` - describes what function is tested
- `TestNewRouteConfig` - clear and descriptive
- Test case names: `"Custom hostname and TLS secret set"` - explains scenario

**Bad test names:**
- `TestFunc1` - not descriptive
- `TestIt` - what is "it"?
- Test case names: `"test1"` - doesn't explain what's being tested

### 3. Deep Equality Checks
Use `github.com/go-test/deep` for struct comparisons:
```go
// Good - shows exact differences
if diff := deep.Equal(actual, expected); diff != nil {
    t.Error(diff)
}

// Bad - unhelpful error message
if actual != expected {
    t.Error("mismatch")
}

// Bad - manual field-by-field comparison (fragile)
if actual.Field1 != expected.Field1 || actual.Field2 != expected.Field2 {
    t.Error("fields don't match")
}
```

### 4. Test Coverage
**Test both success and failure paths:**
```go
tests := []struct {
    name    string
    input   string
    wantErr bool
}{
    {name: "valid input", input: "valid", wantErr: false},
    {name: "empty input", input: "", wantErr: true},
    {name: "invalid format", input: "bad", wantErr: true},
}
```

**Edge cases to consider:**
- Empty inputs (nil, "", empty slices/maps)
- Boundary values (0, -1, max int)
- Missing labels/fields
- Duplicate values
- Large inputs

### 5. Test Structure (Arrange-Act-Assert)
```go
t.Run("test scenario", func(t *testing.T) {
    // Arrange - setup test data
    config := &operatorv1.Console{
        Spec: operatorv1.ConsoleSpec{
            Route: operatorv1.ConsoleConfigRoute{
                Hostname: "custom.example.com",
            },
        },
    }
    
    // Act - call the function
    result := ProcessConfig(config)
    
    // Assert - verify results
    if diff := deep.Equal(result, expected); diff != nil {
        t.Error(diff)
    }
})
```

### 6. Error Handling
**Check error presence:**
```go
// Good - checks if error occurred when expected
if (err != nil) != tt.wantErr {
    t.Errorf("wantErr %v, got error: %v", tt.wantErr, err)
}

// Bad - ignores error
result, _ := FunctionUnderTest(input)
```

**Check error messages (when specific):**
```go
if err != nil && !strings.Contains(err.Error(), "expected substring") {
    t.Errorf("unexpected error message: %v", err)
}
```

### 7. Mocking and Test Isolation
**Prefer interfaces for testability:**
```go
// Good - uses interface, easy to mock
type ConfigGetter interface {
    GetConfig() (*Config, error)
}

func ProcessData(getter ConfigGetter) error {
    config, err := getter.GetConfig()
    // ...
}

// Bad - hard-coded dependency
func ProcessData() error {
    config := getConfigFromKubernetes() // can't test without cluster
    // ...
}
```

**Keep tests isolated:**
- Tests should not depend on execution order
- Tests should not share mutable state
- Each test case should be independent

### 8. Helper Functions
Extract common setup into helpers:
```go
func testConsole() *operatorv1.Console {
    return &operatorv1.Console{
        Spec: operatorv1.ConsoleSpec{
            // common test config
        },
    }
}

func TestMultipleFunctions(t *testing.T) {
    console := testConsole()
    // modify as needed for specific test
}
```

### 9. Assertion Quality
**Good assertions:**
```go
// Specific error message with context
if result != expected {
    t.Errorf("expected %d nodes, got %d", expected, result)
}

// Shows what differs
if diff := deep.Equal(actual, expected); diff != nil {
    t.Errorf("config mismatch: %v", diff)
}
```

**Bad assertions:**
```go
// Vague - what failed?
if result != expected {
    t.Error("failed")
}

// Silent failure
if result != expected {
    // no error reported
}
```

### 10. Test Data Management
**Inline simple data:**
```go
tests := []struct {
    name  string
    input int
}{
    {name: "zero", input: 0},
    {name: "one", input: 1},
}
```

**Extract complex fixtures:**
```go
// testdata/valid_config.yaml
// or helper functions for complex objects
func validConsoleConfig() *operatorv1.Console {
    return &operatorv1.Console{
        // complex config
    }
}
```

## Red Flags

- No table-driven tests for functions with multiple scenarios
- Tests without subtests (can't see which case failed)
- Tests that depend on execution order
- Global mutable state between tests
- Hardcoded sleeps (use mocks/fakes instead)
- Tests without assertions (just calling the function)
- Ignoring errors with `_`
- Testing implementation details instead of behavior
- Overly complex test setup (refactor the code, not the test)

## Common Patterns in This Codebase

### Using go-test/deep
```go
import "github.com/go-test/deep"

if diff := deep.Equal(actual, expected); diff != nil {
    t.Error(diff)
}
```

### Testing Functions That Return Errors
```go
result, err := FunctionUnderTest(input)

if (err != nil) != tt.wantErr {
    t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
    return
}

if !tt.wantErr && diff := deep.Equal(result, tt.expected); diff != nil {
    t.Error(diff)
}
```

### Testing with Kubernetes Objects
```go
node := &v1.Node{
    ObjectMeta: metav1.ObjectMeta{
        Name: "test-node",
        Labels: map[string]string{
            "key": "value",
        },
    },
}
```

## Output Format

For each issue:
- **Test**: Test function name
- **Issue**: What's wrong
- **Impact**: Why it matters
- **Fix**: How to improve

## Example Review Comments

**Critical**: TestProcessConfig has no assertions - test always passes even if function is broken.

**Warning**: Using `==` to compare structs instead of `deep.Equal` - test won't show what differs.

**Suggestion**: Consider adding test case for empty input to TestGetNodes - edge case not covered.

**Info**: Good use of table-driven tests with clear scenario names.
