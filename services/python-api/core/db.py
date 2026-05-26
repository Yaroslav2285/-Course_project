# LR #2: Modern Python
# LR #4: Async/Web
import datetime

from sqlalchemy import event
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from core.config import settings

_is_sqlite = settings.DB_URL.startswith("sqlite")


def _register_sqlite_now(dbapi_connection, connection_record):
    dbapi_connection.create_function(
        "now", 0, lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
    )


engine = create_async_engine(
    settings.DB_URL,
    echo=settings.DB_ECHO,
    **(dict(pool_size=5, max_overflow=10) if not _is_sqlite else dict(connect_args={"check_same_thread": False})),
)

if _is_sqlite:
    event.listen(engine.sync_engine, "connect", _register_sqlite_now)

async_session_factory = async_sessionmaker(
    engine,
    class_=AsyncSession,
    expire_on_commit=False,
)


async def get_db() -> AsyncSession:
    async with async_session_factory() as session:
        try:
            yield session
            await session.commit()
        except Exception:
            await session.rollback()
            raise
