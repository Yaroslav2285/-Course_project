# LR #2: Modern Python
# LR #4: Async/Web
from typing import Any, Generic, TypeVar

from pydantic import BaseModel

T = TypeVar("T")


class ErrorDetail(BaseModel):
    code: str = "ERROR"
    detail: str = "An error occurred"
    fields: list[dict[str, str]] | None = None


class MetaResponse(BaseModel):
    total: int | None = None
    limit: int | None = None
    offset: int | None = None


class APIResponse(BaseModel, Generic[T]):
    data: T | None = None
    meta: MetaResponse = MetaResponse()
    errors: list[ErrorDetail] | None = None


def success_response(
    data: Any = None,
    total: int | None = None,
    limit: int | None = None,
    offset: int | None = None,
) -> dict:
    return {
        "data": data,
        "meta": {
            "total": total,
            "limit": limit,
            "offset": offset,
        },
        "errors": None,
    }


def error_response(
    code: str = "ERROR",
    detail: str = "An error occurred",
    fields: list[dict[str, str]] | None = None,
) -> dict:
    return {
        "data": None,
        "meta": {},
        "errors": [{"code": code, "detail": detail, "fields": fields}],
    }
