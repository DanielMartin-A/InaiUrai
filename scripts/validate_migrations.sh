#!/usr/bin/env bash
set -euo pipefail

###############################################################################
#  validate_migrations.sh — Spin up a temp Postgres, run migrations + seeds,
#  verify the resulting schema and system accounts.
#
#  Requirements: docker
#
#  Exit 0 = all pass, Exit 1 = any fail
###############################################################################

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

CONTAINER_NAME="inaiurai-migration-test-$$"
PG_USER="inaiurai_test"
PG_PASS="testpassword"
PG_DB="inaiurai_test"
PG_PORT=54399
PG_IMAGE="postgres:16-alpine"

PASS=0
FAIL=0

green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
bold()  { printf "\033[1m%s\033[0m\n" "$*"; }
dim()   { printf "\033[2m%s\033[0m\n" "$*"; }

pass() { green "  ✓ $1"; PASS=$((PASS + 1)); }
fail() { red   "  ✗ $1"; FAIL=$((FAIL + 1)); }

cleanup() {
  dim "  Cleaning up container $CONTAINER_NAME ..."
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

DB_URL="postgres://${PG_USER}:${PG_PASS}@localhost:${PG_PORT}/${PG_DB}?sslmode=disable"

run_psql() {
  docker exec -e PGPASSWORD="$PG_PASS" "$CONTAINER_NAME" \
    psql -U "$PG_USER" -d "$PG_DB" -t -A "$@"
}

bold "═══════════════════════════════════════════════════════════════════"
bold "  INAIURAI Migration Validation"
bold "═══════════════════════════════════════════════════════════════════"
echo ""

###############################################################################
# 1. Start temporary Postgres
###############################################################################
bold "1. Starting temp Postgres container"

docker run -d --name "$CONTAINER_NAME" \
  -e POSTGRES_USER="$PG_USER" \
  -e POSTGRES_PASSWORD="$PG_PASS" \
  -e POSTGRES_DB="$PG_DB" \
  -p "${PG_PORT}:5432" \
  "$PG_IMAGE" >/dev/null

dim "  Container: $CONTAINER_NAME  Port: $PG_PORT"

# Wait for Postgres to be ready
RETRIES=30
for i in $(seq 1 $RETRIES); do
  if docker exec -e PGPASSWORD="$PG_PASS" "$CONTAINER_NAME" \
       pg_isready -U "$PG_USER" -d "$PG_DB" >/dev/null 2>&1; then
    pass "Postgres is ready (attempt $i)"
    break
  fi
  if [ "$i" -eq "$RETRIES" ]; then
    fail "Postgres did not become ready in time"
    exit 1
  fi
  sleep 1
done

###############################################################################
# 2. Run migrations
###############################################################################
bold "2. Running migrations"

MIGRATION_DIR="$REPO_ROOT/db/migrations"
UP_FILES=$(find "$MIGRATION_DIR" -name '*.up.sql' | sort)
FILE_COUNT=$(echo "$UP_FILES" | wc -l | tr -d ' ')
dim "  Found $FILE_COUNT up-migration files"

for f in $UP_FILES; do
  FNAME=$(basename "$f")
  SQL=$(cat "$f")
  docker exec -e PGPASSWORD="$PG_PASS" "$CONTAINER_NAME" \
    psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -c "$SQL" >/dev/null 2>&1
  if [ $? -eq 0 ]; then
    pass "Applied $FNAME"
  else
    fail "Failed to apply $FNAME"
  fi
done

###############################################################################
# 3. Run seeds
###############################################################################
bold "3. Running seeds"

for seed in "$REPO_ROOT"/db/seeds/*.sql; do
  SNAME=$(basename "$seed")
  SQL=$(cat "$seed")
  docker exec -e PGPASSWORD="$PG_PASS" "$CONTAINER_NAME" \
    psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -c "$SQL" >/dev/null 2>&1
  if [ $? -eq 0 ]; then
    pass "Applied seed: $SNAME"
  else
    fail "Failed to apply seed: $SNAME"
  fi
done

###############################################################################
# 4. Verify tables exist
###############################################################################
bold "4. Verifying tables"

EXPECTED_TABLES="accounts agents tasks api_keys credit_ledger"
for tbl in $EXPECTED_TABLES; do
  EXISTS=$(run_psql -c "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '$tbl')")
  if [ "$EXISTS" = "t" ]; then
    pass "Table '$tbl' exists"
  else
    fail "Table '$tbl' missing"
  fi
done

###############################################################################
# 5. Verify system accounts (UUID ...0001 and ...0002)
###############################################################################
bold "5. Verifying system accounts"

SYSTEM_PLATFORM_ID="00000000-0000-0000-0000-000000000001"
ADMIN_ACCOUNT_ID="00000000-0000-0000-0000-000000000002"

PLATFORM_EXISTS=$(run_psql -c "SELECT COUNT(*) FROM accounts WHERE id = '$SYSTEM_PLATFORM_ID'")
if [ "$PLATFORM_EXISTS" -ge 1 ]; then
  pass "SystemPlatformAccount ($SYSTEM_PLATFORM_ID) exists"
else
  fail "SystemPlatformAccount ($SYSTEM_PLATFORM_ID) missing"
fi

PLATFORM_IS_SYSTEM=$(run_psql -c "SELECT is_system_account FROM accounts WHERE id = '$SYSTEM_PLATFORM_ID'" 2>/dev/null || echo "")
if [ "$PLATFORM_IS_SYSTEM" = "t" ]; then
  pass "SystemPlatformAccount has is_system_account = TRUE"
else
  fail "SystemPlatformAccount is_system_account != TRUE (got: $PLATFORM_IS_SYSTEM)"
fi

ADMIN_EXISTS=$(run_psql -c "SELECT COUNT(*) FROM accounts WHERE id = '$ADMIN_ACCOUNT_ID'")
if [ "$ADMIN_EXISTS" -ge 1 ]; then
  pass "AdminAccount ($ADMIN_ACCOUNT_ID) exists"
else
  fail "AdminAccount ($ADMIN_ACCOUNT_ID) missing"
fi

ADMIN_IS_SYSTEM=$(run_psql -c "SELECT is_system_account FROM accounts WHERE id = '$ADMIN_ACCOUNT_ID'" 2>/dev/null || echo "")
if [ "$ADMIN_IS_SYSTEM" = "t" ]; then
  pass "AdminAccount has is_system_account = TRUE"
else
  fail "AdminAccount is_system_account != TRUE (got: $ADMIN_IS_SYSTEM)"
fi

# Verify seed API key exists for Admin
SEED_KEY_EXISTS=$(run_psql -c "SELECT COUNT(*) FROM api_keys WHERE account_id = '$ADMIN_ACCOUNT_ID'")
if [ "$SEED_KEY_EXISTS" -ge 1 ]; then
  pass "Seed API key exists for Admin account"
else
  fail "Seed API key missing for Admin account"
fi

###############################################################################
# 6. Verify down migrations (roundtrip)
###############################################################################
bold "6. Verifying down migrations"

DOWN_FILES=$(find "$MIGRATION_DIR" -name '*.down.sql' | sort -r)
for f in $DOWN_FILES; do
  FNAME=$(basename "$f")
  SQL=$(cat "$f")
  docker exec -e PGPASSWORD="$PG_PASS" "$CONTAINER_NAME" \
    psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -c "$SQL" >/dev/null 2>&1
  if [ $? -eq 0 ]; then
    pass "Rolled back $FNAME"
  else
    fail "Failed to roll back $FNAME"
  fi
done

REMAINING=$(run_psql -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'")
if [ "$REMAINING" -eq 0 ]; then
  pass "All tables removed after down migrations"
else
  fail "$REMAINING table(s) remain after down migrations"
fi

###############################################################################
# Summary
###############################################################################
echo ""
bold "═══════════════════════════════════════════════════════════════════"
TOTAL=$((PASS + FAIL))
if [ "$FAIL" -eq 0 ]; then
  green "  All $TOTAL checks passed."
else
  red "  $FAIL of $TOTAL checks failed."
fi
bold "═══════════════════════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then exit 1; fi
exit 0
