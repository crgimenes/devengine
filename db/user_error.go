package db

// UserError is a message authored by the application user (e.g. a pre_save
// script blocking a save). Unlike infrastructure errors it is MEANT to be
// shown verbatim; callers detect it with errors.As and skip the generic
// "internal error" treatment.
type UserError string

func (e UserError) Error() string { return string(e) }
