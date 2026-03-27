"""Tool Proxy: Permission gate enforcing role tool allowlists."""
import json
from typing import Any
from enum import IntEnum
from tools.web_search import search as web_search
from tools.url_reader import read_url
from pipeline.audit import AuditLogger

class ActionTier(IntEnum):
    OBSERVE = 1
    DRAFT = 2
    EXECUTE = 3

TOOL_REGISTRY = {
    "web_search": {"tier": ActionTier.OBSERVE, "description": "Search the web. Returns titles, URLs, snippets.",
        "input_schema": {"type":"object","properties":{"query":{"type":"string","description":"Search query (2-8 words)"},"num_results":{"type":"integer","default":3}},"required":["query"]}},
    "read_url": {"tier": ActionTier.OBSERVE, "description": "Fetch readable text from a URL.",
        "input_schema": {"type":"object","properties":{"url":{"type":"string","description":"URL to read"}},"required":["url"]}},
    "recall_context": {"tier": ActionTier.OBSERVE, "description": "Retrieve org context mid-task (business profile, entities, documents).",
        "input_schema": {"type":"object","properties":{"context_types":{"type":"array","items":{"type":"string","enum":["soul","business_profile","entities","preferences","documents","project_history"]}},"entity_names":{"type":"array","items":{"type":"string"}}},"required":["context_types"]}},
    "check_calculation": {"tier": ActionTier.OBSERVE, "description": "Verify a calculation.",
        "input_schema": {"type":"object","properties":{"expression":{"type":"string"},"expected_result":{"type":"number"}},"required":["expression"]}},
    "draft_email": {"tier": ActionTier.DRAFT, "description": "Draft an email for review before sending.",
        "input_schema": {"type":"object","properties":{"to":{"type":"string"},"subject":{"type":"string"},"body":{"type":"string"}},"required":["to","subject","body"]}},
    "draft_document": {"tier": ActionTier.DRAFT, "description": "Create a structured document for review.",
        "input_schema": {"type":"object","properties":{"title":{"type":"string"},"content":{"type":"string"},"doc_type":{"type":"string","enum":["report","memo","brief","proposal","analysis"]}},"required":["title","content"]}},
    "send_email": {"tier": ActionTier.EXECUTE, "description": "Send email autonomously. Requires authorization.",
        "input_schema": {"type":"object","properties":{"to":{"type":"string"},"subject":{"type":"string"},"body":{"type":"string"}},"required":["to","subject","body"]}},
}

ROLE_TOOL_ALLOWLISTS = {
    "chief-of-staff":  ["web_search","read_url","recall_context","draft_email","draft_document"],
    "coo":             ["web_search","read_url","recall_context","draft_document"],
    "cpo":             ["web_search","read_url","recall_context","draft_document"],
    "cmo":             ["web_search","read_url","recall_context","draft_email","draft_document"],
    "cro":             ["web_search","read_url","recall_context","draft_email","draft_document"],
    "cbo":             ["web_search","read_url","recall_context","draft_document"],
    "cfo":             ["web_search","read_url","recall_context","check_calculation","draft_document"],
    "cio":             ["web_search","read_url","recall_context","draft_document"],
    "researcher":      ["web_search","read_url","recall_context","draft_document"],
    "cco":             ["web_search","read_url","recall_context","draft_email","draft_document"],
    "content-chief":   ["web_search","read_url","recall_context","draft_document"],
    "creative-chief":  ["web_search","read_url","recall_context","draft_document"],
    "general-counsel": ["web_search","read_url","recall_context","draft_document"],
    "cto":             ["web_search","read_url","recall_context","draft_document"],
    "cdo":             ["web_search","read_url","recall_context","check_calculation","draft_document"],
    "product-chief":   ["web_search","read_url","recall_context","draft_document"],
}

class ToolProxy:
    def __init__(self, role_slug, org_id, audit, context_fetcher=None, authorizations=None):
        self.role_slug = role_slug
        self.org_id = org_id
        self.audit = audit
        self.context_fetcher = context_fetcher
        self.authorizations = authorizations or {}
        self.allowlist = ROLE_TOOL_ALLOWLISTS.get(role_slug, ["web_search","read_url","recall_context"])

    def get_tool_schemas(self):
        return [{"name": n, "description": TOOL_REGISTRY[n]["description"], "input_schema": TOOL_REGISTRY[n]["input_schema"]}
            for n in self.allowlist if n in TOOL_REGISTRY]

    async def execute(self, tool_name, tool_input, step):
        if tool_name not in self.allowlist:
            reason = f"Tool '{tool_name}' not available for {self.role_slug}"
            self.audit.log_block(step, tool_name, tool_input, "role_allowlist", reason)
            return {"status": "blocked", "result": reason}
        tool_def = TOOL_REGISTRY.get(tool_name)
        if not tool_def:
            return {"status": "blocked", "result": f"Unknown tool: {tool_name}"}
        tier = tool_def["tier"]
        if tier == ActionTier.DRAFT:
            self.audit.log_tool_call(step, tool_name, tool_input, {"status": "confirmation_needed"}, 0)
            return {"status": "confirmation_needed", "result": f"Draft created. Requires confirmation.", "draft": tool_input}
        if tier == ActionTier.EXECUTE:
            auth = self.authorizations.get(tool_name)
            if not auth or not auth.get("is_enabled"):
                self.audit.log_block(step, tool_name, tool_input, "no_authorization", "Requires customer authorization")
                return {"status": "blocked", "result": "Requires explicit authorization"}
        try:
            result = await self._dispatch(tool_name, tool_input)
            self.audit.log_tool_call(step, tool_name, tool_input, result, 0)
            return {"status": "success", "result": result}
        except Exception as e:
            self.audit.log_error(step, tool_name, str(e))
            return {"status": "error", "result": f"Tool error: {type(e).__name__}. Try a different approach."}

    async def _dispatch(self, name, inp):
        if name == "web_search": return await web_search(inp["query"], inp.get("num_results", 3))
        elif name == "read_url": return {"url": inp["url"], "content": await read_url(inp["url"])}
        elif name == "recall_context":
            if self.context_fetcher: return await self.context_fetcher(self.org_id, inp.get("context_types",[]), inp.get("entity_names",[]))
            return {"error": "Context not configured"}
        elif name == "check_calculation":
            import ast, operator
            SAFE_OPS = {ast.Add: operator.add, ast.Sub: operator.sub, ast.Mult: operator.mul,
                ast.Div: operator.truediv, ast.Mod: operator.mod, ast.Pow: operator.pow, ast.USub: operator.neg}
            def _safe_eval(node):
                if isinstance(node, ast.Num): return node.n
                if isinstance(node, ast.Constant) and isinstance(node.value, (int, float)): return node.value
                if isinstance(node, ast.BinOp) and type(node.op) in SAFE_OPS:
                    left, right = _safe_eval(node.left), _safe_eval(node.right)
                    if type(node.op) == ast.Pow and right > 100: raise ValueError("Exponent too large")
                    if type(node.op) in (ast.Div, ast.Mod) and right == 0: raise ValueError("Division by zero")
                    return SAFE_OPS[type(node.op)](left, right)
                if isinstance(node, ast.UnaryOp) and type(node.op) in SAFE_OPS:
                    return SAFE_OPS[type(node.op)](_safe_eval(node.operand))
                raise ValueError(f"Unsupported operation: {type(node).__name__}")
            try:
                tree = ast.parse(inp["expression"].strip(), mode="eval")
                r = _safe_eval(tree.body); exp = inp.get("expected_result")
                return {"result": r, "expected": exp, "correct": abs(r-exp)<0.01 if exp is not None else None}
            except Exception as e:
                return {"error": f"Calculation error: {type(e).__name__}"}
        elif name == "draft_email": return {"draft_type":"email", **inp, "status":"saved_for_review"}
        elif name == "draft_document": return {"draft_type":"document", **inp, "status":"saved_for_review"}
        return {"error": f"No handler: {name}"}
