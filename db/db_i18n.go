package db

import "fmt"

// I18nOverride is one user adjustment to a UI translation.
type I18nOverride struct {
	Locale      string
	MsgKey      string
	Translation string
}

// ListI18nOverrides returns every stored translation adjustment.
func (s *SQLite) ListI18nOverrides() ([]I18nOverride, error) {
	const q = `SELECT
        locale,      -- 1
        msg_key,     -- 2
        translation  -- 3
    FROM i18n_overrides
    ORDER BY locale, msg_key`

	rows, err := s.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list i18n overrides: %w", err)
	}
	defer rows.Close()

	var out []I18nOverride
	for rows.Next() {
		var o I18nOverride
		err := rows.Scan(
			&o.Locale,      // 1
			&o.MsgKey,      // 2
			&o.Translation, // 3
		)
		if err != nil {
			return nil, fmt.Errorf("scan i18n override: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// UpsertI18nOverride stores or updates one translation adjustment.
func (s *SQLite) UpsertI18nOverride(locale, msgKey, translation string) error {
	const q = `INSERT INTO i18n_overrides (
        locale,      -- 1
        msg_key,     -- 2
        translation  -- 3
    ) VALUES (
        ?, -- 1
        ?, -- 2
        ?  -- 3
    )
    ON CONFLICT (locale, msg_key) DO UPDATE SET
        translation = excluded.translation,
        updated_at = CURRENT_TIMESTAMP`
	return s.Exec(
		q,
		locale,      // 1
		msgKey,      // 2
		translation, // 3
	)
}

// DeleteI18nOverride removes one translation adjustment.
func (s *SQLite) DeleteI18nOverride(locale, msgKey string) error {
	const q = `DELETE FROM i18n_overrides
    WHERE locale = ?  -- 1
    AND msg_key = ?   -- 2`
	return s.Exec(
		q,
		locale, // 1
		msgKey, // 2
	)
}

// ContentTranslation is one translated field of a user-created object.
type ContentTranslation struct {
	Locale string
	RefID  string
	Field  string
	Text   string
}

// ListContentTranslations returns every stored content translation.
func (s *SQLite) ListContentTranslations() ([]ContentTranslation, error) {
	const q = `SELECT
        locale, -- 1
        ref_id, -- 2
        field,  -- 3
        text    -- 4
    FROM content_translations
    ORDER BY locale, ref_id, field`

	rows, err := s.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list content translations: %w", err)
	}
	defer rows.Close()

	var out []ContentTranslation
	for rows.Next() {
		var c ContentTranslation
		err := rows.Scan(
			&c.Locale, // 1
			&c.RefID,  // 2
			&c.Field,  // 3
			&c.Text,   // 4
		)
		if err != nil {
			return nil, fmt.Errorf("scan content translation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertContentTranslation stores or updates one content translation.
func (s *SQLite) UpsertContentTranslation(locale, refID, field, text string) error {
	const q = `INSERT INTO content_translations (
        locale, -- 1
        ref_id, -- 2
        field,  -- 3
        text    -- 4
    ) VALUES (
        ?, -- 1
        ?, -- 2
        ?, -- 3
        ?  -- 4
    )
    ON CONFLICT (locale, ref_id, field) DO UPDATE SET
        text = excluded.text,
        updated_at = CURRENT_TIMESTAMP`
	return s.Exec(
		q,
		locale, // 1
		refID,  // 2
		field,  // 3
		text,   // 4
	)
}

// DeleteContentTranslation removes one content translation.
func (s *SQLite) DeleteContentTranslation(locale, refID, field string) error {
	const q = `DELETE FROM content_translations
    WHERE locale = ? -- 1
    AND ref_id = ?   -- 2
    AND field = ?    -- 3`
	return s.Exec(
		q,
		locale, // 1
		refID,  // 2
		field,  // 3
	)
}
