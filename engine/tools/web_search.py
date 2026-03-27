import os, httpx
SERPER_KEY = os.getenv("SERPER_API_KEY", "")

async def search(query, num_results=5):
    if not SERPER_KEY: return [{"title":"Search unavailable","url":"","snippet":"SERPER_API_KEY not set"}]
    try:
        async with httpx.AsyncClient() as c:
            r = await c.post("https://google.serper.dev/search",
                headers={"X-API-KEY":SERPER_KEY,"Content-Type":"application/json"},
                json={"q":query,"num":num_results}, timeout=10)
            if r.status_code != 200:
                return [{"title":"Search error","url":"","snippet":f"Serper returned {r.status_code}"}]
            return [{"title":i.get("title",""),"url":i.get("link",""),"snippet":i.get("snippet","")}
                for i in r.json().get("organic",[])[:num_results]]
    except Exception as e:
        return [{"title":"Search error","url":"","snippet":f"Search failed: {type(e).__name__}"}]
