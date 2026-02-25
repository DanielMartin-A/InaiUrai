#!/usr/bin/env python3
"""
Register the 3 seed worker agents with the Go Router.

This script is idempotent: re-running it will upsert (INSERT ... ON CONFLICT DO UPDATE)
so agents are always brought to the desired state.

It talks directly to PostgreSQL because the agents table (models.Agent from .cursorrules)
does not yet have a dedicated HTTP CRUD endpoint on the router. For production you would
use an admin API; for bootstrap this is the simplest path.

Requirements: pip install psycopg2-binary python-dotenv  (or use psycopg[binary])
"""

import json
import os
import sys

try:
    import psycopg2
except ImportError:
    print("psycopg2 not installed. Run: pip install psycopg2-binary", file=sys.stderr)
    sys.exit(1)

from dotenv import load_dotenv

load_dotenv()

DATABASE_URL = os.environ.get(
    "DATABASE_URL",
    "postgres://inaiurai_dev:devpassword@localhost:5432/inaiurai?sslmode=disable",
)

ADMIN_ACCOUNT_ID = "00000000-0000-0000-0000-000000000002"

AGENTS_HOST = os.environ.get("AGENTS_HOST", "http://host.docker.internal")

AGENTS = [
    {
        "id": "00000000-0000-0000-0000-a00000000001",
        "account_id": ADMIN_ACCOUNT_ID,
        "role": "worker",
        "endpoint_url": f"{AGENTS_HOST}:8001/task",
        "capabilities_offered": {"Research": {"price": 8}},
        "availability": "online",
        "is_verified": True,
        "name": "ResearchBot",
        "port": 8001,
    },
    {
        "id": "00000000-0000-0000-0000-a00000000002",
        "account_id": ADMIN_ACCOUNT_ID,
        "role": "worker",
        "endpoint_url": f"{AGENTS_HOST}:8002/task",
        "capabilities_offered": {"Summarize": {"price": 3}},
        "availability": "online",
        "is_verified": True,
        "name": "SumBot",
        "port": 8002,
    },
    {
        "id": "00000000-0000-0000-0000-a00000000003",
        "account_id": ADMIN_ACCOUNT_ID,
        "role": "worker",
        "endpoint_url": f"{AGENTS_HOST}:8003/task",
        "capabilities_offered": {"Data Extraction": {"price": 5}},
        "availability": "online",
        "is_verified": True,
        "name": "ExtractBot",
        "port": 8003,
    },
]

UPSERT_SQL = """
INSERT INTO agents (
    id, account_id, role, endpoint_url, capabilities_offered,
    availability, is_verified, schema_compliance_rate, avg_response_time_ms
) VALUES (
    %(id)s, %(account_id)s, %(role)s, %(endpoint_url)s, %(capabilities_offered)s,
    %(availability)s, %(is_verified)s, 1.0, 0
)
ON CONFLICT (id) DO UPDATE SET
    endpoint_url         = EXCLUDED.endpoint_url,
    capabilities_offered = EXCLUDED.capabilities_offered,
    availability         = EXCLUDED.availability,
    is_verified          = EXCLUDED.is_verified,
    updated_at           = now()
"""


def main() -> None:
    print(f"Connecting to: {DATABASE_URL.split('@')[-1]}")
    conn = psycopg2.connect(DATABASE_URL)
    conn.autocommit = True
    cur = conn.cursor()

    for agent in AGENTS:
        params = {
            "id": agent["id"],
            "account_id": agent["account_id"],
            "role": agent["role"],
            "endpoint_url": agent["endpoint_url"],
            "capabilities_offered": json.dumps(agent["capabilities_offered"]),
            "availability": agent["availability"],
            "is_verified": agent["is_verified"],
        }
        cur.execute(UPSERT_SQL, params)
        print(f"  ✓ {agent['name']:15s}  id={agent['id']}  port={agent['port']}  availability={agent['availability']}")

    cur.close()
    conn.close()
    print("\nAll agents registered successfully.")


if __name__ == "__main__":
    main()
