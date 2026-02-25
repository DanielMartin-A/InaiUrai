import json
import logging

import anthropic
from fastapi import BackgroundTasks, FastAPI
from pydantic import BaseModel, Field

from shared.callback import send_result
from shared.config import ANTHROPIC_API_KEY

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
logger = logging.getLogger("summarize_agent")

app = FastAPI(title="Summarize Agent")

PRICE_CREDITS = 3

SYSTEM_PROMPT = (
    "You are a summarization assistant. "
    "Return ONLY valid JSON matching this schema: "
    '{"summary": "<string>", "word_count": <int>, "key_topics": ["<string>"]}'
    "\nDo NOT include any text outside the JSON object."
)


class TaskInput(BaseModel):
    text: str = Field(..., max_length=100_000)
    format: str = Field(default="paragraph")
    focus: str = Field(default="")


class TaskRequest(BaseModel):
    task_id: str
    capability: str = "summarize"
    input_payload: TaskInput
    callback_url: str
    deadline: str | None = None


@app.post("/task")
async def receive_task(req: TaskRequest, background_tasks: BackgroundTasks):
    """Return 200 immediately, process in background per .cursorrules Python Agents rule."""
    logger.info("received task_id=%s format=%s", req.task_id, req.input_payload.format)
    background_tasks.add_task(
        process_summarize,
        task_id=req.task_id,
        task_input=req.input_payload,
        callback_url=req.callback_url,
    )
    return {"status": "acknowledged"}


async def process_summarize(task_id: str, task_input: TaskInput, callback_url: str) -> None:
    """Call Claude, parse the response, and POST result back to the router."""
    try:
        client = anthropic.AsyncAnthropic(api_key=ANTHROPIC_API_KEY)

        focus_line = f"\nFocus on: {task_input.focus}" if task_input.focus else ""
        user_prompt = (
            f"Summarize the following text in {task_input.format} format.{focus_line}\n\n"
            f"{task_input.text}"
        )

        message = await client.messages.create(
            model="claude-sonnet-4-5-20250929",
            max_tokens=2048,
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
                "summary": "",
                "word_count": 0,
                "key_topics": [],
            },
            "output_status": "error",
            "actual_cost": 0,
        }
        await send_result(
            callback_url=callback_url,
            task_id=task_id,
            output_dict=error_result,
        )
