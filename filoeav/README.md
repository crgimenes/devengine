# filoeav Package

This package provides EAV-aware builtins for the Filo scripting language, allowing scripts to read, query, and aggregate EAV (Entity-Attribute-Value) data.

## Architecture

```
Filo Engine <- filoeav <- EAVStore Interface <- db.SQLite
```

The `filoeav` package:
- Defines the `EAVStore` interface for data access abstraction

## Installation

The package is part of the main project. Import it directly:

```go
import "github.com/crgimenes/devengine/filoeav"
```

## Usage

```go
package main

import (
    "context"
    "time"

    "github.com/crgimenes/devengine/db"
    "github.com/crgimenes/devengine/filo"
    "github.com/crgimenes/devengine/filoeav"
)

func main() {
    // Initialize database
    storage, _ := db.NewWithPath("app.db")
    defer storage.Close()

    // Create Filo engine
    engine := filo.NewEngine()

    // Register EAV builtins with workspace context
    filoeav.RegisterEAVBuiltins(engine, storage, filoeav.Config{
        WorkspaceID: 1,  // Current workspace
        UserID:      42, // Current user (for future permission checks)
    })

    // Run a script that uses EAV data
    ctx := context.Background()
    cfg := filo.EvalConfig{
        StepLimit:      10000,
        RecursionLimit: 64,
        Timeout:        5 * time.Second,
    }

    globals := map[string]filo.Value{
        "record-id": filo.VNum(123),
    }

    result, _, _ := engine.RunScript(ctx, `
        (let ((price (eav-get-value "products" record-id "unit_price"))
              (qty (eav-get-value "products" record-id "quantity")))
          (* price qty))
    `, globals, cfg)

    fmt.Println("Total:", result)
}
```

## Available Builtins

### eav-get-value

Read a single field value from a record.

```lisp
(eav-get-value "form-slug" record-id "field-machine-name")
```

Returns the value as appropriate Filo type (number, string, bool), or `#f` if not found.

### eav-find-one

Find the first record matching a field condition.

```lisp
(eav-find-one "form-slug" "field-machine-name" "=" value)
```

Operators: `=`, `!=`, `<`, `<=`, `>`, `>=`

Returns record ID as number, or `#f` if not found.

### eav-sum, eav-count, eav-min, eav-max, eav-avg

Aggregate functions with optional filters.

```lisp
;; Sum all amounts
(eav-sum "orders" "amount" (list))

;; Sum only electronics
(eav-sum "sales" "amount" 
  (list (values "category" "=" "electronics")))

;; Count all records (empty string for field)
(eav-count "items" "" (list))
```



## EAVStore Interface

Implementations must provide:

```go
type EAVStore interface {
    GetFieldValue(ctx context.Context, workspaceID int64, formSlug, fieldMachineName string, recordID int64) (any, error)
    FindRecordByField(ctx context.Context, workspaceID int64, formSlug, fieldMachineName, operator string, value any) (int64, error)
    AggregateField(ctx context.Context, workspaceID int64, formSlug, fieldMachineName, aggFunc string, filters []FieldFilter) (float64, error)
}
```

The `db.SQLite` type implements this interface.

## Testing

Integration tests are in `db/db_filoeav_test.go` (placed there to avoid import cycles with the shared test infrastructure).

Run tests:

```bash
go test -v -run 'TestFiloEAV' ./db/...
```

## Design Notes

1. **No Filo core changes** - All integration is via `RegisterBuiltin`
2. **Interface-based** - `EAVStore` allows different implementations
3. **Safe SQL** - All queries use parameterized placeholders
4. **Type conversion** - Automatic Go <-> Filo value conversion
5. **Context-aware** - Operations honor `context.Context` for cancellation
