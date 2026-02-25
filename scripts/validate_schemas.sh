#!/usr/bin/env bash
set -euo pipefail

###############################################################################
#  validate_schemas.sh — Verify structural invariants of schemas/*.json
#
#  For each schema file, checks:
#    1. Valid JSON
#    2. output_schema exists and uses "oneOf"
#    3. Every oneOf variant has a "status" field
#    4. At least one "error" variant exists with required "error" sub-object
#
#  Requirements: python3 (no pip packages needed — uses stdlib json/pathlib)
###############################################################################

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

exec python3 - "$REPO_ROOT/schemas" <<'PYEOF'
import json
import sys
from pathlib import Path

schema_dir = Path(sys.argv[1])
files = sorted(schema_dir.glob("*.json"))

if not files:
    print(f"ERROR: No JSON files found in {schema_dir}", file=sys.stderr)
    sys.exit(1)

pass_count = 0
fail_count = 0

def ok(msg):
    global pass_count
    print(f"  \033[32m✓\033[0m {msg}")
    pass_count += 1

def fail(msg):
    global fail_count
    print(f"  \033[31m✗\033[0m {msg}")
    fail_count += 1

for path in files:
    print(f"\n\033[1m{path.name}\033[0m")

    # 1. Valid JSON
    try:
        data = json.loads(path.read_text())
        ok("Valid JSON")
    except json.JSONDecodeError as e:
        fail(f"Invalid JSON: {e}")
        continue

    props = data.get("properties", {})

    # 2. output_schema with oneOf
    output_schema = props.get("output_schema")
    if output_schema is None:
        fail("Missing properties.output_schema")
        continue
    ok("output_schema exists")

    one_of = output_schema.get("oneOf")
    if not isinstance(one_of, list) or len(one_of) == 0:
        fail("output_schema missing 'oneOf' array")
        continue
    ok(f"oneOf has {len(one_of)} variant(s)")

    # 3. Every variant has a "status" enum in its properties
    all_have_status = True
    status_values = []
    for i, variant in enumerate(one_of):
        variant_props = variant.get("properties", {})
        status_prop = variant_props.get("status")
        if status_prop is None:
            fail(f"oneOf[{i}] missing 'status' property")
            all_have_status = False
        else:
            enums = status_prop.get("enum", [])
            status_values.extend(enums)
    if all_have_status:
        ok(f"All variants have 'status' field (values: {status_values})")

    # 4. At least one "error" variant with an "error" sub-object
    error_variants = [
        v for v in one_of
        if "error" in v.get("properties", {}).get("status", {}).get("enum", [])
    ]
    if error_variants:
        err_v = error_variants[0]
        err_obj = err_v.get("properties", {}).get("error")
        if err_obj and "code" in err_obj.get("properties", {}) and "message" in err_obj.get("properties", {}):
            ok("Error variant has 'error' object with code + message")
        else:
            fail("Error variant missing 'error.code' or 'error.message'")
    else:
        fail("No oneOf variant with status='error'")

    # 5. input_schema exists
    input_schema = props.get("input_schema")
    if input_schema is None:
        fail("Missing properties.input_schema")
    else:
        ok("input_schema exists")

# Summary
print()
total = pass_count + fail_count
if fail_count == 0:
    print(f"\033[32mAll {total} checks passed across {len(files)} schema(s).\033[0m")
else:
    print(f"\033[31m{fail_count} of {total} checks failed.\033[0m")
    sys.exit(1)
PYEOF
