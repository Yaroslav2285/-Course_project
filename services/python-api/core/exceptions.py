# LR #2: Modern Python
# LR #4: Async/Web
import logging

from fastapi import HTTPException, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException


class AppHTTPException(HTTPException):
    def __init__(
        self,
        status_code: int,
        detail: str = "An error occurred",
        error_code: str | None = None,
    ):
        super().__init__(status_code=status_code, detail=detail)
        self.error_code = error_code


class NotFoundException(AppHTTPException):
    def __init__(self, detail: str = "Resource not found"):
        super().__init__(status_code=404, detail=detail, error_code="NOT_FOUND")


class DuplicateException(AppHTTPException):
    def __init__(self, detail: str = "Resource already exists"):
        super().__init__(status_code=409, detail=detail, error_code="DUPLICATE")


class UnauthorizedException(AppHTTPException):
    def __init__(self, detail: str = "Not authenticated"):
        super().__init__(status_code=401, detail=detail, error_code="UNAUTHORIZED")


class ForbiddenException(AppHTTPException):
    def __init__(self, detail: str = "Forbidden"):
        super().__init__(status_code=403, detail=detail, error_code="FORBIDDEN")


class BadRequestException(AppHTTPException):
    def __init__(self, detail: str = "Bad request"):
        super().__init__(status_code=400, detail=detail, error_code="BAD_REQUEST")


async def validation_exception_handler(
    request: Request, exc: RequestValidationError
) -> JSONResponse:
    errors = [
        {
            "field": ".".join(str(loc) for loc in err.get("loc", [])),
            "message": err["msg"],
        }
        for err in exc.errors()
    ]
    return JSONResponse(
        status_code=422,
        content={
            "data": None,
            "meta": {},
            "errors": [
                {
                    "code": "VALIDATION_ERROR",
                    "detail": "Request validation failed",
                    "fields": errors,
                }
            ],
        },
    )


async def http_exception_handler(
    request: Request, exc: StarletteHTTPException
) -> JSONResponse:
    return JSONResponse(
        status_code=exc.status_code,
        content={
            "data": None,
            "meta": {},
            "errors": [
                {
                    "code": getattr(exc, "error_code", "HTTP_ERROR"),
                    "detail": exc.detail,
                }
            ],
        },
    )


async def unhandled_exception_handler(
    request: Request, exc: Exception
) -> JSONResponse:
    logging.getLogger(__name__).exception(
        "Unhandled exception: %s %s", request.method, request.url.path
    )
    return JSONResponse(
        status_code=500,
        content={
            "data": None,
            "meta": {},
            "errors": [{"code": "INTERNAL_ERROR", "detail": "Internal server error"}],
        },
    )
