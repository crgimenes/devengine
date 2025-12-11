// Package filoeav provides EAV-aware builtins for the Filo scripting language.
// It bridges Filo scripts with the EAV data layer through an injectable EAVStore interface,
// allowing scripts to read, query, and aggregate EAV data without coupling Filo core to the database.
//
// Usage:
//
//	store := db.NewEAVStore(dbConn)
//	engine := filo.NewEngine()
//	filoeav.RegisterEAVBuiltins(engine, store, filoeav.Config{WorkspaceID: 1, UserID: 42})
package filoeav

import "context"

// EAVStore abstracts EAV data access for Filo builtins.
// Implementations must be safe for concurrent use.
type EAVStore interface {
	// GetFieldValue retrieves a single field value from a record.
	// Returns the value in appropriate Go type (int64, float64, string, bool, time.Time),
	// or nil if not found. The caller should convert to Filo Value as needed.
	GetFieldValue(ctx context.Context, workspaceID int64, formSlug, fieldMachineName string, recordID int64) (any, error)

	// FindRecordByField finds the first record where field matches value.
	// Operator must be one of: "=", "!=", "<", "<=", ">", ">=".
	// Returns record ID or 0 if not found.
	FindRecordByField(ctx context.Context, workspaceID int64, formSlug, fieldMachineName, operator string, value any) (int64, error)

	// AggregateField performs an aggregation on a field.
	// aggFunc must be one of: "SUM", "COUNT", "MIN", "MAX", "AVG".
	// Filters is a slice of FieldFilter conditions to apply before aggregation.
	AggregateField(ctx context.Context, workspaceID int64, formSlug, fieldMachineName, aggFunc string, filters []FieldFilter) (float64, error)
}

// FieldFilter represents a filter condition for queries.
type FieldFilter struct {
	FieldMachineName string
	Operator         string // "=", "!=", "<", "<=", ">", ">="
	Value            any
}

// Config holds context for EAV builtin execution.
type Config struct {
	WorkspaceID int64 // Workspace scope for all operations
	UserID      int64 // User ID for future permission checks
}
