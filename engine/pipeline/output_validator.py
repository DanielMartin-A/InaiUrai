"""Output Validator: Validates agent output before delivery."""
import re
from typing import NamedTuple

LEAK_PATTERNS = {
    "uuid": re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", re.I),
    "api_key": re.compile(r"(sk-ant-|sk_test_|sk_live_|whsec_|Bearer\s+)[A-Za-z0-9\-_]{10,}", re.I),
    "internal_path": re.compile(r"(\/app\/|\/home\/claude|engine:8000|backend:8080|localhost:\d{4})", re.I),
    "db_url": re.compile(r"postgres(ql)?://[^\s]+", re.I),
}
FIN_DISC = "\n\n---\n*This is for informational purposes only and does not constitute financial advice. Consult a licensed advisor.*"
LEGAL_DISC = "\n\n---\n*This is for general reference only and does not constitute legal advice. Consult a licensed attorney.*"
FIN_IND = [re.compile(r"\b(invest|tax|revenue|profit|valuation|EBITDA|burn\s*rate)\b", re.I)]
LEG_IND = [re.compile(r"\b(contract|liability|compliance|GDPR|patent|litigation)\b", re.I)]

class ValidationResult(NamedTuple):
    output: str; is_valid: bool; issues: list; auto_fixed: list

def validate_output(output, role_slug, org_id):
    issues, fixed, cleaned = [], [], output
    for lt, pat in LEAK_PATTERNS.items():
        matches = pat.findall(cleaned)
        if matches:
            if lt == "uuid" and len(matches) <= 2: continue
            issues.append(f"leak:{lt}"); cleaned = pat.sub("[REDACTED]", cleaned); fixed.append(f"redacted {lt}")
    if role_slug in ("cfo","cio","cdo") and any(p.search(cleaned) for p in FIN_IND):
        if "not constitute financial advice" not in cleaned.lower(): cleaned += FIN_DISC; fixed.append("financial disclaimer")
    if role_slug == "general-counsel" and any(p.search(cleaned) for p in LEG_IND):
        if "not constitute legal advice" not in cleaned.lower(): cleaned += LEGAL_DISC; fixed.append("legal disclaimer")
    return ValidationResult(cleaned, len(issues)==0, issues, fixed)
