-- Engagements: universal work unit for task/project/department/company modes

CREATE TABLE IF NOT EXISTS engagements (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  created_by UUID REFERENCES members(id),
  name VARCHAR(255),
  objective TEXT NOT NULL,
  engagement_type VARCHAR(20) NOT NULL DEFAULT 'task'
    CHECK (engagement_type IN ('task','project','department','company')),
  status VARCHAR(20) NOT NULL DEFAULT 'active'
    CHECK (status IN ('planning','active','paused','completed','cancelled')),
  roles JSONB NOT NULL DEFAULT '[]'::jsonb,
  execution_plan JSONB,
  budget_monthly_cents INTEGER,
  budget_spent_cents INTEGER NOT NULL DEFAULT 0,
  heartbeat_config JSONB,
  started_at TIMESTAMPTZ DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_engagements_org ON engagements(org_id, status);
CREATE INDEX IF NOT EXISTS idx_engagements_active ON engagements(org_id)
  WHERE status IN ('active','planning');

DROP TRIGGER IF EXISTS engagements_updated_at ON engagements;
CREATE TRIGGER engagements_updated_at BEFORE UPDATE ON engagements
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Add engagement reference to tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS org_id UUID;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS member_id UUID;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS engagement_id UUID REFERENCES engagements(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS parent_task_id UUID REFERENCES tasks(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS depends_on UUID[];
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS checked_out_by UUID;

CREATE INDEX IF NOT EXISTS idx_tasks_engagement ON tasks(engagement_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_org ON tasks(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_checkout ON tasks(status, checked_out_by)
  WHERE status = 'created' AND checked_out_by IS NULL;

-- Add org_id to audit trail
ALTER TABLE agent_audit_trail ADD COLUMN IF NOT EXISTS org_id UUID;
ALTER TABLE agent_sessions ADD COLUMN IF NOT EXISTS org_id UUID;
