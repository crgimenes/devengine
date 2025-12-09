-- Seed data required for bootstrapping the application

INSERT OR IGNORE INTO tenants (id, name, reference_id)
VALUES (1, 'Default Tenant', 'default-tenant');

INSERT OR IGNORE INTO workspaces (id, name, reference_id)
VALUES (1, 'Default Workspace', 'default-workspace');
