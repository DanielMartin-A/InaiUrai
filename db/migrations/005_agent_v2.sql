-- Agent v2: audit trail, sessions, confirmations, authorizations, daily cost

CREATE TABLE IF NOT EXISTS agent_audit_trail (
  id BIGSERIAL PRIMARY KEY,
  task_id TEXT NOT NULL,
  step_number INTEGER NOT NULL,
  action_type VARCHAR(50) NOT NULL,
  tool_name VARCHAR(100),
  tool_input JSONB,
  tool_output JSONB,
  tokens_used INTEGER NOT NULL DEFAULT 0,
  blocked_by VARCHAR(100),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_audit_task ON agent_audit_trail(task_id, step_number);

CREATE TABLE IF NOT EXISTS agent_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
  task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
  model VARCHAR(50),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_task ON agent_sessions(task_id);

CREATE TABLE IF NOT EXISTS customer_action_authorizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  action_key VARCHAR(100) NOT NULL,
  is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (customer_id, action_key)
);

DROP TRIGGER IF EXISTS customer_action_authorizations_updated_at ON customer_action_authorizations;
CREATE TRIGGER customer_action_authorizations_updated_at
  BEFORE UPDATE ON customer_action_authorizations
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE IF NOT EXISTS confirmation_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
  task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
  action_type VARCHAR(50) NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_confirmation_requests_status ON confirmation_requests(status);

CREATE TABLE IF NOT EXISTS daily_cost_tracking (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID,
  tracking_date DATE NOT NULL DEFAULT (CURRENT_DATE),
  estimated_cost_cents INTEGER NOT NULL DEFAULT 0,
  total_tokens BIGINT NOT NULL DEFAULT 0,
  task_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (customer_id, tracking_date)
);

DROP TRIGGER IF EXISTS daily_cost_tracking_updated_at ON daily_cost_tracking;
CREATE TRIGGER daily_cost_tracking_updated_at
  BEFORE UPDATE ON daily_cost_tracking
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();
