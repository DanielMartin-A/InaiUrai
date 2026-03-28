-- API tokens for REST API v1 authentication (Week 4).
-- Tokens are stored as SHA-256 hashes — the plaintext is shown once at creation
-- and never stored.

CREATE TABLE IF NOT EXISTS api_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  token_hash VARCHAR(64) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL DEFAULT 'default',
  scopes VARCHAR(255) NOT NULL DEFAULT 'read,write',
  revoked BOOLEAN NOT NULL DEFAULT FALSE,
  last_used_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '1 year'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_member ON api_tokens(member_id);
CREATE INDEX IF NOT EXISTS idx_api_tokens_lookup ON api_tokens(token_hash) WHERE revoked = FALSE;

ALTER TABLE api_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY api_tokens_org_isolation ON api_tokens
  USING (org_id = current_setting('app.current_org_id', true)::uuid);
