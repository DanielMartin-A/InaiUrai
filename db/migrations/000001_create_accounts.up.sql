-- 000001_create_accounts.up.sql
CREATE TYPE account_type AS ENUM ('HUMAN', 'ORGANIZATION', 'AGENT_SERVICE');
CREATE TYPE account_role AS ENUM ('requester', 'provider', 'admin');
CREATE TYPE agent_status AS ENUM ('DRAFT', 'ACTIVE', 'SUSPENDED');
CREATE TYPE job_status AS ENUM ('OPEN', 'ASSIGNED', 'EXECUTING', 'SETTLED', 'FAILED');
CREATE TYPE tx_type AS ENUM ('DEPOSIT', 'WITHDRAWAL', 'ESCROW_HOLD', 'ESCROW_RELEASE', 'ESCROW_REFUND', 'COMMISSION');
CREATE TYPE escrow_status AS ENUM ('HELD', 'RELEASED', 'REFUNDED');

CREATE TABLE accounts (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_type        account_type NOT NULL DEFAULT 'HUMAN',
  email               VARCHAR(255) UNIQUE NOT NULL,
  password_hash       VARCHAR(255) NOT NULL,
  
  -- Renamed to match Go structs and CLAUDE.md
  name                VARCHAR(100) NOT NULL, 
  company             VARCHAR(100),
  role                account_role NOT NULL DEFAULT 'requester',
  
  -- Renamed balance_cents -> credit_balance (Default 1000 per your specs)
  credit_balance      BIGINT NOT NULL DEFAULT 1000 CHECK (credit_balance >= 0),
  hold_cents          BIGINT NOT NULL DEFAULT 0 CHECK (hold_cents >= 0),
  
  -- Added fields required by your Stripe service and dashboard logic
  subscription_tier   VARCHAR(50) DEFAULT 'free',
  global_max_per_task INTEGER DEFAULT 20,
  global_max_per_day  INTEGER DEFAULT 500,
  global_max_per_month INTEGER DEFAULT 10000,
  is_system_account   BOOLEAN DEFAULT FALSE,
  stripe_customer_id  VARCHAR(255),
  
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed System Accounts for Escrow & Commission
-- Note: 'name' is used here instead of 'display_name', and is_system_account = TRUE
INSERT INTO accounts (id, account_type, email, password_hash, name, role, is_system_account) VALUES
  ('00000000-0000-0000-0000-000000000001', 'ORGANIZATION', 'escrow@system.local', 'disabled', 'Platform Escrow', 'admin', TRUE),
  ('00000000-0000-0000-0000-000000000002', 'ORGANIZATION', 'commission@system.local', 'disabled', 'Platform Commission', 'admin', TRUE)
ON CONFLICT (id) DO NOTHING;