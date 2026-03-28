-- Organizations + Members: B2B-ready data model
-- Every existing customer becomes an org of one

CREATE TABLE IF NOT EXISTS organizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL DEFAULT 'My Organization',
  industry VARCHAR(100),
  website VARCHAR(255),
  subscription_plan VARCHAR(20) NOT NULL DEFAULT 'solo'
    CHECK (subscription_plan IN ('free_trial','solo','team','project','department','company')),
  tasks_used_this_month INTEGER NOT NULL DEFAULT 0,
  tasks_limit INTEGER NOT NULL DEFAULT 50,
  members_limit INTEGER NOT NULL DEFAULT 1,
  roles_limit INTEGER NOT NULL DEFAULT 16,
  free_tasks_remaining INTEGER NOT NULL DEFAULT 3,
  stripe_customer_id VARCHAR(255),
  stripe_subscription_id VARCHAR(255),
  onboarded BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS orgs_updated_at ON organizations;
CREATE TRIGGER orgs_updated_at BEFORE UPDATE ON organizations
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE IF NOT EXISTS members (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name VARCHAR(255),
  email VARCHAR(255),
  telegram_user_id BIGINT UNIQUE,
  slack_user_id VARCHAR(100),
  whatsapp_id VARCHAR(30),
  role_in_company VARCHAR(100),
  preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
  active_channel VARCHAR(20) NOT NULL DEFAULT 'telegram',
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  invited_by UUID REFERENCES members(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_members_org ON members(org_id);
CREATE INDEX IF NOT EXISTS idx_members_telegram ON members(telegram_user_id);
CREATE INDEX IF NOT EXISTS idx_members_slack ON members(slack_user_id);
CREATE INDEX IF NOT EXISTS idx_members_whatsapp ON members(whatsapp_id);

DROP TRIGGER IF EXISTS members_updated_at ON members;
CREATE TRIGGER members_updated_at BEFORE UPDATE ON members
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE IF NOT EXISTS org_context (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  context_type VARCHAR(50) NOT NULL
    CHECK (context_type IN ('org_soul','business_profile','known_entities','preferences','document_summary','project_history','custom')),
  content JSONB NOT NULL,
  source VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_org_context_singleton
  ON org_context (org_id, context_type)
  WHERE context_type IN ('org_soul','business_profile','known_entities','preferences');

CREATE INDEX IF NOT EXISTS idx_org_context_org ON org_context(org_id, context_type);

CREATE TABLE IF NOT EXISTS member_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  profile_type VARCHAR(50) NOT NULL
    CHECK (profile_type IN ('preferences','communication_style','interaction_history')),
  content JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_member_profiles ON member_profiles(member_id, profile_type);

DROP TRIGGER IF EXISTS member_profiles_updated_at ON member_profiles;
CREATE TRIGGER member_profiles_updated_at BEFORE UPDATE ON member_profiles
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();
