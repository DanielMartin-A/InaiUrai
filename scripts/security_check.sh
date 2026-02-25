#!/usr/bin/env bash
set -euo pipefail

###############################################################################
#  security_check.sh — Static security audit for the Inaiurai codebase
#
#  Checks:
#    1. Hardcoded secrets (API keys, passwords, tokens in source)
#    2. is_system_account filters in repository queries
#    3. SQL injection vectors (fmt.Sprintf used to build queries)
#
#  Requirements: grep (or ripgrep)
#  Exit 0 = clean, Exit 1 = issues found
###############################################################################

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0
WARN=0

green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
yellow(){ printf "\033[33m%s\033[0m\n" "$*"; }
bold()  { printf "\033[1m%s\033[0m\n" "$*"; }
dim()   { printf "\033[2m%s\033[0m\n" "$*"; }

pass() { green "  ✓ $1"; PASS=$((PASS + 1)); }
fail() { red   "  ✗ $1"; FAIL=$((FAIL + 1)); }
warn() { yellow "  ⚠ $1"; WARN=$((WARN + 1)); }

# Use ripgrep if available, otherwise grep -rnE (extended regex)
if command -v rg &>/dev/null; then
  HAS_RG=true
else
  HAS_RG=false
fi

# Portable grep wrapper: supports --glob when rg is present, ignores it otherwise
search_code() {
  local pattern="$1"; shift
  local search_dir="$1"; shift
  # remaining args: --glob patterns (ignored for plain grep)
  if $HAS_RG; then
    rg --no-heading --line-number "$pattern" "$search_dir" "$@" 2>/dev/null || true
  else
    grep -rnE "$pattern" "$search_dir" \
      --include='*.go' --include='*.py' --include='*.ts' --include='*.tsx' \
      --include='*.js' --include='*.json' --include='*.sh' --include='*.sql' \
      --include='*.yaml' --include='*.yml' --include='*.toml' \
      --exclude-dir=node_modules --exclude-dir=.next --exclude-dir=.git \
      --exclude-dir=vendor --exclude-dir=__pycache__ --exclude-dir=.pytest_cache \
      --exclude-dir=bin --exclude='*.lock' --exclude='package-lock.json' \
      --exclude='go.sum' \
      2>/dev/null || true
  fi
}

bold "═══════════════════════════════════════════════════════════════════"
bold "  INAIURAI Security Check"
bold "═══════════════════════════════════════════════════════════════════"
echo ""

###############################################################################
# 1. HARDCODED SECRETS
###############################################################################
bold "1. Hardcoded Secrets Scan"

SECRET_PATTERNS=(
  'ANTHROPIC_API_KEY[[:space:]]*=[[:space:]]*["'"'"']'
  'OPENAI_API_KEY[[:space:]]*=[[:space:]]*["'"'"']'
  'DATABASE_URL[[:space:]]*=[[:space:]]*["'"'"']postgres.*password'
  'sk-[a-zA-Z0-9]{20,}'
  'AKIA[0-9A-Z]{16}'
  'ghp_[a-zA-Z0-9]{36}'
)

SECRET_LABELS=(
  "Hardcoded ANTHROPIC_API_KEY"
  "Hardcoded OPENAI_API_KEY"
  "Hardcoded DATABASE_URL with password"
  "OpenAI/Anthropic secret key literal (sk-...)"
  "AWS Access Key ID (AKIA...)"
  "GitHub Personal Access Token (ghp_...)"
)

SECRETS_FOUND=0
for i in "${!SECRET_PATTERNS[@]}"; do
  PATTERN="${SECRET_PATTERNS[$i]}"
  LABEL="${SECRET_LABELS[$i]}"

  HITS=$(search_code "$PATTERN" "$REPO_ROOT")

  # Filter out known-safe patterns
  REAL_HITS=""
  if [ -n "$HITS" ]; then
    REAL_HITS=$(echo "$HITS" \
      | grep -v 'os\.Getenv' \
      | grep -v 'os\.environ' \
      | grep -v 'docker-compose' \
      | grep -v '\.env\.example' \
      | grep -v '_test\.go' \
      | grep -v 'conftest\.py' \
      | grep -v 'password_hash' \
      | grep -v 'PasswordHash' \
      | grep -v 'devpassword' \
      | grep -v 'Makefile:' \
      | grep -v 'smoke_test\.sh' \
      | grep -v 'security_check\.sh' \
      | grep -v '\.cursorrules' \
      | grep -v 'ANTHROPIC_API_KEY}' \
      | grep -v '\${ANTHROPIC' \
      || true)
  fi

  if [ -n "$REAL_HITS" ]; then
    fail "$LABEL"
    echo "$REAL_HITS" | head -5 | while IFS= read -r line; do
      dim "    $line"
    done
    SECRETS_FOUND=$((SECRETS_FOUND + 1))
  else
    pass "No $LABEL found"
  fi
done

# Check .env is gitignored
if grep -q '\.env' "$REPO_ROOT/.gitignore" 2>/dev/null; then
  pass ".env is in .gitignore"
else
  fail ".env is NOT in .gitignore"
fi

###############################################################################
# 2. SYSTEM ACCOUNT FILTERS IN REPOSITORIES
###############################################################################
bold ""
bold "2. System Account Isolation"

# agent_repo.go must filter out system accounts in listing/matching queries
AGENT_REPO="$REPO_ROOT/backend/internal/repository/agent_repo.go"
if [ -f "$AGENT_REPO" ]; then
  if grep -q 'is_system_account\s*=\s*FALSE' "$AGENT_REPO"; then
    pass "agent_repo.go: is_system_account = FALSE filter present"
  else
    fail "agent_repo.go: MISSING is_system_account filter in agent queries"
  fi

  # Verify it's used in FindAvailableWorkers
  if grep -A5 'FindAvailableWorkers' "$AGENT_REPO" | grep -q 'listAgentsWhere\|is_system_account'; then
    pass "agent_repo.go: FindAvailableWorkers uses system account filter"
  else
    fail "agent_repo.go: FindAvailableWorkers does NOT filter system accounts"
  fi

  # Verify it's used in List
  if grep -A5 'func.*List(' "$AGENT_REPO" | grep -q 'listAgentsWhere\|is_system_account'; then
    pass "agent_repo.go: List() uses system account filter"
  else
    fail "agent_repo.go: List() does NOT filter system accounts"
  fi
else
  fail "agent_repo.go not found"
fi

# matching.go should rely on repo filtering (no direct DB access)
MATCHING="$REPO_ROOT/backend/internal/services/matching.go"
if [ -f "$MATCHING" ]; then
  if grep -q 'FindAvailableWorkers' "$MATCHING"; then
    pass "matching.go: delegates to repo's FindAvailableWorkers (inherits filter)"
  else
    warn "matching.go: does not call FindAvailableWorkers — verify system account exclusion"
  fi
else
  fail "matching.go not found"
fi

###############################################################################
# 3. SQL INJECTION PROTECTION
###############################################################################
bold ""
bold "3. SQL Injection Protection"

GO_SRC="$REPO_ROOT/backend"

# fmt.Sprintf with SQL keywords — strong injection risk indicator
SQLI_HITS=$(grep -rnE 'fmt\.Sprintf.*(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE)' \
  "$GO_SRC/internal" --include='*.go' --exclude='*_test.go' 2>/dev/null || true)

if [ -z "$SQLI_HITS" ]; then
  pass "No fmt.Sprintf with SQL keywords in production Go code"
else
  fail "Potential SQL injection: fmt.Sprintf used to build queries"
  echo "$SQLI_HITS" | while IFS= read -r line; do
    dim "    $line"
  done
fi

# Verify parameterized queries ($1, $2, etc.) are used
PARAM_COUNT=$(grep -rnE '\$[1-9]' "$GO_SRC/internal/repository" --include='*.go' 2>/dev/null | wc -l)
PARAM_COUNT=$((PARAM_COUNT + 0))
if [ "$PARAM_COUNT" -ge 10 ]; then
  pass "Repository layer uses parameterized queries (\$1/\$2/...) — $PARAM_COUNT references found"
else
  warn "Only $PARAM_COUNT parameterized query references found — verify coverage"
fi

# String concatenation near SQL (excluding safe constant concat like listAgentsWhere)
CONCAT_HITS=$(grep -rnE '"[^"]*((SELECT|INSERT|UPDATE|DELETE))[^"]*"[[:space:]]*\+' \
  "$GO_SRC/internal/repository" --include='*.go' --exclude='*_test.go' 2>/dev/null || true)

if [ -z "$CONCAT_HITS" ]; then
  pass "No unsafe string concatenation in SQL queries (repository layer)"
else
  UNSAFE_CONCAT=$(echo "$CONCAT_HITS" | grep -v 'listAgentsWhere' || true)
  if [ -z "$UNSAFE_CONCAT" ]; then
    pass "Only safe constant concatenation found (listAgentsWhere)"
  else
    fail "Potential unsafe string concatenation in SQL queries"
    echo "$UNSAFE_CONCAT" | while IFS= read -r line; do
      dim "    $line"
    done
  fi
fi

###############################################################################
# Summary
###############################################################################
echo ""
bold "═══════════════════════════════════════════════════════════════════"
TOTAL=$((PASS + FAIL + WARN))
if [ "$FAIL" -eq 0 ] && [ "$WARN" -eq 0 ]; then
  green "  All $TOTAL checks passed. No security issues found."
elif [ "$FAIL" -eq 0 ]; then
  yellow "  $PASS passed, $WARN warning(s). Review warnings above."
else
  red "  $FAIL FAILED, $WARN warning(s), $PASS passed out of $TOTAL checks."
fi
bold "═══════════════════════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then exit 1; fi
exit 0
