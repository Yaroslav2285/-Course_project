# LR #2: Modern Python
# LR #4: Async/Web
import asyncio
import os
import time
from uuid import UUID

import httpx
import structlog
from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from core.config import settings
from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException
from core.responses import success_response
from models.orders import Order, OrderStatus
from repositories.orders import OrderRepository
from schemas.orders import OrderCreate, OrderRead, OrderStatusUpdate
from schemas.users import UserRead

router = APIRouter()

_blockchain_cache: dict[str, tuple[bool, float]] = {}
_CACHE_TTL = 300  # 5 minutes

async def _check_blockchain_audit(order_id: str) -> bool:
    if os.environ.get("TESTING"):
        return False
    str_id = str(order_id)
    now = time.monotonic()
    if str_id in _blockchain_cache:
        val, ts = _blockchain_cache[str_id]
        if now - ts < _CACHE_TTL:
            return val
    base_url = settings.BLOCKCHAIN_SIM_BASE_URL.rstrip("/")
    try:
        async with httpx.AsyncClient(timeout=httpx.Timeout(5.0, connect=2.0)) as client:
            resp = await asyncio.wait_for(
                client.get(f"{base_url}/v1/chain/audit/{str_id}"),
                timeout=3.0,
            )
            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list):
                    result = len(data) > 0
                elif isinstance(data, dict):
                    result = len(data.get("blocks", [])) > 0
                else:
                    result = False
                _blockchain_cache[str_id] = (result, now)
                return result
    except httpx.ConnectError:
        structlog.get_logger().warning("blockchain_audit_connect_failed", order_id=str_id)
    except httpx.TimeoutException:
        structlog.get_logger().warning("blockchain_audit_timeout", order_id=str_id)
    except asyncio.TimeoutError:
        structlog.get_logger().warning("blockchain_audit_asyncio_timeout", order_id=str_id)
    except Exception:
        structlog.get_logger().exception("blockchain_audit_unexpected_error", order_id=str_id)
    _blockchain_cache[str_id] = (False, now)
    return False


async def _order_to_dict(order: Order) -> dict:
    d = OrderRead.model_validate(order).model_dump()
    d["seller_email"] = order.seller.email if order.seller else None
    d["buyer_email"] = order.buyer.email if order.buyer else None
    if order.status not in ("pending", "created"):
        d["blockchain_verified"] = await _check_blockchain_audit(str(order.id))
    return d


@router.get("/debug", response_model=dict)
async def debug_orders(
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    result = await session.execute(select(Order))
    all_orders = result.scalars().all()
    orders_data = []
    for o in all_orders:
        orders_data.append({
            "id": str(o.id),
            "buyer_id": str(o.buyer_id),
            "seller_id": str(o.seller_id),
            "amount": str(o.amount),
            "status": o.status,
            "created_at": str(o.created_at),
        })
    from repositories.users import UserRepository
    u = UserRepository(session)
    seller_user = await u.get_by_id(UUID("26ccbf6a-bac2-46b3-8ce5-d2e68f981d4e"))
    seller_info = None
    if seller_user:
        seller_info = {"email": seller_user.email, "role": seller_user.role, "id": str(seller_user.id)}
    return {
        "current_user": {"id": str(current_user.id), "email": current_user.email, "role": current_user.role},
        "seller_26ccbf6a": seller_info,
        "orders_count": len(orders_data),
        "orders": orders_data,
    }


@router.get("/", response_model=dict)
async def list_orders(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    status: str | None = Query(
        None, pattern=r"^(pending|funded|in_progress|completed|released|cancelled|disputed|resolved|resolved_refund|resolved_release)$"
    ),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    from models.orders import Order
    repo = OrderRepository(session)
    items, total = await repo.list_by_buyer(
        buyer_id=current_user.id, limit=limit, offset=offset, status=status
    )
    order_list = []
    for o in items:
        order_list.append(await _order_to_dict(o))
    return success_response(data=order_list, total=total, limit=limit, offset=offset)


@router.get("/sold", response_model=dict)
async def list_sold_orders(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    status: str | None = Query(
        None, pattern=r"^(pending|funded|in_progress|completed|released|cancelled|disputed|resolved|resolved_refund|resolved_release)$"
    ),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    items, total = await repo.list_by_seller(
        seller_id=current_user.id, limit=limit, offset=offset, status=status
    )
    order_list = []
    for o in items:
        order_list.append(await _order_to_dict(o))
    return success_response(data=order_list, total=total, limit=limit, offset=offset)


@router.get("/{order_id}", response_model=dict)
async def get_order(
    order_id: UUID,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    order = await repo.get_by_id(order_id)
    if not order:
        raise NotFoundException("Order not found")
    return success_response(data=await _order_to_dict(order))


@router.post("/", response_model=dict, status_code=201)
async def create_order(
    payload: OrderCreate,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    order = await repo.create_order(
        service_id=payload.service_id,
        buyer_id=current_user.id,
        seller_id=payload.seller_id,
        amount=str(payload.amount),
        notes=payload.notes,
    )
    return success_response(data=await _order_to_dict(order))


@router.patch("/{order_id}/status", response_model=dict)
async def update_order_status(
    order_id: UUID,
    payload: OrderStatusUpdate,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    order = await repo.get_by_id(order_id)
    if not order:
        raise NotFoundException("Order not found")

    if order.status == "disputed" and payload.status not in ("resolved_refund", "resolved_release"):
        raise HTTPException(status_code=409, detail="Cannot update order status while disputed. Resolve the dispute first.")

    prev_status = order.status
    updated = await repo.update_status(order, status=payload.status)

    if payload.status == OrderStatus.cancelled.value and prev_status in (
        OrderStatus.funded.value, OrderStatus.in_progress.value,
    ):
        from api.v1.escrow_proxy import _cancel_escrow_internal
        await _cancel_escrow_internal(order, session)

    return success_response(data=await _order_to_dict(updated))
