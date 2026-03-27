import httpx
from urllib.parse import urlparse
import ipaddress
import socket
from bs4 import BeautifulSoup

BLOCKED_HOSTS = {"localhost", "127.0.0.1", "0.0.0.0", "169.254.169.254",
    "metadata.google.internal", "engine", "backend", "postgres", "redis"}
BLOCKED_SCHEMES = {"file", "ftp", "gopher", "data", "javascript"}

def _is_safe_url(url: str) -> tuple[bool, str]:
    try:
        parsed = urlparse(url)
        if not parsed.scheme or not parsed.hostname:
            return False, "Invalid URL"
        if parsed.scheme.lower() in BLOCKED_SCHEMES:
            return False, f"Blocked scheme: {parsed.scheme}"
        hostname = parsed.hostname.lower()
        if hostname in BLOCKED_HOSTS:
            return False, f"Blocked host: {hostname}"
        try:
            for info in socket.getaddrinfo(hostname, None):
                ip = ipaddress.ip_address(info[4][0])
                if ip.is_private or ip.is_loopback or ip.is_link_local or ip.is_reserved:
                    return False, "Blocked: resolves to private IP"
        except socket.gaierror:
            pass
        return True, ""
    except Exception:
        return False, "URL validation error"

async def read_url(url):
    safe, reason = _is_safe_url(url)
    if not safe:
        return f"Blocked: {reason}"
    try:
        async with httpx.AsyncClient(follow_redirects=True, max_redirects=3) as c:
            r = await c.get(url, timeout=10, headers={"User-Agent": "InaiUrai/5.0"})
            if r.status_code >= 400:
                return f"HTTP error: {r.status_code}"
            ct = r.headers.get("content-type", "")
            if "text/html" not in ct and "text/plain" not in ct:
                return f"Skipped non-text content: {ct[:50]}"
            soup = BeautifulSoup(r.text, "html.parser")
            for tag in soup(["script","style","nav","footer","header"]): tag.decompose()
            return soup.get_text(separator="\n", strip=True)[:3000]
    except httpx.TooManyRedirects:
        return "Blocked: too many redirects"
    except Exception as e: return f"Error: {type(e).__name__}"
