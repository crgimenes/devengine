-- v_eav_cells
-- Generic EAV view exposing cells as a relational stream (PostgreSQL).

CREATE OR REPLACE VIEW v_eav_cells AS
SELECT
    -- Entity type
    et.id            AS entity_type_id,
    et.reference_id  AS entity_ref,
    et.machine_name  AS entity_machine,

    -- Record
    r.id             AS record_id,
    r.reference_id   AS record_ref,
    r.rev            AS record_rev,
    r.created_at     AS record_created_at,
    r.updated_at     AS record_updated_at,

    -- Attribute
    a.id             AS attribute_id,
    a.reference_id   AS attribute_ref,
    a.machine_name   AS attr_machine,
    a.primitive_kind AS attr_kind,

    -- Typed values
    v.v_bool,
    v.v_int,
    v.v_real,
    v.v_text,
    v.v_datetime,

    -- Value metadata
    v.updated_at     AS value_updated_at

FROM eav_records r
JOIN eav_entity_types et
    ON et.id = r.entity_type_id
JOIN eav_values v
    ON v.record_id = r.id
JOIN eav_attributes a
    ON a.id = v.attribute_id

WHERE
    r.deleted_at  IS NULL
    AND et.deleted_at IS NULL
    AND a.deleted_at  IS NULL;
