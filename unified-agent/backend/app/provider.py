import httpx
from .config import Settings

class ProviderError(RuntimeError):
    pass

async def complete(settings: Settings, prompt: str, model: str) -> str:
    if not settings.deepseek_api_key:
        raise ProviderError("No authorized DeepSeek-compatible provider key is configured on the gateway.")
    url = settings.deepseek_base_url + "/chat/completions"
    payload = {
        "model": model or settings.deepseek_model,
        "messages": [
            {"role": "system", "content": "You are WhiteRabbit Neo Unified Agent. Use only the capabilities explicitly enabled by the user. Propose device actions; never claim to have executed them."},
            {"role": "user", "content": prompt},
        ],
        "stream": False,
    }
    try:
        async with httpx.AsyncClient(timeout=45.0) as client:
            response = await client.post(url, json=payload, headers={"Authorization": f"Bearer {settings.deepseek_api_key}"})
            response.raise_for_status()
            body = response.json()
            return str(body["choices"][0]["message"]["content"])
    except (httpx.HTTPError, KeyError, IndexError, TypeError) as exc:
        raise ProviderError(f"Provider request failed: {exc}") from exc
