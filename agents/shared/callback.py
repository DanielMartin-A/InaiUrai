import logging

import httpx

from shared.config import ROUTER_URL

logger = logging.getLogger(__name__)

CALLBACK_TIMEOUT = 15.0


async def send_result(
    callback_url: str,
    task_id: str,
    output_dict: dict,
) -> None:
    """POST the task result back to the Router's callback endpoint."""
    url = callback_url
    if not url.startswith("http"):
        url = f"{ROUTER_URL}{callback_url}"

    payload = {
        "task_id": task_id,
        "output_payload": output_dict.get("output_payload", output_dict),
        "output_status": output_dict.get("output_status", "success"),
        "actual_cost": output_dict.get("actual_cost", 0),
    }

    async with httpx.AsyncClient(timeout=CALLBACK_TIMEOUT) as client:
        try:
            resp = await client.post(url, json=payload)
            resp.raise_for_status()
            logger.info("callback sent task_id=%s status=%d", task_id, resp.status_code)
        except httpx.HTTPStatusError as exc:
            logger.error(
                "callback HTTP error task_id=%s status=%d body=%s",
                task_id,
                exc.response.status_code,
                exc.response.text[:200],
            )
        except Exception as exc:
            logger.error("callback failed task_id=%s error=%s", task_id, exc)
