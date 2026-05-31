# LR #2: Modern Python
# LR #4: Async/Web
# LR #3: Financial precision — condecimal for amounts
from datetime import datetime
from decimal import Decimal
from uuid import UUID

from pydantic import BaseModel, Field, condecimal


class TopUpRequest(BaseModel):
    amount: condecimal(max_digits=19, decimal_places=4) = Field(..., gt=Decimal("0"))


class WalletRead(BaseModel):
    id: UUID
    user_id: UUID
    balance: Decimal

    model_config = {"from_attributes": True}


class TransactionRead(BaseModel):
    id: UUID
    wallet_id: UUID
    type: str
    amount: Decimal
    status: str
    reference_id: UUID | None = None
    description: str | None = None
    created_at: datetime

    model_config = {"from_attributes": True}


class TransactionList(BaseModel):
    items: list[TransactionRead]
    total: int
