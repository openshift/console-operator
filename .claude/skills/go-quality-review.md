---
name: go-quality-review
description: Review Go code for quality, deprecated APIs, and common anti-patterns
tags: [review, go, quality, deprecated]
---

# Go Code Quality Review Skill

Review Go code for quality issues, deprecated APIs, and common problems.

## Deprecated API Detection

### Check for Deprecated Imports and Functions

**Deprecated (Do NOT use):**
```go
// DEPRECATED: Use os.ReadFile
ioutil.ReadFile(filename)
ioutil.WriteFile(filename, data, perm)
ioutil.ReadAll(reader)

// DEPRECATED: Use DialContext
Dial: func(network, addr string) (net.Conn, error) {
    return net.Dial(network, addr)
}

// DEPRECATED: Various k8s.io/utils functions
// Check specific package documentation
```

**Modern alternatives:**
```go
// Use os package (Go 1.16+)
os.ReadFile(filename)
os.WriteFile(filename, data, perm)
io.ReadAll(reader)

// Use DialContext
DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
    return (&net.Dialer{}).DialContext(ctx, network, addr)
}
```

## Error Handling

### Error Wrapping
**Good:**
```go
if err != nil {
    return fmt.Errorf("failed to get config: %w", err)
}
```

**Bad:**
```go
if err != nil {
    return fmt.Errorf("failed to get config: %v", err) // Lost error chain
}
```

### Error Context
Add context to errors:
```go
// Good - explains what was happening
return fmt.Errorf("failed to sync deployment %s in namespace %s: %w", 
    name, namespace, err)

// Bad - no context
return err
```

### Checking Specific Errors
```go
// Good - specific error check
if apierrors.IsNotFound(err) {
    // Handle not found
}

// Bad - string matching
if strings.Contains(err.Error(), "not found") {
    // Fragile
}
```

## Resource Management

### Context Propagation
**Good:**
```go
func (c *Controller) Sync(ctx context.Context, ...) error {
    deployment, err := c.client.Get(ctx, name, metav1.GetOptions{})
    // ...
}
```

**Bad:**
```go
func (c *Controller) Sync(ctx context.Context, ...) error {
    deployment, err := c.client.Get(context.Background(), name, metav1.GetOptions{})
    // Ignores parent context cancellation
}
```

### Defer Usage
```go
// Good - cleanup on all paths
func doWork(ctx context.Context) error {
    resource, err := acquire()
    if err != nil {
        return err
    }
    defer release(resource)
    
    return performWork(resource)
}

// Bad - leaks on early return
func doWork(ctx context.Context) error {
    resource, err := acquire()
    if err != nil {
        return err // LEAK!
    }
    
    if someCondition {
        return errors.New("failed") // LEAK!
    }
    
    release(resource)
    return nil
}
```

## Code Smells

### God Functions
Flag functions longer than ~100 lines or with too many responsibilities:
```go
// BAD - doing too much
func (c *Controller) Sync(...) error {
    // 50 lines of config validation
    // 100 lines of deployment logic
    // 75 lines of service setup
    // 80 lines of route configuration
    // = 305 lines, should be split
}

// GOOD - extracted
func (c *Controller) Sync(...) error {
    if err := c.validateConfig(); err != nil {
        return err
    }
    if err := c.syncDeployment(); err != nil {
        return err
    }
    if err := c.syncService(); err != nil {
        return err
    }
    return c.syncRoute()
}
```

### Magic Values
```go
// Bad - magic numbers/strings
if replicas == 2 {
    // Why 2?
}
timeout := 30 * time.Second // Why 30?
if mode == "special" {
    // What makes it special?
}

// Good - named constants
const (
    DefaultReplicaCount = 2 // HA requirement
    DefaultTimeout = 30 * time.Second
    SpecialMode = "special" // Tech preview mode
)
```

### Deep Nesting
```go
// Bad - deeply nested
if condition1 {
    if condition2 {
        if condition3 {
            if condition4 {
                // business logic buried here
            }
        }
    }
}

// Good - early returns
if !condition1 {
    return nil
}
if !condition2 {
    return nil
}
if !condition3 {
    return nil
}
if !condition4 {
    return nil
}
// business logic at top level
```

## Performance

### String Building
```go
// Bad - inefficient string concatenation
var result string
for _, item := range items {
    result += item + "\n"
}

// Good - strings.Builder
var builder strings.Builder
for _, item := range items {
    builder.WriteString(item)
    builder.WriteString("\n")
}
result := builder.String()
```

### Unnecessary Allocations
```go
// Bad - allocates on every call
func (c *Controller) getNamespace() string {
    return []byte("openshift-console")[0:] // Unnecessary allocation
}

// Good - use constant
const namespace = "openshift-console"
```

## Concurrency

### Missing Mutex Protection
```go
// Bad - race condition
type Cache struct {
    data map[string]string
}
func (c *Cache) Set(k, v string) {
    c.data[k] = v // RACE if called from multiple goroutines
}

// Good - protected
type Cache struct {
    mu   sync.RWMutex
    data map[string]string
}
func (c *Cache) Set(k, v string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[k] = v
}
```

## Testing

### Missing Error Checks
```go
// Bad - ignoring errors in tests
result, _ := doSomething()
if result != expected {
    t.Error("mismatch")
}

// Good - checking errors
result, err := doSomething()
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
if result != expected {
    t.Errorf("expected %v, got %v", expected, result)
}
```

## Documentation

### Exported Functions
```go
// Good - documented export
// NewController creates a new console controller.
// It requires a valid operatorClient and informer factory.
func NewController(...) factory.Controller {
    // ...
}

// Bad - no doc
func NewController(...) factory.Controller {
    // ...
}
```

## Output Format

For each issue:
- **File:Line**: Location
- **Issue**: What's wrong
- **Category**: Deprecated / Error Handling / Code Smell / Performance / etc.
- **Fix**: How to improve
- **Priority**: High / Medium / Low

## Example Review Comments

**High Priority**: pkg/console/operator/sync.go:45 - Using deprecated ioutil.ReadFile. Replace with os.ReadFile.

**Medium Priority**: pkg/console/controllers/route/controller.go:123 - Deep nesting (5 levels). Consider early returns.

**Low Priority**: Consider adding godoc comment for exported function NewRouteController.

**Info**: Good use of context propagation throughout sync handler.
