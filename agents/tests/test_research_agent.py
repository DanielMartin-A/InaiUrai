"""Tests for the Research Agent (FastAPI app)."""

from __future__ import annotations

import json
import time
from unittest.mock import patch

import pytest
from httpx import ASGITransport, AsyncClient

from research_agent.main import TaskInput, app, process_research


# -----------------------------------------------------------------------
# 1. GET /health -> 200 OK
# -----------------------------------------------------------------------

@pytest.mark.anyio
async def test_health_endpoint():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        resp = await ac.get("/health")

    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


# -----------------------------------------------------------------------
# 2. POST /task -> 200 immediately (BackgroundTasks proof)
# -----------------------------------------------------------------------

@pytest.mark.anyio
async def test_task_returns_200_immediately():
    payload = {
        "task_id": "00000000-0000-0000-0000-000000000099",
        "capability": "research",
        "input_payload": {
            "query": "Latest AI trends",
            "depth": "quick",
            "max_sources": 3,
        },
        "callback_url": "http://localhost:8080/v1/tasks/00000000-0000-0000-0000-000000000099/result",
    }

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        start = time.monotonic()
        resp = await ac.post("/task", json=payload)
        elapsed = time.monotonic() - start

    assert resp.status_code == 200
    body = resp.json()
    assert body == {"status": "acknowledged"}
    assert elapsed < 1.0, f"Response took {elapsed:.2f}s — background task may be blocking"


# -----------------------------------------------------------------------
# 3. Callback logic: mock Anthropic + capture callback POST
# -----------------------------------------------------------------------

CLAUDE_RESPONSE = json.dumps(
    {
        "findings": "AI is advancing rapidly.",
        "key_points": ["LLMs", "Agents"],
        "sources": [
            {"title": "Paper A", "url": "https://example.com/a", "relevance": "high"},
        ],
    }
)


@pytest.mark.anyio
async def test_callback_logic(anthropic_mock, mock_callback):
    mock_client = anthropic_mock(CLAUDE_RESPONSE)

    task_input = TaskInput(query="AI trends", depth="standard", max_sources=5)
    callback_url = "http://router:8080/v1/tasks/test-task-123/result"

    with patch("research_agent.main.anthropic.AsyncAnthropic", return_value=mock_client):
        await process_research(
            task_id="test-task-123",
            task_input=task_input,
            callback_url=callback_url,
        )

    assert mock_callback.call_count == 1

    request = mock_callback.calls[0].request
    body = json.loads(request.content)

    assert body["task_id"] == "test-task-123"
    assert body["output_status"] == "success"
    assert body["actual_cost"] == 8

    output = body["output_payload"]
    assert output["status"] == "success"
    assert output["findings"] == "AI is advancing rapidly."
    assert "LLMs" in output["key_points"]
    assert len(output["sources"]) == 1
    assert output["sources"][0]["title"] == "Paper A"
