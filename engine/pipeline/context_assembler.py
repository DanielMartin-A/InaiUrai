import os
from http_client import get_backend_client
BUDGETS = {"light":500,"medium":1500,"heavy":3000}

async def assemble(planner_output, org_id):
    budget = BUDGETS.get(planner_output.get("context_budget","medium"), 1500)
    if not planner_output.get("required_context"): return {}
    try:
        c = get_backend_client()
        r = await c.post("/api/context/selective",
            json={"org_id":org_id,"context_types":planner_output.get("required_context",[]),
                "entity_names":planner_output.get("entity_references",[]),"max_tokens":budget},
            timeout=10)
        if r.status_code == 200: return r.json()
    except Exception:
        pass
    return None
