"""Shared fixtures for agent tests."""

from __future__ import annotations

from dataclasses import dataclass, field
from unittest.mock import AsyncMock

import pytest
import respx
from httpx import Response


# ---------------------------------------------------------------------------
# Mock Anthropic client
# ---------------------------------------------------------------------------

@dataclass
class FakeContentBlock:
    text: str


@dataclass
class FakeMessage:
    content: list[FakeContentBlock] = field(default_factory=list)


def make_anthropic_mock(response_json: str) -> AsyncMock:
    """Return a mock ``anthropic.AsyncAnthropic`` whose ``messages.create``
    resolves to *response_json*."""
    client = AsyncMock()
    client.messages.create = AsyncMock(
        return_value=FakeMessage(content=[FakeContentBlock(text=response_json)]),
    )
    return client


@pytest.fixture()
def anthropic_mock():
    """Fixture that yields a factory: call ``anthropic_mock(json_string)``
    to get a mock client that returns that JSON from Claude."""
    return make_anthropic_mock


# ---------------------------------------------------------------------------
# Mock callback server (respx)
# ---------------------------------------------------------------------------

@pytest.fixture()
def mock_callback():
    """Fixture that intercepts POST requests to any callback URL and records
    the payloads.  Yields a ``respx.Route`` whose ``.calls`` list contains
    every request made."""
    with respx.mock(assert_all_mocked=False) as router:
        route = router.post(url__regex=r".*/v1/tasks/.*/result").mock(
            return_value=Response(200, json={"ok": True}),
        )
        yield route
