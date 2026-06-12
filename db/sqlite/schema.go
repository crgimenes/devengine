package sqlite

import "github.com/crgimenes/devengine/db"

// ListRelationalTables returns all user-created relational tables from the SQLite schema.
// Excludes system tables, migrations, FTS tables, and EAV/Forms core tables.
func (s *SQLite) ListRelationalTables() ([]db.TableInfo, error) {
	const sqlSelect = `SELECT
		name,                     -- 1
		COALESCE(sql, '')         -- 2
	FROM sqlite_master
	WHERE type = 'table'
		AND name NOT LIKE 'sqlite_%'
		AND name != 'schema_migrations'
		AND name NOT LIKE '%_fts'
		AND name NOT IN (
			'eav_entity_types',
			'eav_attributes', 
			'eav_records',
			'eav_values',
			'forms',
			'form_blocks',
			'form_fields',
			'form_data_sources'
		)
	ORDER BY name ASC;`

	rows, err := s.Query(sqlSelect)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []db.TableInfo
	for rows.Next() {
		var t db.TableInfo
		if err := rows.Scan(
			&t.Name, // 1
			&t.SQL,  // 2
		); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
