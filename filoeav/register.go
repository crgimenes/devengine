package filoeav

import "github.com/crgimenes/devengine/filo"

// RegisterEAVBuiltins adds EAV-aware builtins to a Filo engine.
// The store provides data access, and cfg supplies workspace/user context.
//
// Registered builtins:
//   - eav-get-value: Read a field value from a record
//   - eav-find-one: Find first record matching a field condition
//   - eav-sum, eav-count, eav-min, eav-max, eav-avg: Aggregations with optional filters
//   - eav-sum-children, eav-count-children, etc.: Subform aggregations
func RegisterEAVBuiltins(eng *filo.Engine, store EAVStore, cfg Config) {
	// Value retrieval
	eng.RegisterBuiltin("eav-get-value", makeGetValueBuiltin(store, cfg))

	// Record finding
	eng.RegisterBuiltin("eav-find-one", makeFindOneBuiltin(store, cfg))

	// Aggregations
	eng.RegisterBuiltin("eav-sum", makeAggregateBuiltin(store, cfg, "SUM"))
	eng.RegisterBuiltin("eav-count", makeAggregateBuiltin(store, cfg, "COUNT"))
	eng.RegisterBuiltin("eav-min", makeAggregateBuiltin(store, cfg, "MIN"))
	eng.RegisterBuiltin("eav-max", makeAggregateBuiltin(store, cfg, "MAX"))
	eng.RegisterBuiltin("eav-avg", makeAggregateBuiltin(store, cfg, "AVG"))

	// Child/subform aggregations

}
