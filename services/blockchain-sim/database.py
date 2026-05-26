import json
import sqlite3
from pathlib import Path

from config import settings


def get_db_path() -> str:
    return settings.db_path


# LR #15: Параметризованные запросы (prepared statements) для SQLite
def init_db(db_path: str | None = None):
    path = db_path or get_db_path()
    conn = sqlite3.connect(path)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS blocks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            data TEXT NOT NULL
        )
        """)
    conn.commit()
    conn.close()


def save_chain(blocks: list[dict], db_path: str | None = None):
    path = db_path or get_db_path()
    conn = sqlite3.connect(path)
    conn.execute("DELETE FROM blocks")
    for block in blocks:
        conn.execute(
            "INSERT INTO blocks (data) VALUES (?)",
            (json.dumps(block),),
        )
    conn.commit()
    conn.close()


def load_chain(db_path: str | None = None) -> list[dict]:
    path = db_path or get_db_path()
    if not Path(path).exists():
        return []
    conn = sqlite3.connect(path)
    rows = conn.execute("SELECT data FROM blocks ORDER BY id").fetchall()
    conn.close()
    return [json.loads(row[0]) for row in rows]
