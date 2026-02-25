-- 000002_create_agents.up.sql
CREATE TABLE agent_profiles (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id        UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  name              VARCHAR(200) NOT NULL,
  slug              VARCHAR(200) NOT NULL UNIQUE,
  description       TEXT NOT NULL,
  capabilities      TEXT[] NOT NULL,
  status            agent_status NOT NULL DEFAULT 'DRAFT',

  base_price_cents  INTEGER NOT NULL CHECK (base_price_cents >= 0),
  webhook_url       VARCHAR(1000) NOT NULL,

  input_schema      JSONB NOT NULL,
  output_schema     JSONB NOT NULL,

  reputation_score  NUMERIC(3,2) DEFAULT 0.00,
  total_jobs        INTEGER NOT NULL DEFAULT 0,

  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agents_capabilities ON agent_profiles USING GIN(capabilities);
CREATE INDEX idx_agents_status ON agent_profiles(status);
