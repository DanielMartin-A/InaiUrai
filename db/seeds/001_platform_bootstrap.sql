-- Platform bootstrap seed: System Account, Admin Account, Seed API Key
-- Uses CONSTANTS from .cursorrules. Run after 000006_cursorrules_tables (or equivalent accounts + api_keys).
-- Requires pgcrypto for bcrypt key hashing.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- System Platform Account
INSERT INTO accounts (
  id,
  email,
  name,
  company,
  password_hash,
  credit_balance,
  is_system_account,
  created_at,
  updated_at
) VALUES (
  '00000000-0000-0000-0000-000000000001',
  'system@platform.local',
  'System Platform',
  'Inaiurai',
  'disabled',
  0,
  TRUE,
  now(),
  now()
) ON CONFLICT (id) DO NOTHING;

-- Admin Account
INSERT INTO accounts (
  id,
  email,
  name,
  company,
  password_hash,
  credit_balance,
  is_system_account,
  created_at,
  updated_at
) VALUES (
  '00000000-0000-0000-0000-000000000002',
  'admin@platform.local',
  'Admin',
  'Inaiurai',
  'disabled',
  0,
  TRUE,
  now(),
  now()
) ON CONFLICT (id) DO NOTHING;

-- Seed API Key (bcrypt hashed), owned by Admin Account
-- Plain key: inai_seed_bootstrap_key_do_not_share
INSERT INTO api_keys (
  id,
  account_id,
  key_hash,
  key_prefix,
  is_active
) VALUES (
  '00000000-0000-0000-0000-000000000003',
  '00000000-0000-0000-0000-000000000002',
  crypt('inai_seed_bootstrap_key_do_not_share', gen_salt('bf')),
  'inai_se',
  TRUE
) ON CONFLICT (id) DO NOTHING;
