-- Cursorrules schema: accounts, agents, tasks, api_keys, credit_ledger
-- Exact columns and types from .cursorrules DATABASE TABLES.
-- Use on a database where these table names do not yet exist (e.g. separate DB or before 000001).

CREATE TABLE accounts (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email               VARCHAR(255) UNIQUE NOT NULL,
  name                VARCHAR(255),
  company             VARCHAR(255),
  password_hash       VARCHAR(255) NOT NULL,
  credit_balance      INT NOT NULL DEFAULT 1000,
  subscription_tier   VARCHAR(100),
  global_max_per_task INT,
  global_max_per_day  INT,
  is_system_account   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_accounts_email ON accounts(email);

CREATE TABLE agents (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id            UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  role                  VARCHAR(50) NOT NULL CHECK (role IN ('requester', 'worker', 'both')),
  endpoint_url          VARCHAR(1000),
  capabilities_offered  JSONB,
  availability          VARCHAR(50) CHECK (availability IN ('online', 'offline')),
  is_verified           BOOLEAN NOT NULL DEFAULT FALSE,
  schema_compliance_rate REAL,
  avg_response_time_ms   INT,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agents_account_id ON agents(account_id);
CREATE INDEX idx_agents_availability ON agents(availability);

CREATE TABLE tasks (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  requester_agent_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  worker_agent_id      UUID REFERENCES agents(id) ON DELETE SET NULL,
  capability_required  VARCHAR(100) NOT NULL,
  input_payload        JSONB NOT NULL,
  output_payload       JSONB,
  output_status       VARCHAR(50) CHECK (output_status IN ('success', 'partial', 'error')),
  status               VARCHAR(50) NOT NULL CHECK (status IN ('created', 'matching', 'dispatched', 'in_progress', 'completed', 'failed')),
  budget               INT NOT NULL,
  actual_cost          INT,
  platform_fee         INT,
  deadline             TIMESTAMPTZ,
  retry_count          INT NOT NULL DEFAULT 0,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_requester_agent_id ON tasks(requester_agent_id);
CREATE INDEX idx_tasks_worker_agent_id ON tasks(worker_agent_id);
CREATE INDEX idx_tasks_status ON tasks(status);

CREATE TABLE api_keys (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  key_hash   VARCHAR(255) NOT NULL,
  key_prefix VARCHAR(50) NOT NULL,
  is_active  BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_api_keys_account_id ON api_keys(account_id);

CREATE TABLE credit_ledger (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id   UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  task_id      UUID REFERENCES tasks(id) ON DELETE SET NULL,
  entry_type   VARCHAR(50) NOT NULL CHECK (entry_type IN ('escrow_lock', 'escrow_release', 'task_earning', 'platform_fee', 'refund')),
  amount       INT NOT NULL,
  balance_after INT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_credit_ledger_account_id ON credit_ledger(account_id);
CREATE INDEX idx_credit_ledger_task_id ON credit_ledger(task_id);
CREATE INDEX idx_credit_ledger_entry_type ON credit_ledger(entry_type);
