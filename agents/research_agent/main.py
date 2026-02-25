import json
import logging

import anthropic
from fastapi import BackgroundTasks, FastAPI
from pydantic import BaseModel, Field

from shared.callback import send_result
from shared.config import ANTHROPIC_API_KEY

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
logger = logging.getLogger("research_agent")

app = FastAPI(title="Research Agent")


@app.get("/health")
async def health():
    return {"status": "ok"}


PRICE_CREDITS = 8

MAX_TOKENS_BY_DEPTH = {
    "quick": 1024,
    "standard": 2048,
    "deep": 4096,
}

SYSTEM_PROMPT = (
    "You are a research assistant. "
    "Return ONLY valid JSON matching this schema: "
    '{"findings": "<string>", "key_points": ["<string>"], '
    '"sources": [{"title": "<string>", "url": "<string>", "relevance": "<string>"}]}'
    "\nDo NOT include any text outside the JSON object."
)


class TaskInput(BaseModel):
    query: str = Field(..., min_length=3, max_length=1000)
    depth: str = Field(default="standard")
    max_sources: int = Field(default=5, ge=1)


class TaskRequest(BaseModel):
    task_id: str
    capability: str = "research"
    input_payload: TaskInput
    callback_url: str
    deadline: str | None = None


@app.post("/task")
async def receive_task(req: TaskRequest, background_tasks: BackgroundTasks):
    """Return 200 immediately, process in background per .cursorrules Python Agents rule."""
    logger.info("received task_id=%s depth=%s", req.task_id, req.input_payload.depth)
    background_tasks.add_task(
        process_research,
        task_id=req.task_id,
        task_input=req.input_payload,
        callback_url=req.callback_url,
    )
    return {"status": "acknowledged"}


async def process_research(task_id: str, task_input: TaskInput, callback_url: str) -> None:
    """Call Claude, parse the response, and POST result back to the router."""
    try:
        client = anthropic.AsyncAnthropic(api_key=ANTHROPIC_API_KEY)

        depth = task_input.depth if task_input.depth in MAX_TOKENS_BY_DEPTH else "standard"
        max_tokens = MAX_TOKENS_BY_DEPTH[depth]

        user_prompt = (
            f"Research the following query: {task_input.query}\n"
            f"Depth level: {depth}\n"
            f"Return up to {task_input.max_sources} sources."
        )

        message = await client.messages.create(
            model="claude-sonnet-4-5-20250929",
            max_tokens=max_tokens,
            system=SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
        )

        raw_text = message.content[0].text
        result = json.loads(raw_text)

        result["status"] = "success"

        await send_result(
            callback_url=callback_url,
            task_id=task_id,
            output_dict={
                "output_payload": result,
                "output_status": "success",
                "actual_cost": PRICE_CREDITS,
            },
        )
        logger.info("task_id=%s completed successfully", task_id)

    except Exception as exc:
        logger.error("task_id=%s failed: %s", task_id, exc)
        error_result = {
            "output_payload": {
                "status": "error",
                "error": {
                    "code": type(exc).__name__,
                    "message": str(exc)[:500],
                },
                "findings": "",
                "key_points": [],
                "sources": [],
            },
            "output_status": "error",
            "actual_cost": 0,
        }
        await send_result(
            callback_url=callback_url,
            task_id=task_id,
            output_dict=error_result,
        )
