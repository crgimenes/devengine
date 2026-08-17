package db

import (
	"errors"
	"fmt"
)

// EAVLookupStorage is the minimal interface LookupEAVEntityTypeAndAttribute
// needs. Both db.Store and filoeav.Storage satisfy it.
type EAVLookupStorage interface {
	GetEAVEntityTypeByMachineName(machineName string) (*EAVEntityType, error)
	ListEAVAttributesByEntityTypeID(entityTypeID int64) ([]EAVAttribute, error)
}

// UnwrapEAVValue picks the right typed column based on the attribute's
// primitive kind and returns a Go value suitable for rendering.
func UnwrapEAVValue(primitive string, v EAVValue) any {
	switch primitive {
	case "BOOL":
		if v.VBool != nil {
			return *v.VBool
		}
	case "INT":
		if v.VInt != nil {
			return *v.VInt
		}
	case "REAL":
		if v.VReal != nil {
			return *v.VReal
		}
	case "TEXT":
		if v.VText != nil {
			return *v.VText
		}
	case "DATETIME":
		if v.VDatetime != nil {
			return *v.VDatetime
		}
	}
	return nil
}

// FormatEAVValue returns a display string for the first value matching attrID,
// falling back to the given string when no value matches.
func FormatEAVValue(kind string, values []EAVValue, attrID int64, fallback string) string {
	for _, v := range values {
		if v.AttributeID != attrID {
			continue
		}
		switch kind {
		case "TEXT":
			if v.VText != nil {
				return *v.VText
			}
		case "INT":
			if v.VInt != nil {
				return fmt.Sprintf("%d", *v.VInt)
			}
		case "REAL":
			if v.VReal != nil {
				return fmt.Sprintf("%g", *v.VReal)
			}
		case "BOOL":
			if v.VBool != nil {
				if *v.VBool {
					return "true"
				}
				return "false"
			}
		case "DATETIME":
			if v.VDatetime != nil {
				return *v.VDatetime
			}
		}
	}
	return fallback
}

// LookupEAVEntityTypeAndAttribute resolves an entity type (by machine_name)
// and an attribute (by machine_name) in one call. Returns nil, nil, nil when
// the entity or attribute does not exist (no error).
func LookupEAVEntityTypeAndAttribute(storage EAVLookupStorage, entityName, attrName string) (*EAVEntityType, *EAVAttribute, error) {
	et, err := storage.GetEAVEntityTypeByMachineName(entityName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if et == nil {
		return nil, nil, nil
	}
	attrs, err := storage.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		return et, nil, err
	}
	for i := range attrs {
		if attrs[i].MachineName == attrName {
			return et, &attrs[i], nil
		}
	}
	return et, nil, nil
}
