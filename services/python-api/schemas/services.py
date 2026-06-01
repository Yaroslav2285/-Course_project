# LR #2: Modern Python
# LR #4: Async/Web
# LR #3: Financial precision — condecimal for prices
from datetime import datetime
from decimal import Decimal
from uuid import UUID

from pydantic import BaseModel, Field, condecimal


class ServiceCreate(BaseModel):
    title: str = Field(..., min_length=1, max_length=200)
    description: str | None = None
    category: str | None = Field(None, max_length=50)
    discount: int | None = Field(None, ge=0, le=100)
    price: condecimal(max_digits=19, decimal_places=4) = Field(..., gt=Decimal("0"))


class ServiceUpdate(BaseModel):
    title: str | None = Field(None, min_length=1, max_length=200)
    description: str | None = None
    category: str | None = Field(None, max_length=50)
    discount: int | None = Field(None, ge=0, le=100)
    price: condecimal(max_digits=19, decimal_places=4) | None = Field(
        None, gt=Decimal("0")
    )
    status: str | None = Field(None, pattern=r"^(draft|published|archived)$")


class ServiceRead(BaseModel):
    id: UUID
    provider_id: UUID
    provider_email: str | None = None
    title: str
    description: str | None
    category: str | None = None
    discount: int | None = None
    price: Decimal
    status: str
    created_at: datetime
    updated_at: datetime

    model_config = {"from_attributes": True}


class ServiceList(BaseModel):
    items: list[ServiceRead]
    total: int
