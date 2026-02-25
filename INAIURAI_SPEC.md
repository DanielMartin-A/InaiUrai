# inaiurai MVP — Autonomous Agent Build Specification

**ROLE & DIRECTIVE:**
You are an autonomous senior software engineer building "inaiurai" — a two-sided marketplace for AI agents. 
Your core directive is **hyper-velocity and extreme simplicity**. We are building a high-performance Modular Monolith. 
DO NOT over-engineer. DO NOT add infrastructure components unless explicitly commanded in this document. 
Read this entire document before writing any code. Execute sequentially.

---

## 1. BANNED TECHNOLOGIES (STRICT CONSTRAINTS)
* **NO Microservices:** All backend code lives in a single Go binary.
* **NO Redis:** Use PostgreSQL for state, or Go in-memory constructs for simple rate-limiting.
* **NO NATS/Kafka/RabbitMQ:** Use PostgreSQL `LISTEN/NOTIFY` via the `riverqueue/river` Go library for all background jobs.
* **NO Rust/Firecracker:** Execution relies on Go outbound HTTP webhooks to third-party sandbox APIs or remote agent servers.
* **NO API Gateway (Kong):** The Next.js frontend will route directly to the Go backend.

---

## 2. TECH STACK (EXACT VERSIONS)
* **Backend:** Go 1.22+ 
* **Database:** PostgreSQL 16.x (Single source of truth)
* **Migrations:** `golang-migrate/migrate/v4`
* **Go Routing/DB:** `net/http` (Go 1.22+ standard library routing), `jackc/pgx/v5`
* **Background Jobs:** `github.com/riverqueue/river`
* **Frontend:** Next.js 14+ (App Router), TypeScript 5.3+, Tailwind CSS, shadcn/ui, Zustand

---

## 3. MONOREPO STRUCTURE
Create this exact structure. Do not deviate.

inaiurai/
├── backend/
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── common/      (db connection, middleware, logger, standard errors)
│   │   ├── auth/        (handlers, service, repo for users/keys)
│   │   ├── registry/    (handlers, service, repo for agent profiles)
│   │   ├── jobs/        (handlers, service, repo for jobs, bids, matchmaker algorithm)
│   │   ├── ledger/      (service, repo for atomic escrow holds/releases)
│   │   └── execution/   (River queue workers, outbound webhook HTTP clients)
│   ├── go.mod
│   └── Dockerfile
├── frontend/
│   ├── src/app/
│   ├── src/components/
│   ├── package.json
│   └── next.config.js
├── db/migrations/
├── docker-compose.yml
├── Makefile
└── README.md

---

## 4. BUILD ORDER (SEQUENTIAL EXECUTION)
You must complete and verify each step before moving to the next.

**STEP 1: Infrastructure & DB Foundation**
* Create `docker-compose.yml` containing ONLY `postgres:16`.
* Create `Makefile` with `dev-up`, `migrate-up`, `build`, `test`.
* Write SQL migrations for: `accounts`, `agent_profiles`, `jobs`, `bids`, `transactions`, `escrow_holds`.
* *Verification:* `make dev-up` and `make migrate-up` execute successfully.

**STEP 2: Core Backend Foundation & Auth**
* Initialize Go module. Setup `pgxpool` and custom slog JSON logger in `internal/common`.
* Implement `internal/auth`: Registration, Login, JWT generation (Ed25519), and JWT validation middleware.
* *Verification:* Unit tests pass. You can register a user and get a valid JWT.

**STEP 3: Agent Registry**
* Implement `internal/registry`: CRUD for agent profiles.
* Fields: name, description, capabilities, price, webhook_url, input_schema, output_schema.
* *Verification:* A provider account can create an agent profile and set it to ACTIVE.

**STEP 4: Jobs, Ledger, & Matchmaker (The Core Loop)**
* Implement `internal/jobs` and `internal/ledger`.
* Implement the Matchmaker algorithm inside Go: `score = (reputation * 0.35) + (price_fit * 0.25) + (speed * 0.20) + (availability * 0.20)`.
* **CRITICAL REQUIREMENT:** Job assignment and Ledger Escrow Hold MUST happen in a single PostgreSQL `*pgx.Tx` transaction. If the user lacks funds, the transaction rolls back, and the job remains OPEN.
* *Verification:* Requester creates a job. Matchmaker selects an agent. Job status becomes ASSIGNED and funds are deducted from requester's available balance in one atomic transaction.

**STEP 5: Execution via Riverqueue**
* Setup `riverqueue/river` using the existing Postgres connection pool.
* When a job is ASSIGNED (Step 4), enqueue an `ExecuteAgentJob` in the same Postgres transaction.
* Implement the worker in `internal/execution`: Parse the job, make an HTTP POST to the agent's `webhook_url`, wait for response, validate against `output_schema`.
* Complete job, trigger Ledger release transaction (Escrow -> Provider), update job status to SETTLED.
* *Verification:* Worker processes the queue, calls a mock HTTP endpoint, and successfully settles the ledger.

**STEP 6: Next.js Frontend Foundation**
* Initialize Next.js app. Configure Tailwind and setup basic layout (Navbar, Sidebar).
* Implement Auth pages (/login, /register) and connect to Go API. Store JWT in HTTP-only cookies.
* *Verification:* User can log in via UI and see a protected dashboard.

**STEP 7: Frontend Marketplace & Dashboards**
* Implement /marketplace (public agent search).
* Implement /dashboard (Requester job creation and live status).
* Implement /provider (Provider agent management and earnings).
* *Verification:* E2E test via UI: Register -> Create Agent -> Create Job -> See Job Complete.

---

## 5. SECURITY & ARCHITECTURE RULES
* **Database Access:** Handlers parse HTTP. Services contain business logic. Repositories write SQL. NO SQL IN SERVICES OR HANDLERS.
* **Parameterized Queries:** Use `pgx` $1, $2 parameters strictly. No string formatting for SQL.
* **Resource Isolation:** Every database query for updating/fetching a specific resource must include `WHERE account_id = $1` to ensure users only see their own data.
* **Transactions:** Any operation that touches the `transactions` or `escrow_holds` tables alongside another table MUST use a database transaction.

---

## 6. DATABASE SCHEMA (MVP DDL MIGRATIONS)
Create these exact migration files in `db/migrations/`. 

**Migration 000001: Enums & Accounts**
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

**Migration 000002: Agent Profiles**
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
  webhook_url       VARCHAR(1000) NOT NULL, -- MVP: All agents use webhooks
  
  input_schema      JSONB NOT NULL,
  output_schema     JSONB NOT NULL,
  
  reputation_score  NUMERIC(3,2) DEFAULT 0.00,
  total_jobs        INTEGER NOT NULL DEFAULT 0,
  
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agents_capabilities ON agent_profiles USING GIN(capabilities);
CREATE INDEX idx_agents_status ON agent_profiles(status);

**Migration 000003: Jobs & Bids**
-- 000003_create_jobs.up.sql
CREATE TABLE jobs (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  requester_id          UUID NOT NULL REFERENCES accounts(id),
  title                 VARCHAR(500) NOT NULL,
  description           TEXT NOT NULL,
  required_capabilities TEXT[] NOT NULL,
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

**Migration 000004: Ledger & Escrow**
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

**Migration 000005: Background Jobs (Riverqueue)**
-- 000005_create_river_queue.up.sql
-- The AI agent must run the standard River setup migrations here.
-- Instructions for AI: Use the river CLI or Go library to generate and run 
-- the standard `river` PostgreSQL migration for queuing.