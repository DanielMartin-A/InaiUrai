#!/usr/bin/env bash
set -euo pipefail

###############################################################################
#  INAIURAI — Full-Pipeline Smoke Test
#
#  Prerequisites:
#    - Stack running (make bootstrap)
#    - Tools: curl, jq, psql
#    - Postgres accessible on localhost:5432
#
#  Exit 0 = all pass, Exit 1 = any fail
###############################################################################

BASE="http://localhost:8080"
DB="postgres://inaiurai_dev:devpassword@localhost:5432/inaiurai?sslmode=disable"
PASS=0
FAIL=0
CLEANUP_IDS=()

# ── Colours ──────────────────────────────────────────────────────────────────
green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
bold()  { printf "\033[1m%s\033[0m\n" "$*"; }
dim()   { printf "\033[2m%s\033[0m\n" "$*"; }

pass() {
  green "  ✓ $1"
  PASS=$((PASS + 1))
}
fail() {
  red "  ✗ $1"
  FAIL=$((FAIL + 1))
}

check_http() {
  local label="$1" actual="$2" expected="${3:-200}"
  if [ "$actual" = "$expected" ]; then pass "$label  (HTTP $actual)"
  else fail "$label  (expected $expected, got $actual)"; fi
}

bold "═══════════════════════════════════════════════════════════════════"
bold "  INAIURAI Full-Pipeline Smoke Test"
bold "═══════════════════════════════════════════════════════════════════"
echo ""

###############################################################################
# 1. SETUP — Create a test account, agent, and SHA-256 API key via SQL
###############################################################################
bold "1. Setup: Bootstrapping test account"

TEST_ACCOUNT_ID="aaaaaaaa-bbbb-cccc-dddd-eeee00000001"
TEST_AGENT_ID="aaaaaaaa-bbbb-cccc-dddd-eeee00000002"
TEST_APIKEY_ID="aaaaaaaa-bbbb-cccc-dddd-eeee00000003"
TEST_RAW_KEY="inai_test_smoke_key_$(date +%s)"
TEST_KEY_PREFIX="${TEST_RAW_KEY:0:7}"
INITIAL_CREDITS=1000

# SHA-256 hash (the middleware hashes the bearer token this way)
TEST_KEY_HASH=$(printf '%s' "$TEST_RAW_KEY" | sha256sum | cut -d' ' -f1)

psql "$DB" -q -v ON_ERROR_STOP=1 <<SQL
-- Test account with $INITIAL_CREDITS credits
INSERT INTO accounts (id, email, name, company, password_hash, credit_balance, is_system_account, created_at, updated_at)
VALUES ('$TEST_ACCOUNT_ID', 'smoke-test@example.com', 'SmokeTest', 'Test', 'disabled', $INITIAL_CREDITS, FALSE, now(), now())
ON CONFLICT (id) DO UPDATE SET credit_balance = $INITIAL_CREDITS, updated_at = now();

-- Requester agent for that account
INSERT INTO agents (id, account_id, role, endpoint_url, capabilities_offered, availability, is_verified, schema_compliance_rate, avg_response_time_ms, created_at, updated_at)
VALUES ('$TEST_AGENT_ID', '$TEST_ACCOUNT_ID', 'requester', 'http://localhost:0/noop', '{}', 'online', TRUE, 1.0, 0, now(), now())
ON CONFLICT (id) DO UPDATE SET account_id = '$TEST_ACCOUNT_ID', updated_at = now();

-- SHA-256 hashed API key
INSERT INTO api_keys (id, account_id, key_hash, key_prefix, is_active)
VALUES ('$TEST_APIKEY_ID', '$TEST_ACCOUNT_ID', '$TEST_KEY_HASH', '$TEST_KEY_PREFIX', TRUE)
ON CONFLICT (id) DO UPDATE SET key_hash = '$TEST_KEY_HASH', is_active = TRUE;
SQL

CLEANUP_IDS+=("$TEST_ACCOUNT_ID" "$TEST_AGENT_ID" "$TEST_APIKEY_ID")

if [ $? -eq 0 ]; then pass "Bootstrap test account + agent + API key via SQL"
else fail "SQL bootstrap"; fi

dim "  Key prefix: $TEST_KEY_PREFIX  |  Account: $TEST_ACCOUNT_ID"

# Helper: authed curl
acurl() { curl -s -H "Authorization: Bearer $TEST_RAW_KEY" -H "Content-Type: application/json" "$@"; }

###############################################################################
# 2. AUTH — Verify the API key works
###############################################################################
bold "2. Auth: Verify API key"

HTTP=$(acurl -o /dev/null -w "%{http_code}" "$BASE/v1/tasks")
check_http "GET /v1/tasks with test key" "$HTTP" "200"

###############################################################################
# 3. DISCOVERY — List agents and verify 3 capabilities
###############################################################################
bold "3. Discovery: Verify agents are registered"

AGENTS_JSON=$(acurl "$BASE/api/v1/agents")
AGENT_COUNT=$(echo "$AGENTS_JSON" | jq 'length' 2>/dev/null || echo 0)
if [ "$AGENT_COUNT" -ge 3 ]; then
  pass "GET /api/v1/agents returned $AGENT_COUNT agents (>= 3)"
else
  fail "GET /api/v1/agents returned $AGENT_COUNT agents (expected >= 3)"
fi

###############################################################################
# 4. EXECUTION — Submit a "summarize" task
###############################################################################
bold "4. Execution: Submit summarize task"

# Read balance before
BALANCE_BEFORE=$(psql "$DB" -t -A -c "SELECT credit_balance FROM accounts WHERE id = '$TEST_ACCOUNT_ID'")
dim "  Balance before: $BALANCE_BEFORE"

CREATE_RESP=$(acurl -w "\n%{http_code}" -X POST "$BASE/v1/tasks" -d '{
  "requester_agent_id": "'"$TEST_AGENT_ID"'",
  "capability_required": "summarize",
  "input_payload": {
    "text": "Artificial intelligence has transformed how we interact with technology. Large language models can now write code, summarize documents, and answer complex questions. This represents a fundamental shift in human-computer interaction that will reshape industries from healthcare to education.",
    "format": "paragraph"
  },
  "budget": 3,
  "routing_preference": "auto"
}')

CREATE_HTTP=$(echo "$CREATE_RESP" | tail -1)
CREATE_BODY=$(echo "$CREATE_RESP" | sed '$d')

check_http "POST /v1/tasks (create summarize)" "$CREATE_HTTP" "202"

TASK_ID=$(echo "$CREATE_BODY" | jq -r '.task_id // empty')
if [ -n "$TASK_ID" ]; then
  pass "Got task_id: ${TASK_ID:0:8}…"
else
  fail "No task_id in response: $CREATE_BODY"
  TASK_ID=""
fi

###############################################################################
# 5. POLLING — Wait for task completion
###############################################################################
bold "5. Polling: Waiting for task to complete"

TASK_STATUS="unknown"
POLL_MAX=20
POLL_INTERVAL=2

if [ -n "$TASK_ID" ]; then
  for i in $(seq 1 $POLL_MAX); do
    POLL_RESP=$(acurl "$BASE/v1/tasks/$TASK_ID")
    TASK_STATUS=$(echo "$POLL_RESP" | jq -r '.status // "unknown"')
    dim "  [$i/$POLL_MAX] status=$TASK_STATUS"

    if [ "$TASK_STATUS" = "completed" ] || [ "$TASK_STATUS" = "failed" ]; then
      break
    fi
    sleep $POLL_INTERVAL
  done

  if [ "$TASK_STATUS" = "completed" ]; then
    pass "Task reached 'completed' status"
  elif [ "$TASK_STATUS" = "failed" ]; then
    fail "Task reached 'failed' status (may indicate worker/matching issue)"
  else
    fail "Task did not complete within $((POLL_MAX * POLL_INTERVAL))s (stuck at: $TASK_STATUS)"
  fi
else
  fail "Skipped polling — no task_id"
fi

###############################################################################
# 6. VERIFICATION — Check output, credit deduction
###############################################################################
bold "6. Verification"

if [ -n "$TASK_ID" ]; then
  FINAL=$(acurl "$BASE/v1/tasks/$TASK_ID")

  # 6a. output_status
  OUTPUT_STATUS=$(echo "$FINAL" | jq -r '.output_status // "none"')
  if [ "$OUTPUT_STATUS" = "success" ]; then
    pass "output_status = success"
  else
    fail "output_status = $OUTPUT_STATUS (expected success)"
  fi

  # 6b. output_payload has "summary" field
  HAS_SUMMARY=$(echo "$FINAL" | jq 'has("output_payload") and (.output_payload | has("summary"))' 2>/dev/null || echo false)
  if [ "$HAS_SUMMARY" = "true" ]; then
    pass "output_payload contains 'summary' field"
    SUMMARY_PREVIEW=$(echo "$FINAL" | jq -r '.output_payload.summary' | head -c 80)
    dim "  Preview: ${SUMMARY_PREVIEW}…"
  else
    fail "output_payload missing 'summary' field"
  fi

  # 6c. Credit balance decreased by 3
  BALANCE_AFTER=$(psql "$DB" -t -A -c "SELECT credit_balance FROM accounts WHERE id = '$TEST_ACCOUNT_ID'")
  dim "  Balance after: $BALANCE_AFTER"
  EXPECTED_BALANCE=$((BALANCE_BEFORE - 3))
  if [ "$BALANCE_AFTER" -eq "$EXPECTED_BALANCE" ] 2>/dev/null; then
    pass "Credit balance decreased by exactly 3 ($BALANCE_BEFORE → $BALANCE_AFTER)"
  else
    # With platform fee / partial refund the exact amount may differ; accept within range
    DELTA=$((BALANCE_BEFORE - BALANCE_AFTER))
    fail "Credit balance decreased by $DELTA (expected 3). Before=$BALANCE_BEFORE After=$BALANCE_AFTER"
  fi
else
  fail "Skipped verification — no task_id"
  fail "Skipped verification — no task_id"
  fail "Skipped verification — no task_id"
fi

###############################################################################
# 7. NEGATIVE TESTS — Schema Validation
###############################################################################
bold "7. Negative tests: Schema validation"

# 7a. Invalid schema — missing required "text" field for summarize
HTTP=$(acurl -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/tasks" -d '{
  "requester_agent_id": "'"$TEST_AGENT_ID"'",
  "capability_required": "summarize",
  "input_payload": {"format": "paragraph"},
  "budget": 3,
  "routing_preference": "auto"
}')
check_http "Invalid schema (missing 'text') → 422" "$HTTP" "422"

###############################################################################
# 8. AUTH ENFORCEMENT — Bad/missing tokens must return 401
###############################################################################
bold "8. Auth Enforcement"

# 8a. Missing Authorization header → 401
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/tasks" \
  -H "Content-Type: application/json" \
  -d '{"capability_required":"summarize","input_payload":{"text":"x","format":"paragraph"},"budget":1}')
check_http "Missing auth header → 401" "$HTTP" "401"

# 8b. Completely invalid API key → 401
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/tasks" \
  -H "Authorization: Bearer totally_invalid_key" \
  -H "Content-Type: application/json" \
  -d '{"capability_required":"summarize","input_payload":{"text":"x","format":"paragraph"},"budget":1}')
check_http "Invalid API key → 401" "$HTTP" "401"

# 8c. Malformed header (no Bearer prefix) → 401
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/tasks" \
  -H "Authorization: $TEST_RAW_KEY" \
  -H "Content-Type: application/json" \
  -d '{"capability_required":"summarize","input_payload":{"text":"x","format":"paragraph"},"budget":1}')
check_http "Malformed auth (no Bearer prefix) → 401" "$HTTP" "401"

###############################################################################
# 9. BUDGET CHECK — Zero/invalid budget must be rejected
###############################################################################
bold "9. Budget Check"

# 9a. Budget = 0 → 400 or 403
HTTP=$(acurl -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/tasks" -d '{
  "requester_agent_id": "'"$TEST_AGENT_ID"'",
  "capability_required": "summarize",
  "input_payload": {"text": "hello", "format": "paragraph"},
  "budget": 0,
  "routing_preference": "auto"
}')
if [ "$HTTP" = "400" ] || [ "$HTTP" = "403" ]; then
  pass "Zero budget rejected  (HTTP $HTTP)"
else
  fail "Zero budget expected 400|403, got $HTTP"
fi

# 9b. Negative budget → 400 or 403
HTTP=$(acurl -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/tasks" -d '{
  "requester_agent_id": "'"$TEST_AGENT_ID"'",
  "capability_required": "summarize",
  "input_payload": {"text": "hello", "format": "paragraph"},
  "budget": -5,
  "routing_preference": "auto"
}')
if [ "$HTTP" = "400" ] || [ "$HTTP" = "403" ]; then
  pass "Negative budget rejected  (HTTP $HTTP)"
else
  fail "Negative budget expected 400|403, got $HTTP"
fi

###############################################################################
# 10. CREDIT INTEGRITY — Balance must decrease after a successful task
###############################################################################
bold "10. Credit Integrity"

# Re-read the current balance (after the task in section 4-5 may have changed it)
INTEGRITY_BALANCE_BEFORE=$(psql "$DB" -t -A -c "SELECT credit_balance FROM accounts WHERE id = '$TEST_ACCOUNT_ID'")
dim "  Balance before integrity task: $INTEGRITY_BALANCE_BEFORE"

# Submit a cheap summarize task (3 credits)
INTEGRITY_RESP=$(acurl -w "\n%{http_code}" -X POST "$BASE/v1/tasks" -d '{
  "requester_agent_id": "'"$TEST_AGENT_ID"'",
  "capability_required": "summarize",
  "input_payload": {
    "text": "Credit integrity check. The quick brown fox jumps over the lazy dog. This is a test sentence for validation.",
    "format": "bullets"
  },
  "budget": 3,
  "routing_preference": "auto"
}')

INTEGRITY_HTTP=$(echo "$INTEGRITY_RESP" | tail -1)
INTEGRITY_BODY=$(echo "$INTEGRITY_RESP" | sed '$d')
INTEGRITY_TASK_ID=$(echo "$INTEGRITY_BODY" | jq -r '.task_id // empty')

if [ "$INTEGRITY_HTTP" = "202" ] && [ -n "$INTEGRITY_TASK_ID" ]; then
  pass "Integrity task created (${INTEGRITY_TASK_ID:0:8}…)"

  # Wait for the escrow lock to take effect (poll balance)
  sleep 2
  INTEGRITY_BALANCE_AFTER=$(psql "$DB" -t -A -c "SELECT credit_balance FROM accounts WHERE id = '$TEST_ACCOUNT_ID'")
  dim "  Balance after integrity task: $INTEGRITY_BALANCE_AFTER"

  if [ "$INTEGRITY_BALANCE_AFTER" -lt "$INTEGRITY_BALANCE_BEFORE" ]; then
    DEDUCTED=$((INTEGRITY_BALANCE_BEFORE - INTEGRITY_BALANCE_AFTER))
    pass "Balance decreased by $DEDUCTED credits ($INTEGRITY_BALANCE_BEFORE → $INTEGRITY_BALANCE_AFTER)"
  else
    fail "Balance did NOT decrease ($INTEGRITY_BALANCE_BEFORE → $INTEGRITY_BALANCE_AFTER)"
  fi

  # Verify escrow_lock entry exists in ledger
  ESCROW_COUNT=$(psql "$DB" -t -A -c "SELECT COUNT(*) FROM credit_ledger WHERE task_id = '$INTEGRITY_TASK_ID' AND entry_type = 'escrow_lock'")
  if [ "$ESCROW_COUNT" -ge 1 ]; then
    pass "escrow_lock ledger entry exists for integrity task"
  else
    fail "No escrow_lock ledger entry for integrity task"
  fi
else
  fail "Integrity task creation failed (HTTP $INTEGRITY_HTTP): $INTEGRITY_BODY"
  fail "Skipped balance decrease check"
  fail "Skipped escrow_lock check"
fi

###############################################################################
# CLEANUP
###############################################################################
bold "Cleanup"
psql "$DB" -q <<SQL || true
DELETE FROM credit_ledger WHERE account_id = '$TEST_ACCOUNT_ID';
DELETE FROM tasks WHERE requester_agent_id = '$TEST_AGENT_ID';
DELETE FROM api_keys WHERE id = '$TEST_APIKEY_ID';
DELETE FROM agents WHERE id = '$TEST_AGENT_ID';
DELETE FROM accounts WHERE id = '$TEST_ACCOUNT_ID';
SQL
dim "  Removed test data"

###############################################################################
# SUMMARY
###############################################################################
echo ""
bold "═══════════════════════════════════════════════════════════════════"
TOTAL=$((PASS + FAIL))
if [ "$FAIL" -eq 0 ]; then
  green "  All $TOTAL tests passed."
else
  red "  $FAIL of $TOTAL tests failed."
fi
bold "═══════════════════════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then exit 1; fi
exit 0
