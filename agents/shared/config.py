import os

from dotenv import load_dotenv

load_dotenv()

ROUTER_URL: str = os.environ.get("ROUTER_URL", "http://localhost:8080")
ANTHROPIC_API_KEY: str = os.environ.get("ANTHROPIC_API_KEY", "")
AGENT_PORT: int = int(os.environ.get("AGENT_PORT", "8001"))
