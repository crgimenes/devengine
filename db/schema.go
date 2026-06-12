package db

// TableInfo represents basic metadata about a database table.
type TableInfo struct {
	Name string // table name
	SQL  string // CREATE TABLE statement
}
