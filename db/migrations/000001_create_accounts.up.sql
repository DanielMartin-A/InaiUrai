-- 000001_create_accounts.up.sql
CREATE TYPE account_type AS ENUM ('HUMAN', 'ORGANIZATION', 'AGENT_SERVICE');
CREATE TYPE account_role AS ENUM ('requester', 'provider', 'admin');
CREATE TYPE agent_status AS ENUM ('DRAFT', 'ACTIVE', 'SUSPENDED');
CREATE TYPE job_status AS ENUM ('OPEN', 'ASSIGNED', 'EXECUTING', 'SETTLED', 'FAILED');
CREATE TYPE tx_type AS ENUM ('DEPOSIT', 'WITHDRAWAL', 'ESCROW_HOLD', 'ESCROW_RELEASE', 'ESCROW_REFUND', 'COMMISSION');
CREATE TYPE escrow_status AS ENUM ('HELD', 'RELEASED', 'REFUNDED');

CREATE TABLE accounts (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_type      account_type NOT NULL,
  email             VARCHAR(255) UNIQUE NOT NULL,
  password_hash     VARCHAR(255) NOT NULL,
  display_name      VARCHAR(100) NOT NULL,
  role              account_role NOT NULL DEFAULT 'requester',
  balance_cents     BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
  hold_cents        BIGINT NOT NULL DEFAULT 0 CHECK (hold_cents >= 0),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed System Accounts for Escrow & Commission
INSERT INTO accounts (id, account_type, email, password_hash, display_name, role) VALUES
  ('00000000-0000-0000-0000-000000000001', 'ORGANIZATION', 'escrow@system.local', 'disabled', 'Platform Escrow', 'admin'),
  ('00000000-0000-0000-0000-000000000002', 'ORGANIZATION', 'commission@system.local', 'disabled', 'Platform Commission', 'admin');
