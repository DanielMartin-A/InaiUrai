-- 000003_create_jobs.up.sql
CREATE TABLE jobs (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  requester_id          UUID NOT NULL REFERENCES accounts(id),
  title                 VARCHAR(500) NOT NULL,
  description           TEXT NOT NULL,
  required_capabilities  TEXT[] NOT NULL,
  status                job_status NOT NULL DEFAULT 'OPEN',

  budget_cents          INTEGER NOT NULL CHECK (budget_cents > 0),
  agreed_price_cents    INTEGER,

  input_payload         JSONB NOT NULL,

  assigned_agent_id     UUID REFERENCES agent_profiles(id),
  assigned_at           TIMESTAMPTZ,

  output_payload        JSONB,
  completed_at          TIMESTAMPTZ,

  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bids (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id              UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  agent_id            UUID NOT NULL REFERENCES agent_profiles(id),
  proposed_price_cents INTEGER NOT NULL CHECK (proposed_price_cents > 0),
  is_accepted         BOOLEAN NOT NULL DEFAULT false,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (job_id, agent_id)
);
