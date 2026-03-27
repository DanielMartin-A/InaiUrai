"""Cost Governor: Hard limits on agent execution per tier and role."""
import os, time, logging
from dataclasses import dataclass, field
from http_client import get_backend_client

logger = logging.getLogger("inaiurai.cost_governor")

@dataclass
class TierLimits:
    max_iterations: int; max_tool_calls: int; max_tokens: int; max_wall_clock_seconds: int; daily_cost_ceiling_cents: int

TIER_LIMITS = {
    "solo":       TierLimits(10, 12, 40_000, 180, 500),
    "team":       TierLimits(10, 12, 40_000, 180, 2000),
    "project":    TierLimits(14, 18, 60_000, 300, 5000),
    "department": TierLimits(14, 18, 60_000, 300, 5000),
    "company":    TierLimits(20, 25, 100_000, 480, 10000),
    "free_trial": TierLimits(6, 8, 20_000, 120, 100),
}
ROLE_ITER_MULT = {"researcher": 1.5, "cio": 1.3, "content-chief": 0.8}

@dataclass
class CostMeter:
    tier: str; role_slug: str; org_id: str
    role_budget_cents: int = 0
    start_time: float = field(default_factory=time.time)
    iterations: int = 0; tool_calls: int = 0; tokens_used: int = 0
    _limits: TierLimits = field(init=False)
    def __post_init__(self): self._limits = TIER_LIMITS.get(self.tier, TIER_LIMITS["solo"])
    @property
    def effective_max_iterations(self): return max(3, int(self._limits.max_iterations * ROLE_ITER_MULT.get(self.role_slug, 1.0)))
    @property
    def elapsed(self): return time.time() - self.start_time
    @property
    def estimated_cost_cents(self): return max(1, int(self.tokens_used * 0.0005))
    def record_iteration(self): self.iterations += 1
    def record_tool_call(self, t=0): self.tool_calls += 1; self.tokens_used += t
    def record_tokens(self, t): self.tokens_used += t
    def check_limits(self):
        if self.iterations >= self.effective_max_iterations: return False, f"iteration_limit"
        if self.tool_calls >= self._limits.max_tool_calls: return False, "tool_call_limit"
        if self.tokens_used >= self._limits.max_tokens: return False, "token_limit"
        if self.elapsed >= self._limits.max_wall_clock_seconds: return False, "time_limit"
        if self.role_budget_cents > 0 and self.estimated_cost_cents >= self.role_budget_cents:
            return False, f"role_budget_limit ({self.estimated_cost_cents}c/{self.role_budget_cents}c)"
        return True, None
    def summary(self): return {"iterations": self.iterations, "tool_calls": self.tool_calls, "tokens_used": self.tokens_used, "elapsed_s": round(self.elapsed,1), "estimated_cost_cents": self.estimated_cost_cents}

async def check_daily_limit(org_id, tier):
    limits = TIER_LIMITS.get(tier, TIER_LIMITS["solo"])
    try:
        c = get_backend_client()
        r = await c.get(f"/api/internal/daily-cost/{org_id}", timeout=5)
        if r.status_code == 200:
            spent = r.json().get("estimated_cost_cents", 0)
            return spent < limits.daily_cost_ceiling_cents, max(0, limits.daily_cost_ceiling_cents - spent)
    except Exception as e:
        logger.warning(f"daily limit check failed for {org_id}: {type(e).__name__}",
            extra={"org_id": org_id})
        return False, 0

async def record_task_cost(org_id, tokens, tool_calls):
    try:
        c = get_backend_client()
        await c.post("/api/internal/record-cost",
            json={"org_id": org_id, "tokens": tokens, "tool_calls": tool_calls, "estimated_cost_cents": max(1, int(tokens*0.0005))},
            timeout=5)
    except Exception:
        pass
