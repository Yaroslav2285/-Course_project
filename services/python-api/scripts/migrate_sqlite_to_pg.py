"""
LR #4: Async/Web — One-time data migration from SQLite to PostgreSQL.

Usage:
    python scripts/migrate_sqlite_to_pg.py

Requires:
    - marketplace.db (SQLite) in the project root
    - PostgreSQL running on localhost:5432 with credentials from .env
"""

import asyncio
import sys
from datetime import datetime, timezone
from pathlib import Path
from uuid import UUID

import aiosqlite
from sqlalchemy import DateTime, text
from sqlalchemy.dialects.postgresql import UUID as PG_UUID
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import create_async_engine

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from models.orders import Order
from models.services import Service
from models.users import User
from models.wallet import Wallet, Transaction

SQLITE_PATH = Path(__file__).resolve().parent.parent / "marketplace.db"
PG_URL = "postgresql+asyncpg://app_user:ChangeMe123!@localhost:5432/app_db"

TABLE_ORDER = [
    ("users", User),
    ("services", Service),
    ("orders", Order),
    ("wallets", Wallet),
    ("transactions", Transaction),
]


def _needs_uuid(col_type) -> bool:
    return isinstance(col_type, PG_UUID)


def _needs_datetime(col_type) -> bool:
    return isinstance(col_type, DateTime)


def _convert_value(value, col_type):
    if value is None:
        return None
    if _needs_uuid(col_type):
        return UUID(value) if isinstance(value, str) else value
    if _needs_datetime(col_type):
        if isinstance(value, str):
            dt = datetime.fromisoformat(value)
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return dt
        return value
    return value


async def migrate():
    if not SQLITE_PATH.exists():
        print(f"SQLite database not found at {SQLITE_PATH}")
        print("Nothing to migrate.")
        return

    print(f"Reading data from {SQLITE_PATH} ...")
    sqlite_conn = await aiosqlite.connect(str(SQLITE_PATH))
    sqlite_conn.row_factory = aiosqlite.Row

    engine = create_async_engine(PG_URL)

    async with engine.begin() as conn:
        for table_name, _ in reversed(TABLE_ORDER):
            await conn.execute(text(f"DELETE FROM {table_name}"))  # nosec B608
            print(f"  Cleared {table_name}")

    async with engine.connect() as conn:
        for table_name, model in TABLE_ORDER:
            cursor = await sqlite_conn.execute(f"SELECT * FROM {table_name}")  # nosec B608
            rows = await cursor.fetchall()
            if not rows:
                print(f"  {table_name}: 0 rows (skipping)")
                continue

            columns = [desc[0] for desc in cursor.description]
            col_names = ", ".join(columns)
            placeholders = ", ".join(f":{col}" for col in columns)

            pg_cols = {c.name: c.type for c in model.__table__.columns}
            skipped = 0

            for row in rows:
                data = dict(zip(columns, row))
                for k, v in data.items():
                    if k in pg_cols:
                        data[k] = _convert_value(v, pg_cols[k])
                try:
                    async with conn.begin_nested():
                        await conn.execute(
                            text(f"INSERT INTO {table_name} ({col_names}) VALUES ({placeholders}) ON CONFLICT DO NOTHING"),  # nosec B608
                            data,
                        )
                except IntegrityError:
                    skipped += 1

            await conn.commit()

            migrated = len(rows) - skipped
            print(f"  {table_name}: {migrated} rows migrated", end="")
            if skipped:
                print(f" ({skipped} skipped due to foreign key violations)")
            else:
                print()

    await sqlite_conn.close()
    await engine.dispose()
    print("\nMigration complete!")


if __name__ == "__main__":
    asyncio.run(migrate())
