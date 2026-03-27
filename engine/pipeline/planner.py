"""Intent classifier: determines what the user wants (Haiku-powered)."""
import json, asyncio
from anthropic import Anthropic
from utils import parse_json_response
client = Anthropic()

SYSTEM = """Classify this request. Available intents: research, summarize, extract, write, translate, analyze, general_chat, assistant.
Return ONLY JSON: {"intent":"...","required_context":[],"entity_references":[],"context_budget":"light|medium|heavy"}"""

async def plan(input_text, org_summary=None):
    msg = f"Message: {input_text}"
    if org_summary: msg += f"\nContext: {json.dumps(org_summary)}"
    loop = asyncio.get_running_loop()
    try:
        r = await loop.run_in_executor(None, lambda: client.messages.create(
            model="claude-haiku-4-5-20251001", max_tokens=300, system=SYSTEM, messages=[{"role":"user","content":msg}]))
        return parse_json_response(r.content[0].text)
    except Exception:
        return {"intent":"general_chat","required_context":["soul","business_profile"],"context_budget":"medium","degraded":True}
