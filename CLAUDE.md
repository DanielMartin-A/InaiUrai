# InaiUrai v5.0 — AI Workforce-as-a-Service

## Quick Reference
- Backend: Go 1.23+ on :8080
- Engine: Python 3.12 + FastAPI on :8000 (internal)
- DB: PostgreSQL 16
- LLM: Claude Sonnet 4.6 (reasoning), Haiku 4.5 (planning)
- Deploy: Docker Compose on Railway

## Key Abstractions
- Organization: billing entity, owns context + engagements
- Member: human in an org, has channel connections + preferences
- Engagement: unit of work (task/project/department/company mode)
- Task: single agent execution within an engagement
- Role: AI executive spec (system prompt + tools + constraints)

## Build Commands
```bash
make bootstrap    # up + migrate all + seed
make reset        # clean restart
make test-all     # all tests
make logs         # docker compose logs -f
```

## Common Tasks

### Add new tool
1. Add to TOOL_REGISTRY in engine/pipeline/tool_proxy.py
2. Add handler in ToolProxy._dispatch()
3. Add to role allowlists in ROLE_TOOL_ALLOWLISTS

### Add new role
1. Add config to ROLE_CONFIGS in engine/configs/roles/base.py
2. INSERT into role_catalog (new migration or seed)
3. Add tool allowlist entry

### Debug failed task
1. SELECT * FROM tasks WHERE id = 'xxx'
2. SELECT * FROM agent_audit_trail WHERE task_id = 'xxx' ORDER BY step_number
3. SELECT * FROM engagements WHERE id = (task's engagement_id)

### Get human-readable trace for a task
GET /trace/{task_id} on the engine returns a clean narrative:
  Step 1: Searched for "CRM tools" → Found 5 results
  Step 2: Read page at buffer.com
  Step 3: Produced final output (2400 chars)
This powers the /trace Telegram command and dashboard trace view.
