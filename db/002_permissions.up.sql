-- Multitenancy and permission model (tenants, groups, permits)

CREATE TABLE tenants (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  reference_id TEXT
);

CREATE TABLE tenant_members (
  id INTEGER PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(tenant_id, user_id)
);

CREATE TABLE groups (
  id INTEGER PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  UNIQUE(tenant_id, name) -- group names unique within a tenant
);

CREATE INDEX ix_groups_by_tenant ON groups(tenant_id);

CREATE TABLE group_members (
  id INTEGER PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  group_id  INTEGER NOT NULL REFERENCES groups(id)  ON DELETE CASCADE,
  user_id   INTEGER NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(tenant_id, group_id, user_id)
);

CREATE INDEX ix_group_members_by_user  ON group_members(tenant_id, user_id);
CREATE INDEX ix_group_members_by_group ON group_members(tenant_id, group_id);

CREATE TABLE permits (
  id INTEGER PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id  INTEGER REFERENCES users(id)  ON DELETE CASCADE,
  group_id INTEGER REFERENCES groups(id) ON DELETE CASCADE,
  resource TEXT NOT NULL,    -- e.g., 'mesa.participar', 'mesa.convidar'
  scope    TEXT,             -- NULL = global; convention: 'type:id' (e.g., 'mesa:t1')
  allowed  INTEGER NOT NULL DEFAULT 0 CHECK (allowed IN (0,1)), -- 1=true (allow), 0=false (deny/disabled)
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK ( (user_id IS NOT NULL) <> (group_id IS NOT NULL) ) -- XOR: only one subject
);

CREATE UNIQUE INDEX ux_permits_user
  ON permits(tenant_id, user_id,  resource, scope)
  WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX ux_permits_group
  ON permits(tenant_id, group_id, resource, scope)
  WHERE group_id IS NOT NULL;
CREATE INDEX ix_permits_lookup_user
  ON permits(tenant_id, resource, scope, user_id);
CREATE INDEX ix_permits_lookup_group
  ON permits(tenant_id, resource, scope, group_id);
CREATE INDEX ix_permits_scope_null
  ON permits(tenant_id, resource)
  WHERE scope IS NULL;
CREATE INDEX ix_permits_by_user  ON permits(user_id);
CREATE INDEX ix_permits_by_group ON permits(group_id);

CREATE TABLE resources (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER REFERENCES resources(id) ON DELETE SET NULL, -- hierarchy for UI grouping
  resource TEXT NOT NULL UNIQUE,  -- e.g., 'mesa.participar', 'mesa.convidar'
  label TEXT NOT NULL,            -- e.g., 'Participar de mesa', 'Convidar para mesa'
  scope_required INTEGER NOT NULL DEFAULT 0
    CHECK (scope_required IN (0,1)), -- 0 = optional, 1 = required (UI hint)
  description TEXT
);

CREATE INDEX ix_resources_parent ON resources(parent_id);

CREATE VIEW effective_permits AS
    -- Direct user permits
    SELECT
        p.tenant_id  AS tenant_id,
        p.user_id    AS user_id,
        p.resource   AS resource,
        p.scope      AS scope,
        p.allowed    AS allowed,
        p.created_at AS created_at
    FROM permits p
    WHERE p.user_id IS NOT NULL

    UNION ALL

    -- Group-derived permits (same tenant)
    SELECT
        p.tenant_id  AS tenant_id,
        gm.user_id   AS user_id,
        p.resource   AS resource,
        p.scope      AS scope,
        p.allowed    AS allowed,
        p.created_at AS created_at
    FROM permits p
    JOIN group_members gm
      ON gm.tenant_id = p.tenant_id
     AND gm.group_id  = p.group_id;
