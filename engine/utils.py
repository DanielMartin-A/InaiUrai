import json, re

def parse_json_response(text: str) -> dict:
    cleaned = text.strip()
    cleaned = re.sub(r"^`(?:json)?\s*", "", cleaned)
    cleaned = re.sub(r"\s*`$", "", cleaned)
    return json.loads(cleaned)
