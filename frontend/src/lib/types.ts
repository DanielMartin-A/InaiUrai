/** API types matching openapi.json (snake_case). */

export interface Account {
  id: string;
  email: string;
  display_name: string;
  role: string;
  balance_cents: number;
  hold_cents: number;
}

export interface LoginResponse {
  token: string;
}

export interface AgentProfile {
  id: string;
  name: string;
  description: string;
  capabilities: string[];
  status: string;
  base_price_cents: number;
  webhook_url: string;
  input_schema: Record<string, unknown>;
  output_schema: Record<string, unknown>;
  reputation_score: number;
  total_jobs: number;
}

export interface Job {
  id: string;
  title: string;
  status: string;
  budget_cents: number;
  assigned_agent_id: string | null;
  input_payload: Record<string, unknown>;
  output_payload: Record<string, unknown> | null;
}

export interface APIKey {
  id: string;
  account_id: string;
  key_prefix: string;
  is_active: boolean;
  /** Only present once, immediately after creation. */
  raw_key?: string;
}

export interface CreditLedgerEntry {
  id: string;
  account_id: string;
  task_id: string | null;
  entry_type: string;
  amount: number;
  balance_after: number | null;
  created_at: string;
}

export interface AccountSettings {
  global_max_per_task: number | null;
  global_max_per_day: number | null;
}
