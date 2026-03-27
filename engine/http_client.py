"""Shared httpx client — connection pool reused across all modules.

Prevents per-request TCP connection churn. Every module that calls the Go
backend should import `backend_client` from here instead of creating
its own `httpx.AsyncClient()`.
"""
import os, httpx

BACKEND_URL = os.getenv("BACKEND_URL", "http://localhost:8080")
INTERNAL_KEY = os.getenv("INTERNAL_API_KEY", "")

_backend_client: httpx.AsyncClient | None = None

def get_backend_client() -> httpx.AsyncClient:
    global _backend_client
    if _backend_client is None or _backend_client.is_closed:
        _backend_client = httpx.AsyncClient(
            base_url=BACKEND_URL,
            timeout=httpx.Timeout(10.0, connect=5.0),
            limits=httpx.Limits(max_connections=20, max_keepalive_connections=10),
            headers={"X-Internal-Key": INTERNAL_KEY} if INTERNAL_KEY else {},
        )
    return _backend_client

async def close_backend_client():
    global _backend_client
    if _backend_client and not _backend_client.is_closed:
        await _backend_client.aclose()
        _backend_client = None
