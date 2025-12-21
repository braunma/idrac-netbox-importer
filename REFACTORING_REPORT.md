# Code Refactoring Report

This document outlines code quality issues, redundant patterns, and refactoring opportunities discovered in the iDRAC NetBox Importer codebase.

## Executive Summary

**Status**: The codebase is well-structured with good separation of concerns, but contains several redundant patterns and one critical missing component.

**Critical Issues**: 3 (2 fixed, 1 remaining)
**Refactoring Opportunities**: 8 patterns identified
**Overall Code Quality**: Good architecture with room for DRY improvements

---

## Critical Issues

### 1. ✅ FIXED - Duplicate Import in formatter.go

**Location**: `internal/output/formatter.go:213`

**Issue**: The file had a duplicate `import "time"` statement in the middle of the code, which is a Go compilation error.

**Status**: **FIXED** - Removed duplicate import and moved `time` to the main import block at the top of the file.

**Before**:
```go
// Line 213
import "time"  // ❌ Duplicate - time not in main imports
```

**After**:
```go
// Line 10 (in main import block)
import (
    "encoding/json"
    "fmt"
    "io"
    "strings"
    "text/tabwriter"
    "time"  // ✅ Correctly placed

    "github.com/yourusername/idrac-inventory/internal/models"
)
```

---

### 2. ✅ FIXED - Duplicate Import in main.go

**Location**: `cmd/idrac-inventory/main.go:306`

**Issue**: The file had a duplicate import statement at the end of the file.

**Status**: **FIXED** - Removed the duplicate import at the end of the file. The models package was already imported at the top.

**Before**:
```go
// Line 306 (end of file)
// Import models for outputResults
import "github.com/yourusername/idrac-inventory/internal/models"  // ❌ Duplicate
```

**After**:
```go
// Removed - already imported at line 17
```

---

### 3. ❌ CRITICAL - Missing Scanner Implementation

**Location**: `internal/scanner/scanner.go` (file does not exist)

**Issue**: The scanner package is missing its main implementation file. Only the test file exists (`scanner_test.go`), but there's no corresponding implementation.

**Impact**:
- **The application will not compile or run**
- `main.go` imports and uses `scanner.New()` which doesn't exist
- Tests reference scanner functionality but there's nothing to test

**Expected Implementation** (based on test file and usage patterns):
```go
package scanner

import (
    "context"
    "sync"
    "time"

    "github.com/yourusername/idrac-inventory/internal/config"
    "github.com/yourusername/idrac-inventory/internal/models"
)

type Scanner struct {
    cfg        *config.Config
    httpClient *http.Client
    // ... other fields
}

func New(cfg *config.Config) *Scanner {
    // Initialize scanner with config
}

func (s *Scanner) ScanAll(ctx context.Context) ([]models.ServerInfo, models.CollectionStats) {
    // Parallel scan implementation with worker pool
}

func (s *Scanner) ValidateConnections(ctx context.Context) map[string]error {
    // Connection validation
}
```

**Required Methods** (from usage in main.go):
- `New(cfg *config.Config) *Scanner` - Constructor
- `ScanAll(ctx context.Context) ([]models.ServerInfo, models.CollectionStats)` - Scan all servers
- `ValidateConnections(ctx context.Context) map[string]error` - Validate connections

**Recommendation**: Implement `scanner.go` as the highest priority task.

---

## Refactoring Opportunities

### 1. Duplicate Error Counting Pattern

**Severity**: Medium
**Files**: `cmd/idrac-inventory/main.go`
**Lines**: 221-234, 284-293

**Issue**: The same pattern of iterating through results and counting failures appears twice.

**Current Code**:
```go
// In runValidateConnections (lines 221-234)
successCount := 0
for host, err := range results {
    if err != nil {
        fmt.Printf("❌ %s: %v\n", host, err)
    } else {
        fmt.Printf("✅ %s: OK\n", host)
        successCount++
    }
}

// In runNetBoxSync (lines 284-293)
failCount := 0
for _, r := range syncResults {
    if !r.Success {
        failCount++
    }
}
```

**Refactored Solution**:
```go
// Add to a new internal/util/results.go file
package util

type Result interface {
    IsSuccess() bool
    GetHost() string
    GetError() error
}

func CountResults(results []Result) (success, failed int) {
    for _, r := range results {
        if r.IsSuccess() {
            success++
        } else {
            failed++
        }
    }
    return
}

func PrintResults(results []Result, successIcon, failIcon string) int {
    successCount := 0
    for _, r := range results {
        if r.GetError() != nil {
            fmt.Printf("%s %s: %v\n", failIcon, r.GetHost(), r.GetError())
        } else {
            fmt.Printf("%s %s: OK\n", successIcon, r.GetHost())
            successCount++
        }
    }
    return successCount
}
```

**Benefits**:
- Eliminates code duplication
- Easier to maintain
- Consistent result handling across the application

---

### 2. Redundant Timeout Conversion Methods

**Severity**: Low
**Files**: `internal/config/config.go`
**Lines**: 42-48, 108-114, 92-98

**Issue**: Multiple structs have nearly identical `Timeout()` methods that convert seconds to `time.Duration`.

**Current Code**:
```go
// NetBoxConfig.Timeout() - lines 42-48
func (n NetBoxConfig) Timeout() time.Duration {
    if n.TimeoutSeconds <= 0 {
        return defaults.GetNetBoxTimeout()
    }
    return time.Duration(n.TimeoutSeconds) * time.Second
}

// DefaultsConfig.Timeout() - lines 108-114
func (d DefaultsConfig) Timeout() time.Duration {
    if d.TimeoutSeconds <= 0 {
        return defaults.GetTimeout()
    }
    return time.Duration(d.TimeoutSeconds) * time.Second
}

// ServerConfig.GetTimeout() - lines 92-98
func (s ServerConfig) GetTimeout(defaultTimeout time.Duration) time.Duration {
    if s.TimeoutSeconds != nil && *s.TimeoutSeconds > 0 {
        return time.Duration(*s.TimeoutSeconds) * time.Second
    }
    return defaultTimeout
}
```

**Refactored Solution**:
```go
// Add to pkg/defaults/defaults.go or internal/config/helpers.go
package config

import "time"

// secondsToDuration converts a seconds value to time.Duration,
// returning defaultDuration if seconds <= 0
func secondsToDuration(seconds int, defaultDuration time.Duration) time.Duration {
    if seconds <= 0 {
        return defaultDuration
    }
    return time.Duration(seconds) * time.Second
}

// secondsPtrToDuration converts a pointer to seconds value to time.Duration
func secondsPtrToDuration(seconds *int, defaultDuration time.Duration) time.Duration {
    if seconds == nil || *seconds <= 0 {
        return defaultDuration
    }
    return time.Duration(*seconds) * time.Second
}

// Then use in methods:
func (n NetBoxConfig) Timeout() time.Duration {
    return secondsToDuration(n.TimeoutSeconds, defaults.GetNetBoxTimeout())
}

func (d DefaultsConfig) Timeout() time.Duration {
    return secondsToDuration(d.TimeoutSeconds, defaults.GetTimeout())
}

func (s ServerConfig) GetTimeout(defaultTimeout time.Duration) time.Duration {
    return secondsPtrToDuration(s.TimeoutSeconds, defaultTimeout)
}
```

**Benefits**:
- Single source of truth for timeout conversion
- Easier to add validation or logging
- Consistent handling of edge cases

---

### 3. Repeated Default Value Checks

**Severity**: Low
**Files**: `internal/config/config.go`
**Lines**: 138-165, 174-187

**Issue**: Multiple methods follow the pattern: "if value <= 0, return default, else return value"

**Current Code**:
```go
func (r RetryConfig) GetMaxAttempts() int {
    if r.MaxAttempts <= 0 {
        return defaults.DefaultRetryMaxAttempts
    }
    return r.MaxAttempts
}

func (h HTTPConfig) GetMaxIdleConns() int {
    if h.MaxIdleConns <= 0 {
        return defaults.DefaultHTTPMaxIdleConns
    }
    return h.MaxIdleConns
}

func (h HTTPConfig) GetIdleConnTimeout() time.Duration {
    if h.IdleConnTimeoutSec <= 0 {
        return defaults.GetHTTPIdleConnTimeout()
    }
    return time.Duration(h.IdleConnTimeoutSec) * time.Second
}
```

**Refactored Solution**:
```go
// Add generic helper
func getOrDefault[T comparable](value, defaultValue T, zeroValue T) T {
    if value == zeroValue {
        return defaultValue
    }
    return value
}

// Or for numeric types specifically:
func getIntOrDefault(value, defaultValue int) int {
    if value <= 0 {
        return defaultValue
    }
    return value
}

// Usage:
func (r RetryConfig) GetMaxAttempts() int {
    return getIntOrDefault(r.MaxAttempts, defaults.DefaultRetryMaxAttempts)
}

func (h HTTPConfig) GetMaxIdleConns() int {
    return getIntOrDefault(h.MaxIdleConns, defaults.DefaultHTTPMaxIdleConns)
}
```

**Benefits**:
- DRY principle
- Type-safe with Go generics (Go 1.18+)
- Easier to extend with validation

---

### 4. Duplicate NetBox Field Name Mapping

**Severity**: Low
**Files**: `internal/netbox/client.go`
**Lines**: 50-67

**Issue**: The `DefaultFieldNames()` function manually maps all 14 field names from the defaults package, essentially duplicating data.

**Current Code**:
```go
func DefaultFieldNames() FieldNames {
    return FieldNames{
        CPUCount:       defaults.NetBoxFieldCPUCount,
        CPUModel:       defaults.NetBoxFieldCPUModel,
        CPUCores:       defaults.NetBoxFieldCPUCores,
        // ... 11 more fields ...
    }
}
```

**Refactored Solution**:

**Option 1**: Move FieldNames struct to defaults package
```go
// pkg/defaults/netbox.go
package defaults

type NetBoxFieldNames struct {
    CPUCount       string
    CPUModel       string
    // ...
}

func GetNetBoxFieldNames() NetBoxFieldNames {
    return NetBoxFieldNames{
        CPUCount:   getEnvOrDefault("NETBOX_FIELD_CPU_COUNT", "hw_cpu_count"),
        CPUModel:   getEnvOrDefault("NETBOX_FIELD_CPU_MODEL", "hw_cpu_model"),
        // ...
    }
}
```

**Option 2**: Use struct tags for reflection
```go
type FieldNames struct {
    CPUCount   string `env:"NETBOX_FIELD_CPU_COUNT" default:"hw_cpu_count"`
    CPUModel   string `env:"NETBOX_FIELD_CPU_MODEL" default:"hw_cpu_model"`
    // ...
}

func DefaultFieldNames() FieldNames {
    return loadFieldNamesFromEnv() // Use reflection
}
```

**Benefits**:
- Single source of truth
- Eliminates manual mapping code
- Easier to add new fields

---

### 5. Repeated Fallback Logic in ServerConfig

**Severity**: Low
**Files**: `internal/config/config.go`
**Lines**: 61-98

**Issue**: Multiple `GetX()` methods follow the same "return value if set, else return default" pattern.

**Current Code**:
```go
func (s ServerConfig) GetUsername(defaultUser string) string {
    if s.Username != "" {
        return s.Username
    }
    return defaultUser
}

func (s ServerConfig) GetPassword(defaultPass string) string {
    if s.Password != "" {
        return s.Password
    }
    return defaultPass
}

func (s ServerConfig) GetDisplayName() string {
    if s.Name != "" {
        return s.Name
    }
    return s.Host
}
```

**Refactored Solution**:
```go
// Generic helper for string fallback
func getStringOrDefault(value, defaultValue string) string {
    if value != "" {
        return value
    }
    return defaultValue
}

func (s ServerConfig) GetUsername(defaultUser string) string {
    return getStringOrDefault(s.Username, defaultUser)
}

func (s ServerConfig) GetPassword(defaultPass string) string {
    return getStringOrDefault(s.Password, defaultPass)
}

func (s ServerConfig) GetDisplayName() string {
    return getStringOrDefault(s.Name, s.Host)
}
```

**Benefits**:
- Reduced boilerplate
- Consistent null-checking
- Easier testing

---

### 6. Similar Status Formatting Methods

**Severity**: Low
**Files**: `internal/output/formatter.go`
**Lines**: 182-210

**Issue**: `formatPowerState` and `formatHealth` follow nearly identical patterns.

**Current Code**:
```go
func (f *ConsoleFormatter) formatPowerState(state string) string {
    if f.NoColor {
        return state
    }
    switch state {
    case "On":
        return "🟢 On"
    case "Off":
        return "🔴 Off"
    default:
        return "🟡 " + state
    }
}

func (f *ConsoleFormatter) formatHealth(health string) string {
    if f.NoColor {
        return health
    }
    switch health {
    case "OK":
        return "✓ OK"
    case "Warning":
        return "⚠ Warning"
    case "Critical":
        return "✗ Critical"
    default:
        return health
    }
}
```

**Refactored Solution**:
```go
type StatusMapping map[string]string

var powerStateIcons = StatusMapping{
    "On":  "🟢",
    "Off": "🔴",
}

var healthIcons = StatusMapping{
    "OK":       "✓",
    "Warning":  "⚠",
    "Critical": "✗",
}

func (f *ConsoleFormatter) formatWithIcon(value string, mapping StatusMapping, defaultIcon string) string {
    if f.NoColor {
        return value
    }
    if icon, ok := mapping[value]; ok {
        return icon + " " + value
    }
    if defaultIcon != "" {
        return defaultIcon + " " + value
    }
    return value
}

func (f *ConsoleFormatter) formatPowerState(state string) string {
    return f.formatWithIcon(state, powerStateIcons, "🟡")
}

func (f *ConsoleFormatter) formatHealth(health string) string {
    return f.formatWithIcon(health, healthIcons, "")
}
```

**Benefits**:
- Eliminates duplicate switch logic
- Easy to add new status types
- Icons defined as data, not code

---

### 7. Redundant Device Lookup Logic

**Severity**: Medium
**Files**: `internal/netbox/client.go`
**Lines**: 234-254, 287-303

**Issue**: Device lookup by service tag and fallback to serial is duplicated.

**Current Code**:
```go
// In FindDeviceByServiceTag (lines 234-254)
func (c *Client) FindDeviceByServiceTag(ctx context.Context, serviceTag string) (*Device, error) {
    // ... search by asset_tag ...
    if result.Count > 0 {
        return &result.Results[0], nil
    }
    // Fall back to serial number search
    return c.FindDeviceBySerial(ctx, serviceTag)
}

// In SyncServerInfo (lines 287-303)
var device *Device
var err error

if info.ServiceTag != "" {
    device, err = c.FindDeviceByServiceTag(ctx, info.ServiceTag)
    if err != nil {
        return err
    }
}

if device == nil && info.SerialNumber != "" {
    device, err = c.FindDeviceBySerial(ctx, info.SerialNumber)
    if err != nil {
        return err
    }
}
```

**Refactored Solution**:
```go
// Simplify SyncServerInfo to use FindDeviceByServiceTag directly
// since it already has the fallback logic
func (c *Client) SyncServerInfo(ctx context.Context, info models.ServerInfo) error {
    // ... logging ...

    // Try service tag first (includes serial fallback)
    device, err := c.findDevice(ctx, info)
    if err != nil {
        return err
    }

    if device == nil {
        return fmt.Errorf("device not found in NetBox (service_tag=%s, serial=%s)",
            info.ServiceTag, info.SerialNumber)
    }

    // ... rest of method ...
}

// New helper method
func (c *Client) findDevice(ctx context.Context, info models.ServerInfo) (*Device, error) {
    if info.ServiceTag != "" {
        device, err := c.FindDeviceByServiceTag(ctx, info.ServiceTag)
        if err != nil || device != nil {
            return device, err
        }
    }

    if info.SerialNumber != "" {
        return c.FindDeviceBySerial(ctx, info.SerialNumber)
    }

    return nil, nil
}
```

**Benefits**:
- Single device lookup logic
- Clearer intent
- Easier to add additional search methods

---

### 8. Repeated Validation Error Building

**Severity**: Low
**Files**: `internal/config/config.go`
**Lines**: 279-335

**Issue**: The validation method builds error messages using the same pattern repeatedly.

**Current Code**:
```go
func (c *Config) Validate() error {
    var errs []string

    if len(c.Servers) == 0 {
        errs = append(errs, "no servers configured")
    }

    for i, srv := range c.Servers {
        if srv.Host == "" {
            errs = append(errs, fmt.Sprintf("server[%d]: host is required", i))
        }
        // ... more error checks ...
    }

    if len(errs) > 0 {
        return errors.New(strings.Join(errs, "; "))
    }
    return nil
}
```

**Refactored Solution**:
```go
// Use the existing MultiError type from pkg/errors
func (c *Config) Validate() error {
    errs := &errors.MultiError{}

    if len(c.Servers) == 0 {
        errs.Add(errors.NewConfigError("servers", "no servers configured"))
    }

    for i, srv := range c.Servers {
        if srv.Host == "" {
            errs.Add(errors.NewConfigError(fmt.Sprintf("server[%d].host", i), "host is required"))
        }
        // ... more error checks using errs.Add() ...
    }

    return errs.ErrorOrNil()
}
```

**Benefits**:
- Uses existing error infrastructure
- Better error type support
- Supports errors.Is() for error checking

---

## Recommended Refactoring Priority

### High Priority (Do First)
1. **Implement scanner.go** - Critical blocking issue
2. **Fix duplicate imports** - ✅ Already fixed

### Medium Priority (Next Sprint)
3. **Extract error counting pattern** - Used in multiple places
4. **Simplify device lookup logic** - Improves NetBox integration reliability

### Low Priority (Technical Debt)
5. **Consolidate timeout conversion methods**
6. **Extract status formatting logic**
7. **Simplify default value helpers**
8. **Refactor field name mapping**
9. **Use MultiError in validation**

---

## Code Quality Metrics

### Strengths
✅ Good package organization (cmd, internal, pkg separation)
✅ Centralized defaults in dedicated package
✅ Interface-based design (Formatter interface)
✅ Custom error types with context
✅ Structured logging throughout
✅ Comprehensive test coverage (where implemented)
✅ Environment variable support for config
✅ Docker containerization

### Weaknesses
❌ Missing core scanner implementation
❌ Some code duplication (addressed in this report)
❌ Manual field mapping (could use reflection)
❌ Repeated validation patterns

### Overall Assessment
The codebase demonstrates **good architectural decisions** with proper separation of concerns and consistent patterns. The main issues are:
1. The missing scanner implementation (critical)
2. Opportunities to reduce code duplication through helper functions
3. Some manual mappings that could be automated

**Recommendation**: After implementing scanner.go, focus on extracting common patterns into helpers. The refactorings are not urgent but would improve maintainability.

---

## Implementation Notes

### Testing After Refactoring
After applying any refactoring:
1. Run full test suite: `go test ./...`
2. Verify linting: `golangci-lint run`
3. Check for unused code: `staticcheck ./...`
4. Run integration tests
5. Test with real iDRAC servers

### Breaking Changes
None of the proposed refactorings should introduce breaking changes since:
- All changes are internal implementation details
- Public API remains the same
- Config file format unchanged
- CLI flags unchanged

### Migration Path
1. Implement scanner.go (required)
2. Add helper functions without removing old code
3. Gradually migrate existing code to use helpers
4. Remove old code once all references updated
5. Run tests at each step

---

## Conclusion

The iDRAC NetBox Importer codebase is well-structured with good architectural patterns. The main issues are:

1. **Critical**: Missing scanner implementation (blocks compilation)
2. **Fixed**: Duplicate import statements
3. **Opportunities**: Several patterns could be consolidated to improve maintainability

After implementing the missing scanner.go file and applying the recommended refactorings, the codebase will be more maintainable, testable, and follow the DRY (Don't Repeat Yourself) principle more closely.

The refactorings are **backward compatible** and can be applied incrementally without risk to the existing functionality.
