import asyncio
from core.db import async_session_factory
from sqlalchemy import text

async def c():
    async with async_session_factory() as s:
        r = await s.execute(text("SELECT id, status FROM orders WHERE status = 'disputed'"))
        rows = r.fetchall()
        for row in rows:
            print(row)
        print('Total:', len(rows))

asyncio.run(c())
