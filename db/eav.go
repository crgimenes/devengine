package db

import (
	"errors"
	"time"
)

// Sentinel errors for EAV operations.
var (
	// ErrNotFound is returned when an entity, attribute, record, or value is not found.
	ErrNotFound = errors.New("eav: not found")

	// ErrConflict is returned when an optimistic lock fails (rev mismatch).
	ErrConflict = errors.New("eav: conflict")

	// ErrInvalidValue is returned when a value violates type or column rules.
	ErrInvalidValue = errors.New("eav: invalid value")
)

// EAVEntityType represents a logical schema (similar to a table in relational databases).
// This is the new EAV core implementation without workspace or forms concepts.
type EAVEntityType struct {
	ID          int64     `json:"id"`
	ReferenceID string    `json:"reference_id"`
	MachineName string    `json:"machine_name"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PreSave     string    `json:"pre_save"` // Filo script executed before saving records
	PosLoad     string    `json:"pos_load"` // Filo script executed after loading records
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeletedAt   time.Time `json:"deleted_at"` // zero value means not deleted
}

// EAVAttribute represents a typed field (column) belonging to an entity type.
type EAVAttribute struct {
	ID            int64  `json:"id"`
	ReferenceID   string `json:"reference_id"`
	EntityTypeID  int64  `json:"entity_type_id"`
	MachineName   string `json:"machine_name"`
	Label         string `json:"label"`
	HelpText      string `json:"help_text"`
	PrimitiveKind string `json:"primitive_kind"` // BOOL, INT, REAL, TEXT, DATETIME
	IsRequired    bool   `json:"is_required"`
	IsUnique      bool   `json:"is_unique"`
	IsIndexed     bool   `json:"is_indexed"`
	MaxLength     *int   `json:"max_length,omitempty"` // For TEXT fields, NULL for other types
	IsComputed    bool   `json:"is_computed"`
	ComputedExpr  string `json:"computed_expr"` // Filo expression
	// Default values for new records (user-defined)
	DefaultVBool     *bool     `json:"default_v_bool,omitempty"`
	DefaultVInt      *int64    `json:"default_v_int,omitempty"`
	DefaultVReal     *float64  `json:"default_v_real,omitempty"`
	DefaultVText     *string   `json:"default_v_text,omitempty"`
	DefaultVDatetime *string   `json:"default_v_datetime,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	DeletedAt        time.Time `json:"deleted_at"` // zero value means not deleted
}

// EAVRecord represents an instance (row) of an entity type.
type EAVRecord struct {
	ID           int64     `json:"id"`
	ReferenceID  string    `json:"reference_id"`
	EntityTypeID int64     `json:"entity_type_id"`
	Status       string    `json:"status"` // 'draft' or 'active'
	Rev          int       `json:"rev"`    // optimistic lock counter, starts at 1
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DeletedAt    time.Time `json:"deleted_at"` // zero value means not deleted
}

// EAVValue represents a typed cell value for a (record, attribute) pair.
// Exactly one of the v_* fields should be set based on the attribute's primitive_kind.
type EAVValue struct {
	RecordID    int64     `json:"record_id"`
	AttributeID int64     `json:"attribute_id"`
	VBool       *bool     `json:"v_bool,omitempty"`
	VInt        *int64    `json:"v_int,omitempty"`
	VReal       *float64  `json:"v_real,omitempty"`
	VText       *string   `json:"v_text,omitempty"`
	VDatetime   *string   `json:"v_datetime,omitempty"` // ISO-8601 UTC
	UpdatedAt   time.Time `json:"updated_at"`
}

// ====================================================================
// Entity Type Operations
// ====================================================================
