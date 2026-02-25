#!/usr/bin/env bash
set -euo pipefail

###############################################################################
#  perf_test.sh — Performance baseline for Inaiurai API
#
#  Stress-tests GET /v1/capabilities using `hey` (preferred) or `ab`.
#  Target: p99 < 50ms at 50 concurrent requests.
#
#  Prerequisites:
#    - Backend running on localhost:8080
#    - `hey` installed (go install github.com/rakyll/hey@latest) OR `ab`
#
#  Exit 0 = target met, Exit 1 = target missed or tool unavailable
###############################################################################

BASE="${API_BASE:-http://localhost:8080}"
ENDPOINT="$BASE/v1/capabilities"
CONCURRENCY=50
TOTAL_REQUESTS=1000
TARGET_P99_MS=50

green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
bold()  { printf "\033[1m%s\033[0m\n" "$*"; }
dim()   { printf "\033[2m%s\033[0m\n" "$*"; }

pass() { green "  ✓ $1"; }
fail() { red   "  ✗ $1"; }

bold "═══════════════════════════════════════════════════════════════════"
bold "  INAIURAI Performance Baseline"
bold "═══════════════════════════════════════════════════════════════════"
echo ""
dim "  Endpoint:     $ENDPOINT"
dim "  Concurrency:  $CONCURRENCY"
dim "  Requests:     $TOTAL_REQUESTS"
dim "  Target:       p99 < ${TARGET_P99_MS}ms"
echo ""

# Verify endpoint is reachable
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ENDPOINT" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "000" ]; then
  fail "Cannot reach $ENDPOINT — is the backend running?"
  exit 1
fi
pass "Endpoint reachable (HTTP $HTTP_CODE)"

###############################################################################
# Run with `hey` (preferred)
###############################################################################
if command -v hey &>/dev/null; then
  bold ""
  bold "Running benchmark with hey..."
  echo ""

  OUTPUT=$(hey -n "$TOTAL_REQUESTS" -c "$CONCURRENCY" "$ENDPOINT" 2>&1)
  echo "$OUTPUT"

  # Extract p99 latency from hey output (line like "  99%  in X.XXXX secs")
  P99_SECS=$(echo "$OUTPUT" | grep '99%' | head -1 | awk '{print $NF}' | sed 's/secs//')

  if [ -z "$P99_SECS" ]; then
    fail "Could not parse p99 from hey output"
    exit 1
  fi

  # Convert to milliseconds (awk handles float math)
  P99_MS=$(awk "BEGIN { printf \"%.2f\", $P99_SECS * 1000 }")

  echo ""
  bold "Results"
  dim "  p99 latency: ${P99_MS}ms (target: <${TARGET_P99_MS}ms)"

  PASS_CHECK=$(awk "BEGIN { print ($P99_MS < $TARGET_P99_MS) ? 1 : 0 }")
  if [ "$PASS_CHECK" -eq 1 ]; then
    pass "p99 ${P99_MS}ms < ${TARGET_P99_MS}ms target — PASSED"
    exit 0
  else
    fail "p99 ${P99_MS}ms >= ${TARGET_P99_MS}ms target — FAILED"
    exit 1
  fi
fi

###############################################################################
# Fallback: `ab` (Apache Bench)
###############################################################################
if command -v ab &>/dev/null; then
  bold ""
  bold "Running benchmark with ab (hey not found)..."
  echo ""

  OUTPUT=$(ab -n "$TOTAL_REQUESTS" -c "$CONCURRENCY" "$ENDPOINT/" 2>&1)
  echo "$OUTPUT"

  # ab reports "99%" percentile in a line like "  99%    45"
  P99_MS=$(echo "$OUTPUT" | grep '99%' | awk '{print $2}')

  if [ -z "$P99_MS" ]; then
    fail "Could not parse p99 from ab output"
    exit 1
  fi

  echo ""
  bold "Results"
  dim "  p99 latency: ${P99_MS}ms (target: <${TARGET_P99_MS}ms)"

  if [ "$P99_MS" -lt "$TARGET_P99_MS" ] 2>/dev/null; then
    pass "p99 ${P99_MS}ms < ${TARGET_P99_MS}ms target — PASSED"
    exit 0
  else
    fail "p99 ${P99_MS}ms >= ${TARGET_P99_MS}ms target — FAILED"
    exit 1
  fi
fi

###############################################################################
# Fallback: curl-based rough benchmark
###############################################################################
bold ""
bold "Neither hey nor ab found — running curl-based rough benchmark..."
dim "  Install hey for accurate results: go install github.com/rakyll/hey@latest"
echo ""

LATENCIES=()
SAMPLE_SIZE=100
FAILURES=0

for i in $(seq 1 $SAMPLE_SIZE); do
  TIME_MS=$(curl -s -o /dev/null -w "%{time_total}" "$ENDPOINT" | awk '{printf "%.2f", $1 * 1000}')
  LATENCIES+=("$TIME_MS")
  if (( i % 25 == 0 )); then
    dim "  [$i/$SAMPLE_SIZE] last=${TIME_MS}ms"
  fi
done

# Sort and find p99 (index 98 for 100 samples, 0-indexed)
P99_MS=$(printf '%s\n' "${LATENCIES[@]}" | sort -n | awk "NR==$((SAMPLE_SIZE * 99 / 100)){print}")

echo ""
bold "Results (serial requests — not concurrent)"
dim "  p99 latency: ${P99_MS}ms (target: <${TARGET_P99_MS}ms)"
dim "  Note: serial benchmark; true p99 under load may differ."

PASS_CHECK=$(awk "BEGIN { print ($P99_MS < $TARGET_P99_MS) ? 1 : 0 }")
if [ "$PASS_CHECK" -eq 1 ]; then
  pass "p99 ${P99_MS}ms < ${TARGET_P99_MS}ms target — PASSED (serial)"
else
  fail "p99 ${P99_MS}ms >= ${TARGET_P99_MS}ms target — FAILED (serial)"
  dim "  Install hey for concurrent testing: go install github.com/rakyll/hey@latest"
  exit 1
fi
