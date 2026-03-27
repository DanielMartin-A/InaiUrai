#!/bin/bash
set -e
PASS=0; FAIL=0
ENGINE="${ENGINE_URL:-http://localhost:8000}"
pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

echo "=== INAIURAI v5.0 SMOKE TEST ==="

curl -sf "$ENGINE/health" | python3 -c "
import sys,json; d=json.load(sys.stdin)
assert d['status']=='ok'
" && pass "Health (liveness)" || fail "Health"

curl -sf "$ENGINE/health/ready" | python3 -c "
import sys,json; d=json.load(sys.stdin)
assert d['status'] in ('ready','degraded')
assert 'checks' in d
" && pass "Health/ready (readiness)" || fail "Health/ready (expected if backend not running)"

curl -sf "$ENGINE/v1/roles" -H "X-Internal-Key: $INTERNAL_API_KEY" | python3 -c "
import sys,json; d=json.load(sys.stdin); assert len(d['roles'])==16
" && pass "16 roles loaded (/v1/)" || fail "Roles /v1/"

curl -sf "$ENGINE/roles" -H "X-Internal-Key: $INTERNAL_API_KEY" | python3 -c "
import sys,json; d=json.load(sys.stdin); assert len(d['roles'])==16
" && pass "16 roles loaded (unversioned)" || fail "Roles unversioned"

curl -sf -X POST "$ENGINE/detect_pii" -H "Content-Type: application/json" \
  -H "X-Internal-Key: $INTERNAL_API_KEY" \
  -d '{"text":"SSN 123-45-6789"}' | python3 -c "
import sys,json; assert json.load(sys.stdin)['has_pii']==True
" && pass "PII detection" || fail "PII"

if [ -n "$ANTHROPIC_API_KEY" ]; then
  ROUTE=$(curl -sf -X POST "$ENGINE/orchestrate" -H "Content-Type: application/json" \
    -H "X-Internal-Key: $INTERNAL_API_KEY" \
    -d '{"objective":"Research top CRM tools for small agencies"}')
  echo "$ROUTE" | python3 -c "
import sys,json; d=json.load(sys.stdin)
assert d.get('engagement_type') in ('task','project')
assert len(d.get('team',[])) >= 1
" && pass "Orchestrator routing" || fail "Orchestrator: $ROUTE"

  SOLO=$(curl -sf -X POST "$ENGINE/route" -H "Content-Type: application/json" \
    -H "X-Internal-Key: $INTERNAL_API_KEY" \
    -d '{"input_text":"Analyze our competitor pricing strategy","org_soul":""}')
  echo "$SOLO" | python3 -c "
import sys,json; d=json.load(sys.stdin)
assert d.get('role_slug') in ('cio','researcher','cmo','chief-of-staff')
" && pass "Solo auto-routing (/route)" || fail "Route: $SOLO"

  RESULT=$(curl -sf -X POST "$ENGINE/run_task" -H "Content-Type: application/json" \
    -H "X-Internal-Key: $INTERNAL_API_KEY" \
    -d '{"task_id":"test-1","input_text":"What is 2+2?","org_context":{},"org_soul":"","role":"chief-of-staff","tier":"solo"}')
  echo "$RESULT" | python3 -c "
import sys,json; assert json.load(sys.stdin)['status']=='success'
" && pass "Agent loop execution" || fail "Agent loop: $RESULT"
else
  echo "  ⏭ Skipping API tests (ANTHROPIC_API_KEY not set)"
fi

echo ""
echo "=== $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
