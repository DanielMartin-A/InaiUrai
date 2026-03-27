"""Audit Logger: Append-only log of every agent reasoning step."""
import os, json, time, logging
from dataclasses import dataclass, field
from pathlib import Path
from http_client import get_backend_client

logger = logging.getLogger("inaiurai.audit")
AUDIT_FALLBACK_DIR = Path(os.getenv("AUDIT_FALLBACK_DIR", "/tmp/audit_fallback"))

BACKEND_URL = os.getenv("BACKEND_URL", "http://localhost:8080")

@dataclass
class AuditEntry:
    step_number: int; action_type: str; tool_name: str|None = None
    tool_input: dict|None = None; tool_output: dict|None = None
    tokens_used: int = 0; blocked_by: str|None = None; timestamp: float = field(default_factory=time.time)

class AuditLogger:
    def __init__(self, task_id, org_id):
        self.task_id = task_id; self.org_id = org_id; self.entries = []; self._step = 0
    @property
    def current_step(self): return self._step
    def _next_step(self): self._step += 1; return self._step
    def _trunc(self, obj, n=1000):
        if obj is None: return None
        if isinstance(obj, dict):
            s = json.dumps(obj, default=str)
            return {"_t": True, "p": s[:n]} if len(s)>n else obj
        return {"v": str(obj)[:n]}
    def log_tool_call(self, s, name, inp, out, tok=0):
        self.entries.append(AuditEntry(s, "tool_call", name, self._trunc(inp,500), self._trunc(out,2000), tok))
    def log_reasoning(self, s, summary, tok=0):
        self.entries.append(AuditEntry(s, "reasoning", tool_output={"summary": summary[:500]}, tokens_used=tok))
    def log_output(self, s, preview, tok=0):
        self.entries.append(AuditEntry(s, "output", tool_output={"preview": preview[:500]}, tokens_used=tok))
    def log_block(self, s, name, inp, by, reason):
        self.entries.append(AuditEntry(s, "guardrail_block", name, self._trunc(inp,300), {"reason":reason}, blocked_by=by))
    def log_error(self, s, ctx, err):
        self.entries.append(AuditEntry(s, "error", ctx, tool_output={"error": err[:500]}))
    def log_limit_reached(self, s, lt, details):
        self.entries.append(AuditEntry(s, "error", "cost_governor", tool_output={"limit":lt,"details":details}, blocked_by="cost_governor"))
    async def flush(self):
        if not self.entries: return
        payload = {"task_id": self.task_id, "org_id": self.org_id,
            "entries": [{"step_number":e.step_number,"action_type":e.action_type,"tool_name":e.tool_name,
                "tool_input":e.tool_input,"tool_output":e.tool_output,"tokens_used":e.tokens_used,"blocked_by":e.blocked_by} for e in self.entries]}
        try:
            c = get_backend_client()
            await c.post("/api/internal/audit", json=payload, timeout=10)
        except Exception as exc:
            logger.warning(f"audit flush failed, writing to fallback: {type(exc).__name__}",
                extra={"task_id": self.task_id, "org_id": self.org_id})
            try:
                AUDIT_FALLBACK_DIR.mkdir(parents=True, exist_ok=True)
                fallback_path = AUDIT_FALLBACK_DIR / f"{self.task_id}_{int(time.time())}.json"
                fallback_path.write_text(json.dumps(payload, default=str))
            except Exception as file_exc:
                logger.error(f"audit fallback write ALSO failed: {type(file_exc).__name__}",
                    extra={"task_id": self.task_id})
    def summary(self):
        return {"steps":len(self.entries),"tool_calls":sum(1 for e in self.entries if e.action_type=="tool_call"),
            "blocks":sum(1 for e in self.entries if e.action_type=="guardrail_block"),"tokens":sum(e.tokens_used for e in self.entries)}


async def generate_trace_summary(task_id: str) -> dict:
    """Fetch audit trail from backend and produce a human-readable reasoning trace."""
    try:
        c = get_backend_client()
        resp = await c.get(f"/api/internal/audit/{task_id}", timeout=10)
        if resp.status_code != 200:
            return {"task_id": task_id, "trace": "Trace not available.", "steps": []}
        entries = resp.json().get("entries", [])
    except Exception:
        return {"task_id": task_id, "trace": "Trace service unavailable.", "steps": []}

    steps = []
    for entry in entries:
        action = entry.get("action_type", "")
        tool = entry.get("tool_name", "")
        output = entry.get("tool_output", {}) or {}

        if action == "tool_call" and tool == "web_search":
            query = (entry.get("tool_input") or {}).get("query", "")
            count = len(output) if isinstance(output, list) else "?"
            steps.append(f"Searched for \"{query}\" \u2192 Found {count} results")
        elif action == "tool_call" and tool == "read_url":
            url = (entry.get("tool_input") or {}).get("url", "")
            domain = url.split("/")[2] if url.count("/") >= 2 else url
            steps.append(f"Read page at {domain}")
        elif action == "tool_call" and tool == "recall_context":
            types = (entry.get("tool_input") or {}).get("context_types", [])
            steps.append(f"Retrieved context: {', '.join(types)}")
        elif action == "tool_call" and tool == "check_calculation":
            expr = (entry.get("tool_input") or {}).get("expression", "")
            steps.append(f"Verified calculation: {expr}")
        elif action == "tool_call" and tool in ("draft_email", "draft_document"):
            steps.append(f"Created draft ({tool.replace('draft_', '')})")
        elif action == "reasoning":
            summary_text = output.get("summary", "")
            if summary_text:
                steps.append(f"Reasoning: {summary_text[:120]}...")
        elif action == "output":
            preview = output.get("preview", "")
            steps.append(f"Produced final output ({len(preview)} chars)")
        elif action == "guardrail_block":
            reason = output.get("reason", "unknown")
            steps.append(f"Blocked by guardrail: {reason}")
        elif action == "error":
            steps.append(f"Error: {output.get('error', output.get('limit', 'unknown'))}")

    numbered = [f"Step {i+1}: {s}" for i, s in enumerate(steps)]
    trace_text = "\n".join(numbered) if numbered else "No reasoning steps recorded."

    return {"task_id": task_id, "trace": trace_text, "steps": steps, "total_steps": len(steps)}
