import json
import logging

import anthropic
from fastapi import BackgroundTasks, FastAPI
from pydantic import BaseModel, Field

from shared.callback import send_result
from shared.config import ANTHROPIC_API_KEY

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
logger = logging.getLogger("extraction_agent")

app = FastAPI(title="Data Extraction Agent")

PRICE_CREDITS = 5

SYSTEM_PROMPT = (
    "You are a data extraction assistant. "
    "Return ONLY valid JSON matching this schema: "
    '{"extracted_data": {<field_name>: <value>}, "confidence": <float 0-1>, "notes": "<string>"}'
    "\nDo NOT include any text outside the JSON object."
)


class FieldSpec(BaseModel):
    name: str
    type: str
    description: str = ""


class TaskInput(BaseModel):
    text: str = Field(...)
    fields: list[FieldSpec] = Field(..., min_length=1)


class TaskRequest(BaseModel):
    task_id: str
    capability: str = "data_extraction"
    input_payload: TaskInput
    callback_url: str
    deadline: str | None = None


@app.post("/task")
async def receive_task(req: TaskRequest, background_tasks: BackgroundTasks):
    """Return 200 immediately, process in background per .cursorrules Python Agents rule."""
    logger.info("received task_id=%s fields=%d", req.task_id, len(req.input_payload.fields))
    background_tasks.add_task(
        process_extraction,
        task_id=req.task_id,
        task_input=req.input_payload,
        callback_url=req.callback_url,
    )
    return {"status": "acknowledged"}


async def process_extraction(task_id: str, task_input: TaskInput, callback_url: str) -> None:
    """Call Claude, parse the response, and POST result back to the router."""
    try:
        client = anthropic.AsyncAnthropic(api_key=ANTHROPIC_API_KEY)

        fields_description = "\n".join(
            f"- {f.name} ({f.type}): {f.description}" for f in task_input.fields
        )
        user_prompt = (
            f"Extract the following fields from the text below.\n\n"
            f"Fields to extract:\n{fields_description}\n\n"
            f"Text:\n{task_input.text}"
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
                "extracted_data": {},
                "confidence": 0.0,
                "notes": "",
            },
            "output_status": "error",
            "actual_cost": 0,
        }
        await send_result(
            callback_url=callback_url,
            task_id=task_id,
            output_dict=error_result,
        )
