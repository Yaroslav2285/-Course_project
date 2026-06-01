"""Assign categories to all services that don't have one yet."""

import asyncio
import os

from sqlalchemy import text
from sqlalchemy.ext.asyncio import create_async_engine

CATEGORIES = [
    'Electronics', 'Clothing', 'Home & Garden', 'Books', 'Sports',
    'Toys', 'Health', 'Beauty', 'Automotive', 'Food',
    'Music', 'Office', 'Pet Supplies', 'Baby', 'Jewelry', 'Other',
]


async def main():
    engine = create_async_engine(os.environ['DB_URL'])
    async with engine.begin() as conn:
        result = await conn.execute(
            text("SELECT id, title FROM services WHERE category IS NULL")
        )
        rows = result.fetchall()
        print(f"Found {len(rows)} services without category")

        for row in rows:
            h = hash(row.title) % len(CATEGORIES)
            cat = CATEGORIES[h]
            await conn.execute(
                text("UPDATE services SET category = :cat WHERE id = :id"),
                {"cat": cat, "id": row.id},
            )
            print(f"  {row.id} | {row.title} → {cat}")

    await engine.dispose()
    print("Done.")


if __name__ == "__main__":
    asyncio.run(main())
