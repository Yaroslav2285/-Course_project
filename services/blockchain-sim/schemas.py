from pydantic import BaseModel, Field
from typing import Any


class SubmitRequest(BaseModel):
    order_id: str = Field(..., description="ID of the escrow order")
    action: str = Field(..., description="Action performed on the order")
    data: dict[str, Any] = Field(default_factory=dict, description="Additional context")


class SubmitResponse(BaseModel):
    block_index: int
    tx_hash: str
    prev_hash: str


class VerifyResponse(BaseModel):
    valid: bool
    blocks_count: int


class BlockResponse(BaseModel):
    index: int
    timestamp: str
    prev_hash: str
    transactions: list[dict]
    nonce: int
    hash: str


class HealthResponse(BaseModel):
    status: str
