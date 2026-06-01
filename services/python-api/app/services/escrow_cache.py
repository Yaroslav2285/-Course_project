import json
from typing import Any

import structlog

from core.config import settings

logger = structlog.get_logger()

ESCROW_CACHE_TTL = 86400

_client: Any = None
_fallback_cache: dict[str, str] = {}
_fallback_data: dict[str, dict[str, Any]] = {}


async def _get_client():
    global _client
    if _client is not None:
        return _client
    try:
        import redis.asyncio as aioredis
        r = aioredis.from_url(settings.REDIS_URL, decode_responses=True, socket_connect_timeout=2)
        await r.ping()
        _client = r
        logger.info("redis_connected")
        return r
    except Exception as exc:
        logger.warning("redis_unavailable", reason=str(exc), msg="Falling back to in-memory cache")
        _client = False
        return None


async def close() -> None:
    global _client
    if _client is not None and _client is not False:
        try:
            await _client.close()
        except Exception:
            logger.warning("redis_close_error")
    _client = None
    _fallback_cache.clear()
    _fallback_data.clear()


async def escrow_exists(order_id: str) -> bool:
    return await get_escrow_id(order_id) is not None


async def _redis_op(r, key: str, op: str = "get", value=None, **kwargs):
    try:
        if op == "get":
            return await r.get(key)
        elif op == "setex":
            return await r.setex(key, kwargs.get("ttl", ESCROW_CACHE_TTL), value)
        elif op == "delete":
            return await r.delete(key)
    except RuntimeError:
        return None


async def cache_set(order_id: str, escrow_id: str, data: dict | None = None) -> None:
    r = await _get_client()
    if r is not None and r is not False:
        await _redis_op(r, f"escrow:order:{order_id}:id", "setex", escrow_id)
        if data:
            data_key = f"escrow:id:{escrow_id}:data"
            await _redis_op(r, data_key, "setex", json.dumps(data, default=str))
    _fallback_cache[order_id] = escrow_id
    if data:
        _fallback_data[escrow_id] = data


async def cache_get(order_id: str) -> dict[str, Any] | None:
    r = await _get_client()
    if r is not None and r is not False:
        escrow_id = await _redis_op(r, f"escrow:order:{order_id}:id")
        if escrow_id:
            data_key = f"escrow:id:{escrow_id}:data"
            raw = await _redis_op(r, data_key)
            if raw:
                try:
                    data = json.loads(raw)
                    return data
                except json.JSONDecodeError:
                    pass
            return {"escrow_id": escrow_id, "order_id": order_id}

    escrow_id = _fallback_cache.get(order_id)
    if not escrow_id:
        return None
    data = _fallback_data.get(escrow_id)
    if data:
        return data
    return {"escrow_id": escrow_id, "order_id": order_id}


async def get_escrow_id(order_id: str) -> str | None:
    r = await _get_client()
    if r is not None and r is not False:
        val = await _redis_op(r, f"escrow:order:{order_id}:id")
        if val:
            return val
    return _fallback_cache.get(order_id)


async def delete(order_id: str) -> None:
    r = await _get_client()
    if r is not None and r is not False:
        escrow_id = await _redis_op(r, f"escrow:order:{order_id}:id")
        await _redis_op(r, f"escrow:order:{order_id}:id", "delete")
        if escrow_id:
            await _redis_op(r, f"escrow:id:{escrow_id}:data", "delete")
    escrow_id = _fallback_cache.pop(order_id, None)
    if escrow_id:
        _fallback_data.pop(escrow_id, None)
