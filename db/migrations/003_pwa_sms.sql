-- PWA + SMS channel tables

CREATE TABLE IF NOT EXISTS customer_channels (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  channel VARCHAR(20) NOT NULL,
  external_id VARCHAR(255),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_customer_channels_customer ON customer_channels(customer_id);

DROP TRIGGER IF EXISTS customer_channels_updated_at ON customer_channels;
CREATE TRIGGER customer_channels_updated_at
  BEFORE UPDATE ON customer_channels
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE IF NOT EXISTS sms_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
  direction VARCHAR(10) NOT NULL,
  to_number VARCHAR(32),
  from_number VARCHAR(32),
  body TEXT,
  provider_message_id VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sms_log_customer ON sms_log(customer_id, created_at DESC);
