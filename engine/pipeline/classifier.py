import json, asyncio
from anthropic import Anthropic
client = Anthropic()

async def classify(input_text):
    loop = asyncio.get_running_loop()
    try:
        r = await loop.run_in_executor(None, lambda: client.messages.create(
            model="claude-sonnet-4-6", max_tokens=200,
            system='Classify: research, summarize, extract, write, translate, analyze. JSON: {"capability":"...","refined_query":"..."}',
            messages=[{"role":"user","content":input_text}]))
        return json.loads(r.content[0].text)
    except (json.JSONDecodeError, IndexError, KeyError):
        return {"capability":"research","refined_query":input_text}
    except Exception:
        return {"capability":"assistant","refined_query":input_text,"degraded":True}
