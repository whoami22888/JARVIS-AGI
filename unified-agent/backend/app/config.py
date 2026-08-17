import os
from dataclasses import dataclass
from pathlib import Path

@dataclass(frozen=True)
class Settings:
    pairing_code: str
    token_secret: str
    data_dir: Path
    deepseek_api_key: str
    deepseek_base_url: str
    deepseek_model: str

def settings() -> Settings:
    data_dir = Path(os.getenv("DATA_DIR", "./data")).expanduser().resolve()
    data_dir.mkdir(parents=True, exist_ok=True)
    return Settings(
        pairing_code=os.getenv("PAIRING_CODE", "development-pairing-code"),
        token_secret=os.getenv("TOKEN_SECRET", "development-token-secret-change-me"),
        data_dir=data_dir,
        deepseek_api_key=os.getenv("DEEPSEEK_API_KEY", ""),
        deepseek_base_url=os.getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com").rstrip("/"),
        deepseek_model=os.getenv("DEEPSEEK_MODEL", "deepseek-reasoner"),
    )
