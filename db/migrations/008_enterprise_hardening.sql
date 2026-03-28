-- Enterprise hardening: Row-Level Security, encryption columns, data retention,
-- org deletion function.

-- ============================================================================
-- 1. ROW-LEVEL SECURITY (RLS) — Prevent cross-tenant data access
-- ============================================================================

-- RLS is enabled on all tenant tables but NOT forced on the table owner.
-- The backend connects as the DB owner (inaiurai), which bypasses RLS by default.
-- When a dedicated app role (e.g. inaiurai_app) is introduced, add FORCE ROW LEVEL
-- SECURITY and ensure every query sets app.current_org_id via WithOrgScope.

ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS org_isolation ON organizations
  USING (id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE members ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS member_org_isolation ON members
  USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE org_context ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS context_org_isolation ON org_context
  USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE member_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS profiles_org_isolation ON member_profiles
  USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE engagements ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS engagements_org_isolation ON engagements
  USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS tasks_org_isolation ON tasks
  USING (org_id IS NULL OR org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE agent_audit_trail ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS audit_org_isolation ON agent_audit_trail
  USING (org_id IS NULL OR org_id = current_setting('app.current_org_id', true)::uuid);

-- ============================================================================
-- 2. ENCRYPTION COLUMNS — for sensitive data at rest
-- ============================================================================

ALTER TABLE org_context ADD COLUMN IF NOT EXISTS encrypted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE org_context ADD COLUMN IF NOT EXISTS encryption_key_id VARCHAR(64);

ALTER TABLE member_profiles ADD COLUMN IF NOT EXISTS encrypted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE member_profiles ADD COLUMN IF NOT EXISTS encryption_key_id VARCHAR(64);

-- ============================================================================
-- 3. DATA RETENTION — audit trail TTL + cleanup
-- ============================================================================

ALTER TABLE agent_audit_trail ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE OR REPLACE FUNCTION set_audit_expiry()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.expires_at IS NULL THEN
    NEW.expires_at := NOW() + INTERVAL '90 days';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_set_expiry ON agent_audit_trail;
CREATE TRIGGER audit_set_expiry BEFORE INSERT ON agent_audit_trail
  FOR EACH ROW EXECUTE FUNCTION set_audit_expiry();

CREATE OR REPLACE FUNCTION cleanup_expired_audit()
RETURNS INTEGER AS $$
DECLARE
  deleted_count INTEGER;
BEGIN
  DELETE FROM agent_audit_trail WHERE expires_at < NOW();
  GET DIAGNOSTICS deleted_count = ROW_COUNT;
  RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 4. ORG DATA DELETION — GDPR Article 17 / CCPA
-- ============================================================================

CREATE OR REPLACE FUNCTION delete_org_data(target_org_id UUID)
RETURNS TABLE(table_name TEXT, deleted_count BIGINT) AS $$
BEGIN
  DELETE FROM agent_audit_trail WHERE org_id = target_org_id;
  table_name := 'agent_audit_trail'; deleted_count := (SELECT count(*) FROM agent_audit_trail WHERE org_id = target_org_id);
  RETURN NEXT;

  DELETE FROM agent_sessions WHERE org_id = target_org_id;
  table_name := 'agent_sessions'; deleted_count := 0;
  RETURN NEXT;

  BEGIN
    DELETE FROM daily_cost_tracking WHERE customer_id IN
      (SELECT id FROM members WHERE org_id = target_org_id);
    table_name := 'daily_cost_tracking'; deleted_count := 0;
    RETURN NEXT;
  EXCEPTION WHEN undefined_column THEN NULL;
  END;

  DELETE FROM organizations WHERE id = target_org_id;
  table_name := 'organizations'; deleted_count := 1;
  RETURN NEXT;
END;
$$ LANGUAGE plpgsql;
