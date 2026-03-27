-- Role catalog + per-customer role prefs; link tasks to AI role slugs

CREATE TABLE IF NOT EXISTS role_catalog (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(64) NOT NULL UNIQUE,
  title VARCHAR(255) NOT NULL,
  division VARCHAR(100),
  description TEXT,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS customer_roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  role_slug VARCHAR(64) NOT NULL REFERENCES role_catalog(slug),
  preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (customer_id, role_slug)
);

CREATE INDEX IF NOT EXISTS idx_customer_roles_customer ON customer_roles(customer_id);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS role_slug VARCHAR(64);

-- Seed 16 executive roles (idempotent)
INSERT INTO role_catalog (slug, title, division) VALUES
  ('chief-of-staff', 'Chief of Staff', 'Operations & Execution'),
  ('coo', 'Chief Operating Officer', 'Operations & Execution'),
  ('cpo', 'Chief People Officer', 'Operations & Execution'),
  ('cmo', 'Chief Marketing Officer', 'Growth & Revenue'),
  ('cro', 'Chief Revenue Officer', 'Growth & Revenue'),
  ('cbo', 'Chief Brand Officer', 'Growth & Revenue'),
  ('cfo', 'Chief Financial Officer', 'Finance & Intelligence'),
  ('cio', 'Chief Intelligence Officer', 'Finance & Intelligence'),
  ('researcher', 'Chief Research Officer', 'Finance & Intelligence'),
  ('cco', 'Chief Communications Officer', 'Communications & Content'),
  ('content-chief', 'Chief Content Officer', 'Communications & Content'),
  ('creative-chief', 'Chief Creative Officer', 'Communications & Content'),
  ('general-counsel', 'General Counsel', 'Legal & Technical'),
  ('cto', 'Chief Technology Officer', 'Legal & Technical'),
  ('cdo', 'Chief Data Officer', 'Legal & Technical'),
  ('product-chief', 'Chief Product Officer', 'Legal & Technical')
ON CONFLICT (slug) DO NOTHING;
