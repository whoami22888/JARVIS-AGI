"""
AI Account Manager - Unified interface for managing multiple AI provider subscriptions.

Supports: OpenRouter, DeepInfra, HuggingFace, DeepSeek, OpenGPT, Local Ollama, and more.
Provides fallback chain, account validation, and model selection.
"""

import os
import json
import requests
from dataclasses import dataclass, field, asdict
from typing import Dict, List, Optional, Any
from pathlib import Path
from dotenv import load_dotenv

load_dotenv()

CONFIG_PATH = Path("ASSETS/ai_accounts.json")


@dataclass
class ProviderAccount:
    """Configuration for a single AI provider account."""
    name: str
    enabled: bool = True
    priority: int = 10
    api_key: str = ""
    base_url: str = ""
    model: str = ""
    extra: Dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> Dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: Dict) -> "ProviderAccount":
        return cls(**data)


class AIAccountManager:
    """Manages AI provider accounts and provides fallback chain."""

    DEFAULT_FALLBACK_CHAIN = [
        "local_ollama",
        "openrouter",
        "deepinfra",
        "huggingface",
        "deepseek",
        "opengpt",
        "liaobots",
    ]

    PROVIDER_CONFIGS = {
        "openrouter": {
            "env_key": "OPENROUTER",
            "default_model": "meta-llama/llama-3-8b-instruct:free",
            "base_url": "https://openrouter.ai/api/v1",
            "test_endpoint": "/models",
            "free_models": [
                "meta-llama/llama-3-8b-instruct:free",
                "mistralai/mistral-7b-instruct:free",
                "huggingfaceh4/zephyr-7b-beta",
                "openchat/openchat-7b",
            ],
        },
        "deepinfra": {
            "env_key": "DEEPINFRA",
            "default_model": "meta-llama/Meta-Llama-3.1-405B-Instruct",
            "base_url": "https://api.deepinfra.com/v1/openai",
            "test_endpoint": "/models",
            "free_models": [],
        },
        "huggingface": {
            "env_key": "HUGGING_FACE_READ",
            "default_model": "meta-llama/Meta-Llama-3-70B-Instruct",
            "base_url": "https://api-inference.huggingface.co",
            "test_endpoint": "/models/meta-llama/Meta-Llama-3-70B-Instruct",
            "free_models": [],
        },
        "deepseek": {
            "env_key": "DEEPSEEK",
            "default_model": "deepseek-reasoner",
            "base_url": "https://api.deepseek.com/v1",
            "test_endpoint": "/models",
            "free_models": [],
        },
        "opengpt": {
            "env_key": "OPENGPT_ASSISTANT_ID",
            "default_model": "gpt-3.5-turbo",
            "base_url": "https://opengpts-example-vz4y4ooboq-uc.a.run.app",
            "test_endpoint": "",
            "free_models": [],
        },
        "liaobots": {
            "env_key": "LIAOBOTS_X_AUTH_CODE",
            "default_model": "gpt-3.5-turbo",
            "base_url": "https://api.liaobots.com",
            "test_endpoint": "",
            "free_models": [],
        },
        "local_ollama": {
            "env_key": "",
            "default_model": "llama3.1:70b",
            "base_url": "http://127.0.0.1:11434/v1",
            "test_endpoint": "/models",
            "free_models": ["any local model"],
        },
    }

    def __init__(self, config_path: Path = CONFIG_PATH):
        self.config_path = config_path
        self.accounts: Dict[str, ProviderAccount] = {}
        self.fallback_chain: List[str] = self.DEFAULT_FALLBACK_CHAIN.copy()
        self._load_config()

    def _load_config(self):
        """Load accounts from config file and environment variables."""
        # Start with environment variables
        for provider_id, config in self.PROVIDER_CONFIGS.items():
            api_key = ""
            if config["env_key"]:
                api_key = os.environ.get(config["env_key"], "")

            self.accounts[provider_id] = ProviderAccount(
                name=provider_id,
                enabled=bool(api_key) or provider_id == "local_ollama",
                priority=10,
                api_key=api_key,
                base_url=config["base_url"],
                model=config["default_model"],
            )

        # Override with config file if exists
        if self.config_path.exists():
            try:
                with open(self.config_path, "r") as f:
                    data = json.load(f)

                for provider_id, account_data in data.get("accounts", {}).items():
                    if provider_id in self.accounts:
                        self.accounts[provider_id] = ProviderAccount.from_dict(account_data)
                    else:
                        self.accounts[provider_id] = ProviderAccount.from_dict(account_data)

                self.fallback_chain = data.get("fallback_chain", self.DEFAULT_FALLBACK_CHAIN)
            except Exception as e:
                print(f"Warning: Failed to load AI accounts config: {e}")

    def save_config(self):
        """Save current accounts to config file."""
        self.config_path.parent.mkdir(parents=True, exist_ok=True)
        data = {
            "version": 1,
            "accounts": {k: v.to_dict() for k, v in self.accounts.items()},
            "fallback_chain": self.fallback_chain,
        }
        with open(self.config_path, "w") as f:
            json.dump(data, f, indent=2)

    def add_account(self, provider_id: str, api_key: str, **kwargs) -> bool:
        """Add or update an account for a provider."""
        if provider_id not in self.PROVIDER_CONFIGS and provider_id != "custom":
            print(f"Unknown provider: {provider_id}")
            return False

        config = self.PROVIDER_CONFIGS.get(provider_id, {})
        self.accounts[provider_id] = ProviderAccount(
            name=provider_id,
            enabled=True,
            priority=kwargs.get("priority", 10),
            api_key=api_key,
            base_url=kwargs.get("base_url", config.get("base_url", "")),
            model=kwargs.get("model", config.get("default_model", "")),
            extra=kwargs.get("extra", {}),
        )
        self.save_config()
        return True

    def remove_account(self, provider_id: str) -> bool:
        """Disable and remove an account."""
        if provider_id in self.accounts:
            self.accounts[provider_id].enabled = False
            self.accounts[provider_id].api_key = ""
            self.save_config()
            return True
        return False

    def test_provider(self, provider_id: str) -> Dict[str, Any]:
        """Test if a provider is accessible."""
        if provider_id not in self.accounts:
            return {"success": False, "error": "Provider not configured"}

        account = self.accounts[provider_id]
        if not account.enabled or not account.api_key:
            if provider_id != "local_ollama":
                return {"success": False, "error": "Provider disabled or no API key"}

        config = self.PROVIDER_CONFIGS.get(provider_id, {})
        test_url = account.base_url + config.get("test_endpoint", "")

        try:
            headers = {}
            if account.api_key:
                headers["Authorization"] = f"Bearer {account.api_key}"

            response = requests.get(test_url, headers=headers, timeout=10)
            if response.status_code == 200:
                return {"success": True, "models": response.json() if "models" in test_url else "OK"}
            return {"success": False, "error": f"HTTP {response.status_code}: {response.text[:200]}"}
        except Exception as e:
            return {"success": False, "error": str(e)}

    def test_all(self) -> Dict[str, Dict]:
        """Test all enabled providers."""
        results = {}
        for provider_id, account in self.accounts.items():
            if account.enabled:
                results[provider_id] = self.test_provider(provider_id)
        return results

    def get_best_provider(self, prefer_free: bool = True) -> Optional[ProviderAccount]:
        """Get the highest-priority working provider."""
        for provider_id in self.fallback_chain:
            if provider_id in self.accounts:
                account = self.accounts[provider_id]
                if account.enabled:
                    if provider_id == "local_ollama" or account.api_key:
                        return account
        return None

    def get_free_model_provider(self) -> Optional[ProviderAccount]:
        """Get a provider with free tier models."""
        for provider_id in ["openrouter", "huggingface", "local_ollama"]:
            if provider_id in self.accounts:
                account = self.accounts[provider_id]
                if account.enabled and (provider_id == "local_ollama" or account.api_key):
                    return account
        return None

    def list_accounts(self) -> List[Dict]:
        """List all accounts with status."""
        result = []
        for provider_id, account in self.accounts.items():
            config = self.PROVIDER_CONFIGS.get(provider_id, {})
            result.append({
                "provider": provider_id,
                "enabled": account.enabled,
                "priority": account.priority,
                "has_key": bool(account.api_key) or provider_id == "local_ollama",
                "model": account.model,
                "base_url": account.base_url,
                "free_tier": provider_id in ["openrouter", "huggingface", "local_ollama"],
            })
        return sorted(result, key=lambda x: x["priority"])

    def set_fallback_chain(self, chain: List[str]):
        """Update the fallback priority chain."""
        self.fallback_chain = chain
        self.save_config()

    def get_client_config(self, provider_id: str) -> Optional[Dict]:
        """Get configuration dict for initializing a provider client."""
        if provider_id not in self.accounts:
            return None
        account = self.accounts[provider_id]
        if not account.enabled:
            return None
        return {
            "api_key": account.api_key,
            "base_url": account.base_url,
            "model": account.model,
            **account.extra,
        }


def main():
    """CLI entry point for account management."""
    import sys

    manager = AIAccountManager()

    if len(sys.argv) < 2:
        print("Usage: python -m TOOLS.AIAccountManager <command> [args]")
        print("Commands:")
        print("  list                    - List all accounts")
        print("  test [provider]         - Test provider(s)")
        print("  add <provider> <key>    - Add account (provider: openrouter, deepinfra, etc.)")
        print("  remove <provider>       - Disable account")
        print("  fallback <list>         - Set fallback chain (comma-separated)")
        print("  best                    - Show best available provider")
        return

    cmd = sys.argv[1]

    if cmd == "list":
        for acc in manager.list_accounts():
            status = "✓" if acc["enabled"] and acc["has_key"] else "✗"
            free = " (FREE)" if acc["free_tier"] else ""
            print(f"  {status} {acc['provider']}: model={acc['model']}, priority={acc['priority']}{free}")

    elif cmd == "test":
        if len(sys.argv) > 2:
            provider = sys.argv[2]
            result = manager.test_provider(provider)
            print(f"{provider}: {result}")
        else:
            results = manager.test_all()
            for provider, result in results.items():
                status = "✓" if result["success"] else "✗"
                print(f"  {status} {provider}: {result.get('error', 'OK')}")

    elif cmd == "add":
        if len(sys.argv) < 4:
            print("Usage: add <provider> <api_key> [model]")
            return
        provider = sys.argv[2]
        api_key = sys.argv[3]
        model = sys.argv[4] if len(sys.argv) > 4 else ""
        manager.add_account(provider, api_key, model=model)
        print(f"Added {provider}")

    elif cmd == "remove":
        if len(sys.argv) < 3:
            print("Usage: remove <provider>")
            return
        manager.remove_account(sys.argv[2])
        print(f"Removed {sys.argv[2]}")

    elif cmd == "fallback":
        if len(sys.argv) < 3:
            print("Current fallback chain:", " -> ".join(manager.fallback_chain))
            return
        chain = sys.argv[2].split(",")
        manager.set_fallback_chain(chain)
        print(f"Fallback chain updated: {' -> '.join(chain)}")

    elif cmd == "best":
        best = manager.get_best_provider()
        if best:
            print(f"Best provider: {best.name} (model: {best.model})")
        else:
            print("No working provider found")

    else:
        print(f"Unknown command: {cmd}")


if __name__ == "__main__":
    main()