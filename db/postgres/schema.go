package postgres

import "github.com/crgimenes/devengine/db"

// ListRelationalTables returns all user-created relational tables in the
// current schema. Excludes migrations bookkeeping, engine meta and EAV/Forms
// core tables. PostgreSQL does not keep the original CREATE TABLE text, so
// TableInfo.SQL is empty here.
func (s *Postgres) ListRelationalTables() ([]db.TableInfo, error) {
	const sqlSelect = `SELECT
		table_name                -- 1
	FROM information_schema.tables
	WHERE table_schema = current_schema()
		AND table_type = 'BASE TABLE'
		AND table_name NOT IN (
			'schema_migrations',
			'devengine_meta',
			'eav_entity_types',
			'eav_attributes',
			'eav_records',
			'eav_values',
			'forms',
			'form_blocks',
			'form_fields',
			'form_data_sources'
		)
	ORDER BY table_name ASC;`

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
		); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
