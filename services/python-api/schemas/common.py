# LR #2: Modern Python
# LR #4: Async/Web
from typing import Generic, TypeVar

from pydantic import BaseModel

T = TypeVar("T")


class PaginationMeta(BaseModel):
    total: int | None = None
    limit: int | None = None
    offset: int | None = None


class ErrorField(BaseModel):
    field: str | None = None
    message: str | None = None


class ErrorDetail(BaseModel):
    code: str = "ERROR"
    detail: str = "An error occurred"
    fields: list[ErrorField] | None = None


class APIResponse(BaseModel, Generic[T]):
    data: T | None = None
    meta: PaginationMeta = PaginationMeta()
    errors: list[ErrorDetail] | None = None


class SuccessMessage(BaseModel):
    message: str
