"""InaiUrai Engine v5.0 — AI Workforce-as-a-Service."""
import json, os, re, asyncio, base64, time, hmac, signal, uuid
import logging
from collections import defaultdict
from contextlib import asynccontextmanager
from fastapi import FastAPI, Request, HTTPException, APIRouter
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from typing import Optional
from anthropic import Anthropic
from pipeline.agent_loop import agent_loop
from pipeline.orchestrator import orchestrate_engagement, route_to_role
from pipeline.planner import plan as run_planner
from tools.doc_parser import parse_file
from utils import parse_json_response
from http_client import get_backend_client, close_backend_client

LOG_FORMAT = os.getenv("LOG_FORMAT", "json")

class JSONFormatter(logging.Formatter):
    def format(self, record):
        log = {
            "ts": self.formatTime(record, self.datefmt),
            "level": record.levelname,
            "msg": record.getMessage(),
            "logger": record.name,
        }
        for key in ("org_id", "task_id", "request_id"):
            if hasattr(record, key):
                log[key] = getattr(record, key)
        if record.exc_info and record.exc_info[0]:
            log["error"] = f"{record.exc_info[0].__name__}: {record.exc_info[1]}"
        return json.dumps(log)

def setup_logging():
    root = logging.getLogger()
    root.setLevel(logging.INFO)
    handler = logging.StreamHandler()
    if LOG_FORMAT == "json":
        handler.setFormatter(JSONFormatter())
    else:
        handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s [%(name)s] %(message)s"))
    root.handlers = [handler]

setup_logging()
logger = logging.getLogger("inaiurai.engine")

INTERNAL_KEY = os.getenv("INTERNAL_API_KEY", "")
INTERNAL_KEY_PREV = os.getenv("INTERNAL_API_KEY_PREVIOUS", "")

MAX_CONCURRENT_TASKS = int(os.getenv("MAX_CONCURRENT_TASKS", "15"))
_task_semaphore = asyncio.Semaphore(MAX_CONCURRENT_TASKS)

_shutting_down = False

@asynccontextmanager
async def lifespan(app):
    key = os.environ.get("ANTHROPIC_API_KEY", "")
    key_status = "loaded" if key and len(key) > 10 else "MISSING"
    internal_status = "configured" if INTERNAL_KEY else "WARNING: not set"
    rotation_status = "active" if INTERNAL_KEY_PREV else "off"
    logger.info(f"InaiUrai Engine v5.0 starting",
        extra={"anthropic_key": key_status, "internal_key": internal_status, "key_rotation": rotation_status})
    logger.info("Security layers: Auth Middleware | Permission Gate | Firewall | Validator | Governor | Audit")

    def handle_sigterm(*args):
        global _shutting_down
        _shutting_down = True
        logger.warning("SIGTERM received — draining in-flight tasks")
    signal.signal(signal.SIGTERM, handle_sigterm)

    yield

    logger.info("Engine shutting down — waiting for in-flight tasks")
    for _ in range(MAX_CONCURRENT_TASKS):
        await _task_semaphore.acquire()
    await close_backend_client()
    logger.info("All tasks drained. Shutdown complete.")

app = FastAPI(title="InaiUrai Engine", version="5.0", lifespan=lifespan)

_anthropic_client = None
def get_anthropic_client() -> Anthropic:
    global _anthropic_client
    if _anthropic_client is None:
        _anthropic_client = Anthropic()
    return _anthropic_client

client = get_anthropic_client()

def _verify_key(provided: str) -> bool:
    """Constant-time key verification with rotation support."""
    if not provided:
        return False
    if INTERNAL_KEY and hmac.compare_digest(provided.encode(), INTERNAL_KEY.encode()):
        return True
    if INTERNAL_KEY_PREV and hmac.compare_digest(provided.encode(), INTERNAL_KEY_PREV.encode()):
        return True
    return False

@app.middleware("http")
async def verify_internal_key(request: Request, call_next):
    if request.url.path in ("/health", "/health/ready"):
        return await call_next(request)
    if _shutting_down:
        return JSONResponse(status_code=503, content={"error": "Engine shutting down"})
    if not INTERNAL_KEY:
        raise HTTPException(status_code=503, detail="Engine not configured: INTERNAL_API_KEY missing")
    provided = request.headers.get("X-Internal-Key", "")
    if not _verify_key(provided):
        raise HTTPException(status_code=403, detail="Forbidden")
    request.state.request_id = request.headers.get("X-Request-ID", str(uuid.uuid4()))
    return await call_next(request)

_rate_buckets: dict[str, list[float]] = defaultdict(list)
_rate_last_cleanup: float = time.time()

def check_rate_limit(org_id: str, max_per_minute: int = 30) -> bool:
    global _rate_last_cleanup
    now = time.time()
    if now - _rate_last_cleanup > 300:
        stale = [k for k, v in _rate_buckets.items() if not v or now - v[-1] > 120]
        for k in stale:
            del _rate_buckets[k]
        _rate_last_cleanup = now
    bucket = _rate_buckets[org_id]
    _rate_buckets[org_id] = [t for t in bucket if now - t < 60]
    if len(_rate_buckets[org_id]) >= max_per_minute:
        logger.warning(f"Rate limit hit: {org_id}", extra={"org_id": org_id})
        return False
    _rate_buckets[org_id].append(now)
    return True

MAX_INPUT_LENGTH = 50_000

class TaskRequest(BaseModel):
    task_id: str = ""
    input_text: str
    org_context: dict = {}
    org_soul: str = ""
    member_profile: str = ""
    role: str = "chief-of-staff"
    tier: str = "solo"
    engagement_id: str = ""
    org_id: str = ""
    member_id: str = ""
    goal_ancestry: Optional[dict] = None
    role_budget_cents: int = 0

    def model_post_init(self, __context):
        if len(self.input_text) > MAX_INPUT_LENGTH:
            self.input_text = self.input_text[:MAX_INPUT_LENGTH]

class OrchestrateRequest(BaseModel):
    objective: str
    org_context: dict = {}
    org_soul: str = ""
    org_id: str = ""
    member_id: str = ""

    def model_post_init(self, __context):
        if len(self.objective) > MAX_INPUT_LENGTH:
            self.objective = self.objective[:MAX_INPUT_LENGTH]

class RouteRequest(BaseModel):
    input_text: str
    org_soul: str = ""

    def model_post_init(self, __context):
        if len(self.input_text) > MAX_INPUT_LENGTH:
            self.input_text = self.input_text[:MAX_INPUT_LENGTH]

class PlanRequest(BaseModel):
    input_text: str
    org_id: str = ""
    org_summary: Optional[dict] = None

    def model_post_init(self, __context):
        if len(self.input_text) > MAX_INPUT_LENGTH:
            self.input_text = self.input_text[:MAX_INPUT_LENGTH]

class ExtractRequest(BaseModel):
    text: str
    filename: str = ""
    mime_type: str = ""
    encoding: str = ""

class SoulRequest(BaseModel):
    text: str

class PIIRequest(BaseModel):
    text: str

v1 = APIRouter(prefix="/v1", tags=["v1"])

@v1.post("/run_task")
async def run_task(req: TaskRequest):
    if req.org_id and not check_rate_limit(req.org_id):
        raise HTTPException(status_code=429, detail="Rate limit exceeded. Max 30 requests/minute.")
    async with _task_semaphore:
        return await agent_loop.run(req.model_dump())

@v1.post("/orchestrate")
async def orchestrate(req: OrchestrateRequest):
    """Analyze objective and propose team + execution plan."""
    if req.org_id and not check_rate_limit(req.org_id):
        raise HTTPException(status_code=429, detail="Rate limit exceeded.")
    return await orchestrate_engagement(req.model_dump())

@v1.post("/route")
async def route(req: RouteRequest):
    """For solo messages: determine the single best role. Fast (Haiku-powered)."""
    return await route_to_role(req.input_text, req.org_soul)

@v1.post("/plan")
async def plan_task(req: PlanRequest):
    return await run_planner(input_text=req.input_text, org_summary=req.org_summary)

@v1.post("/classify")
async def classify_task(req: RouteRequest):
    from pipeline.classifier import classify
    return await classify(req.input_text)

@v1.get("/roles")
async def list_roles():
    from configs.roles.base import get_all_roles
    return {"roles": get_all_roles()}

@v1.post("/extract_context")
async def extract_context(req: ExtractRequest):
    text = req.text
    try:
        if req.encoding == "base64":
            text = parse_file(base64.b64decode(req.text), req.mime_type)
        elif req.mime_type and req.mime_type != "text/plain":
            text = parse_file(req.text.encode("utf-8", errors="replace"), req.mime_type)
    except Exception:
        return {"summary": "Failed to decode file", "entities_found": []}
    loop = asyncio.get_running_loop()
    try:
        response = await loop.run_in_executor(None, lambda: client.messages.create(
            model="claude-sonnet-4-6", max_tokens=500,
            system='Summarize in under 200 words. Extract entities. JSON only: {"summary":"...","entities_found":[...]}',
            messages=[{"role": "user", "content": text[:5000]}]))
        return parse_json_response(response.content[0].text)
    except Exception:
        return {"summary": "Context extraction failed", "entities_found": []}

@v1.post("/generate_soul")
async def generate_soul(req: SoulRequest):
    loop = asyncio.get_running_loop()
    try:
        response = await loop.run_in_executor(None, lambda: client.messages.create(
            model="claude-sonnet-4-6", max_tokens=300,
            system="Based on this organization data, write a 100-200 word instruction for an AI employee about how to work for this organization. Include: their business, industry, competitors, products, brand voice. Write in second person. Be specific.",
            messages=[{"role": "user", "content": req.text[:10000]}]))
        return {"soul": response.content[0].text}
    except Exception:
        return {"soul": "New organization. Learn about this client through conversation."}

@v1.post("/detect_pii")
async def detect_pii(req: PIIRequest):
    patterns = {"ssn": r"\b\d{3}-\d{2}-\d{4}\b", "credit_card": r"\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b",
        "email": r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b", "phone": r"\b\d{3}[-.]?\d{3}[-.]?\d{4}\b"}
    found, count = [], 0
    for t, pat in patterns.items():
        m = re.findall(pat, req.text)
        if m: found.append(t); count += len(m)
    return {"has_pii": len(found) > 0, "types_found": found, "count": count}

@v1.get("/trace/{task_id}")
async def get_trace(task_id: str):
    from pipeline.audit import generate_trace_summary
    return await generate_trace_summary(task_id)

app.include_router(v1)
app.include_router(v1, prefix="")

@app.get("/health")
async def health():
    return {"status": "ok", "version": "5.0"}

@app.get("/health/ready")
async def health_ready():
    """Readiness probe — checks downstream dependencies."""
    checks = {"engine": "ok", "backend": "unknown", "anthropic": "unknown"}
    ready = True
    try:
        import httpx
        async with httpx.AsyncClient() as c:
            r = await c.get(f"{os.getenv('BACKEND_URL', 'http://backend:8080')}/health", timeout=3)
            checks["backend"] = "ok" if r.status_code == 200 else f"error:{r.status_code}"
            if r.status_code != 200: ready = False
    except Exception:
        checks["backend"] = "unreachable"; ready = False
    try:
        key = os.environ.get("ANTHROPIC_API_KEY", "")
        checks["anthropic"] = "configured" if key and len(key) > 10 else "MISSING"
        if not key: ready = False
    except Exception:
        checks["anthropic"] = "error"; ready = False
    status_code = 200 if ready else 503
    return JSONResponse(status_code=status_code,
        content={"status": "ready" if ready else "degraded", "version": "5.0",
            "checks": checks, "concurrent_tasks": MAX_CONCURRENT_TASKS - _task_semaphore._value})
