-- 000004_create_ledger.up.sql
CREATE TABLE transactions (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tx_type           tx_type NOT NULL,
  job_id            UUID REFERENCES jobs(id),

  debit_account_id  UUID NOT NULL REFERENCES accounts(id),
  credit_account_id UUID NOT NULL REFERENCES accounts(id),
  amount_cents      BIGINT NOT NULL CHECK (amount_cents > 0),

  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE escrow_holds (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id            UUID NOT NULL REFERENCES jobs(id),
  requester_id      UUID NOT NULL REFERENCES accounts(id),
  amount_cents      BIGINT NOT NULL CHECK (amount_cents > 0),
  status            escrow_status NOT NULL DEFAULT 'HELD',
  hold_tx_id        UUID NOT NULL REFERENCES transactions(id),
  release_tx_id     UUID REFERENCES transactions(id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
