package postgres

import (
	"fmt"

	"github.com/crgimenes/devengine/db"
)

// ListI18nOverrides returns every stored translation adjustment.
func (s *Postgres) ListI18nOverrides() ([]db.I18nOverride, error) {
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

	var out []db.I18nOverride
	for rows.Next() {
		var o db.I18nOverride
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
func (s *Postgres) UpsertI18nOverride(locale, msgKey, translation string) error {
	const q = `INSERT INTO i18n_overrides (
        locale,      -- 1
        msg_key,     -- 2
        translation  -- 3
    ) VALUES (
        $1, -- 1
        $2, -- 2
        $3  -- 3
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
func (s *Postgres) DeleteI18nOverride(locale, msgKey string) error {
	const q = `DELETE FROM i18n_overrides
    WHERE locale = $1  -- 1
    AND msg_key = $2   -- 2`
	return s.Exec(
		q,
		locale, // 1
		msgKey, // 2
	)
}

// ListContentTranslations returns every stored content translation.
func (s *Postgres) ListContentTranslations() ([]db.ContentTranslation, error) {
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

	var out []db.ContentTranslation
	for rows.Next() {
		var c db.ContentTranslation
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
func (s *Postgres) UpsertContentTranslation(locale, refID, field, text string) error {
	const q = `INSERT INTO content_translations (
        locale, -- 1
        ref_id, -- 2
        field,  -- 3
        text    -- 4
    ) VALUES (
        $1, -- 1
        $2, -- 2
        $3, -- 3
        $4  -- 4
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
func (s *Postgres) DeleteContentTranslation(locale, refID, field string) error {
	const q = `DELETE FROM content_translations
    WHERE locale = $1 -- 1
    AND ref_id = $2   -- 2
    AND field = $3    -- 3`
	return s.Exec(
		q,
		locale, // 1
		refID,  // 2
		field,  // 3
	)
}
