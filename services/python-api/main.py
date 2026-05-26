# LR #2: Modern Python
# LR #4: Async/Web
import uuid
from contextlib import asynccontextmanager

import structlog
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from api.v1 import router as v1_router
from api.v1.ui import mount_static, router as ui_router
from core.config import settings
from core.db import engine
from core.exceptions import (
    http_exception_handler,
    unhandled_exception_handler,
    validation_exception_handler,
)
from fastapi.exceptions import RequestValidationError
from starlette.exceptions import HTTPException as StarletteHTTPException


structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.stdlib.add_log_level,
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.stdlib.BoundLogger,
    context_class=dict,
    logger_factory=structlog.PrintLoggerFactory(),
    cache_logger_on_first_use=True,
)

logger = structlog.get_logger()


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("Starting Service Marketplace API")
    from models.base import Base
    import models.users  # noqa: F401
    import models.services  # noqa: F401
    import models.orders  # noqa: F401
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    logger.info("Database tables created")
    yield
    logger.info("Shutting down Service Marketplace API")
    await engine.dispose()


app = FastAPI(
    title=settings.API_TITLE,
    version=settings.API_VERSION,
    root_path=settings.API_ROOT_PATH,
    docs_url=settings.API_DOCS_URL,
    redoc_url=settings.API_REDOC_URL,
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origin_list,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.middleware("http")
async def add_x_request_id(request: Request, call_next):
    request_id = request.headers.get("X-Request-ID", str(uuid.uuid4()))
    response = await call_next(request)
    response.headers["X-Request-ID"] = request_id
    return response


@app.middleware("http")
async def log_requests(request: Request, call_next):
    logger.info("request", method=request.method, path=request.url.path)
    response = await call_next(request)
    logger.info(
        "response",
        status_code=response.status_code,
        method=request.method,
        path=request.url.path,
    )
    return response


app.add_exception_handler(RequestValidationError, validation_exception_handler)
app.add_exception_handler(StarletteHTTPException, http_exception_handler)
app.add_exception_handler(Exception, unhandled_exception_handler)


@app.get("/health")
async def health():
    return {"status": "healthy"}


mount_static(app)
app.include_router(ui_router)
app.include_router(v1_router)
