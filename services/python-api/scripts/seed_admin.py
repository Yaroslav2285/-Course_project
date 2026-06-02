import asyncio
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from passlib.context import CryptContext
from sqlalchemy import text
from sqlalchemy.ext.asyncio import create_async_engine

pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")
DB_URL = os.getenv("DB_URL", "postgresql+asyncpg://app_user:ChangeMe123!@localhost:5432/app_db")


async def main():
    engine = create_async_engine(DB_URL, echo=True)

    async with engine.connect() as conn:
        result = await conn.execute(
            text("SELECT id FROM users WHERE email = 'admin@marketplace.local'")
        )
        row = result.fetchone()
        if row:
            print("Admin user already exists (id=%s)" % row[0])
        else:
            hashed = pwd_context.hash("admin123!")
            await conn.execute(
                text(
                    "INSERT INTO users (id, email, hashed_password, role, created_at, updated_at) "
                    "VALUES (gen_random_uuid(), 'admin@marketplace.local', :pw, 'admin', now(), now())"
                ),
                {"pw": hashed},
            )
            await conn.commit()
            print("Created admin user: admin@marketplace.local / admin123!")

    await engine.dispose()


if __name__ == "__main__":
    asyncio.run(main())
