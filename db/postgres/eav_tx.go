package postgres

import (
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/utils"
)

// InsertEAVRecord creates an active record (rev 1) inside the transaction
// and returns its ids.
func (t *Transaction) InsertEAVRecord(entityTypeID int64) (int64, string, error) {
	const q = `INSERT INTO eav_records (
        reference_id,   -- 1
        entity_type_id, -- 2
        status,
        rev
    ) VALUES (
        $1, -- 1
        $2, -- 2
        'active',
        1
    )
    RETURNING
        id,           -- 1
        reference_id  -- 2`

	var id int64
	var refID string
	err := t.QueryRow(
		q,
		utils.NewOpaqueID(), // 1
		entityTypeID,        // 2
	).Scan(
		&id,    // 1
		&refID, // 2
	)
	if err != nil {
		return 0, "", err
	}
	return id, refID, nil
}

// BumpEAVRecordRev advances the optimistic-locking revision of a record.
func (t *Transaction) BumpEAVRecordRev(recordID int64) error {
	const q = `UPDATE eav_records
    SET rev = rev + 1,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = $1 -- 1`
	return t.Exec(
		q,
		recordID, // 1
	)
}

// SaveRecordValues inserts or replaces the typed values of a record inside
// the transaction. Values map by attribute machine name; an entry whose
// typed columns all end up NULL (e.g. a cleared optional datetime) deletes
// the stored value instead, keeping the one-value CHECK satisfied. Computed
// values persist like any other: lists and CSV read straight from
// eav_values.
func (t *Transaction) SaveRecordValues(recordID int64, attributes []db.EAVAttribute, values db.EAVRecordValues) error {
	for machineName, rawValue := range values {
		var attr *db.EAVAttribute
		for i := range attributes {
			if attributes[i].MachineName == machineName {
				attr = &attributes[i]
				break
			}
		}
		if attr == nil {
			continue
		}

		var vBool, vInt, vReal, vText, vDatetime any
		switch attr.PrimitiveKind {
		case "BOOL":
			if b, ok := rawValue.(bool); ok {
				vBool = b
			}
		case "INT":
			if i, ok := rawValue.(int64); ok {
				vInt = i
			}
		case "REAL":
			if f, ok := rawValue.(float64); ok {
				vReal = f
			} else if i, ok := rawValue.(int64); ok {
				vReal = float64(i)
			}
		case "DATETIME":
			if s, ok := rawValue.(string); ok && s != "" {
				vDatetime = s
			}
		default: // TEXT
			if s, ok := rawValue.(string); ok {
				vText = s
			}
		}

		if vBool == nil && vInt == nil && vReal == nil && vText == nil && vDatetime == nil {
			const qDel = `DELETE FROM eav_values
            WHERE record_id = $1    -- 1
            AND attribute_id = $2   -- 2`
			err := t.Exec(
				qDel,
				recordID, // 1
				attr.ID,  // 2
			)
			if err != nil {
				return err
			}
			continue
		}

		const qUpsert = `INSERT INTO eav_values (
            record_id,    -- 1
            attribute_id, -- 2
            v_bool,       -- 3
            v_int,        -- 4
            v_real,       -- 5
            v_text,       -- 6
            v_datetime    -- 7
        ) VALUES (
            $1, -- 1
            $2, -- 2
            $3, -- 3
            $4, -- 4
            $5, -- 5
            $6, -- 6
            $7  -- 7
        )
        ON CONFLICT (record_id, attribute_id) DO UPDATE SET
            v_bool = excluded.v_bool,
            v_int = excluded.v_int,
            v_real = excluded.v_real,
            v_text = excluded.v_text,
            v_datetime = excluded.v_datetime,
            updated_at = now()`
		err := t.Exec(
			qUpsert,
			recordID,  // 1
			attr.ID,   // 2
			vBool,     // 3
			vInt,      // 4
			vReal,     // 5
			vText,     // 6
			vDatetime, // 7
		)
		if err != nil {
			return err
		}
	}
	return nil
}
