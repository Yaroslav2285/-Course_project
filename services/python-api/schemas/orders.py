# LR #2: Modern Python
# LR #4: Async/Web
# LR #3: Financial precision — condecimal for amounts
from datetime import datetime
from decimal import Decimal
from uuid import UUID

from pydantic import BaseModel, Field, condecimal


class OrderCreate(BaseModel):
    service_id: UUID
    buyer_id: UUID
    seller_id: UUID
    amount: condecimal(max_digits=19, decimal_places=4) = Field(..., gt=Decimal("0"))
    notes: str | None = None


class OrderStatusUpdate(BaseModel):
    status: str = Field(
        ...,
        pattern=r"^(pending|funded|released|cancelled|disputed)$",
    )


class OrderRead(BaseModel):
    id: UUID
    service_id: UUID
    buyer_id: UUID
    seller_id: UUID
    amount: Decimal
    status: str
    notes: str | None
    created_at: datetime
    updated_at: datetime

    model_config = {"from_attributes": True}


class OrderList(BaseModel):
    items: list[OrderRead]
    total: int
