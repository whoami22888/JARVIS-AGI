from typing import Any, Literal
from pydantic import BaseModel, Field

class PairRequest(BaseModel):
    device_name: str = Field(min_length=1, max_length=80)
    pairing_code: str = Field(min_length=8, max_length=256)

class PairResponse(BaseModel):
    device_id: str
    token: str
    expires_in_seconds: int

class TaskRequest(BaseModel):
    prompt: str = Field(min_length=1, max_length=16000)
    model: str = "deepseek-reasoner"
    enabled_capabilities: list[str] = Field(default_factory=list, max_length=32)

class RegistryCreate(BaseModel):
    kind: Literal["tool", "skill", "mcp"]
    name: str = Field(min_length=1, max_length=100)
    definition: dict[str, Any] = Field(default_factory=dict)

class AutomationProposal(BaseModel):
    action: str = Field(min_length=1, max_length=100)
    target: str = Field(min_length=1, max_length=300)
    reason: str = Field(min_length=1, max_length=500)
    risk: Literal["low", "medium", "high"] = "high"
