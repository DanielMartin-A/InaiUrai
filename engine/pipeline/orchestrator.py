"""Orchestrator: Dynamic team assembly and engagement planning.

Analyzes customer objectives and proposes the right team of AI executives.
For solo messages: auto-routes to the best single role.
For complex objectives: proposes multi-role team with execution plan.
"""
import json
import asyncio
from anthropic import Anthropic
from configs.roles.base import get_all_roles, ROLE_CONFIGS
from utils import parse_json_response

client = Anthropic()

ORCHESTRATOR_SYSTEM = """You are the InaiUrai engagement planner. Your job is to analyze a customer's
business objective and determine the right team of AI executives to accomplish it.

AVAILABLE ROLES:
{roles_list}

ENGAGEMENT TYPES:
- task: Single role, single request, immediate response (30-120 sec)
- project: Multiple roles, coordinated deliverable, one-time (hours to days)
- department: Multiple roles, ongoing with proactive heartbeats (monthly)
- company: Full AI workforce, build and operate business functions (monthly)

RULES:
1. For simple, single-topic requests → engagement_type: "task", assign 1 role
2. For multi-faceted objectives needing coordination → "project", assign 2-6 roles
3. For "be my X department" or ongoing monitoring → "department", 2-4 roles with heartbeats
4. For "build and run my X" → "company", 4-8 roles with phases
5. Always include Chief of Staff for projects/departments/companies as synthesis coordinator
6. Order tasks by dependencies — what must finish before what can start
7. For heartbeats: specify what each role does proactively and how often

Return ONLY JSON (no markdown fences):
{
  "engagement_type": "task|project|department|company",
  "team": [
    {"role_slug": "...", "purpose": "what this role does in this engagement"}
  ],
  "execution_plan": {
    "phases": [
      {
        "name": "...",
        "tasks": [
          {"role_slug": "...", "description": "...", "depends_on": []}
        ]
      }
    ]
  },
  "heartbeat_config": {
    "role_slug": {"schedule": "weekly|daily|biweekly", "task_description": "what to do proactively"}
  },
  "estimated_tasks": 5,
  "estimated_duration": "2 hours",
  "summary": "One-line summary of the plan for the customer"
}

For task-mode (single role): omit execution_plan and heartbeat_config.
For project: include execution_plan, omit heartbeat_config.
For department/company: include both execution_plan (foundation phase) and heartbeat_config (ongoing).
"""

ROUTE_SYSTEM = """You are a request router. Given a user message, determine which single AI executive
role is best suited to handle it. Return ONLY JSON: {"role_slug": "...", "reasoning": "..."}

ROLES:
{roles_list}

Rules:
- Market/competitor questions → cio
- Content writing/blogs/social → content-chief or cmo
- Financial analysis/budgets → cfo
- Legal questions/contracts → general-counsel
- Technical architecture → cto
- Data analysis/metrics → cdo
- Product decisions/PRDs → product-chief
- General business/coordination → chief-of-staff
- Deep research → researcher
- Brand/naming → cbo
- Sales/outreach → cro
- Press/comms → cco
- Hiring/HR → cpo
- Operations/processes → coo
- Creative/brainstorming → creative-chief
"""


def _roles_list() -> str:
    lines = []
    for r in get_all_roles():
        prompt_preview = ROLE_CONFIGS[r['slug']].get('system_prompt', '')[:80]
        lines.append(f"- {r['slug']}: {r['title']} ({r['division']}) — {prompt_preview}...")
    return "\n".join(lines)


async def orchestrate_engagement(req: dict) -> dict:
    """Analyze objective and return engagement plan with team composition."""
    objective = req.get("objective", "")
    org_soul = req.get("org_soul", "")

    user_msg = f"OBJECTIVE: {objective}"
    if org_soul:
        user_msg += f"\n\nORGANIZATION CONTEXT:\n{org_soul}"

    system = ORCHESTRATOR_SYSTEM.replace("{roles_list}", _roles_list())

    loop = asyncio.get_running_loop()
    try:
        response = await loop.run_in_executor(None, lambda: client.messages.create(
            model="claude-sonnet-4-6", max_tokens=1500,
            system=system, messages=[{"role": "user", "content": user_msg}]))
        result = parse_json_response(response.content[0].text)
        valid_slugs = set(ROLE_CONFIGS.keys())
        validated_team = [r for r in result.get("team", []) if r.get("role_slug") in valid_slugs]
        if not validated_team:
            validated_team = [{"role_slug": "chief-of-staff", "purpose": "general assistance"}]
        eng_type = result.get("engagement_type", "task")
        if eng_type not in ("task", "project", "department", "company"):
            eng_type = "task"
        return {
            "engagement_type": eng_type,
            "team": validated_team,
            "execution_plan": result.get("execution_plan"),
            "heartbeat_config": result.get("heartbeat_config"),
            "estimated_tasks": result.get("estimated_tasks", 1),
            "estimated_duration": result.get("estimated_duration", "1 minute"),
            "summary": result.get("summary", ""),
        }
    except Exception as e:
        return {"engagement_type": "task", "team": [{"role_slug": "chief-of-staff", "purpose": "general assistance"}],
            "summary": "Routing to Chief of Staff (orchestration unavailable)", "error": "orchestration_failed"}


async def route_to_role(input_text: str, org_soul: str = "") -> dict:
    """For solo messages: determine the single best role to handle this request."""
    user_msg = f"MESSAGE: {input_text}"
    if org_soul:
        user_msg += f"\n\nORG CONTEXT: {org_soul[:500]}"

    system = ROUTE_SYSTEM.replace("{roles_list}", _roles_list())

    loop = asyncio.get_running_loop()
    try:
        response = await loop.run_in_executor(None, lambda: client.messages.create(
            model="claude-haiku-4-5-20251001", max_tokens=200,
            system=system, messages=[{"role": "user", "content": user_msg}]))
        result = parse_json_response(response.content[0].text)
        slug = result.get("role_slug", "chief-of-staff")
        if slug not in ROLE_CONFIGS:
            slug = "chief-of-staff"
        return {"role_slug": slug, "reasoning": result.get("reasoning", "")}
    except Exception:
        return {"role_slug": "chief-of-staff", "reasoning": "fallback"}
