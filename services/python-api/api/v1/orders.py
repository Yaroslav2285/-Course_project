# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from fastapi import APIRouter, Depends, Query
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException
from core.responses import success_response
from models.orders import Order, OrderStatus
from repositories.orders import OrderRepository
from schemas.orders import OrderCreate, OrderRead, OrderStatusUpdate
from schemas.users import UserRead

router = APIRouter()


def _order_to_dict(order: Order) -> dict:
    d = OrderRead.model_validate(order).model_dump()
    d["seller_email"] = order.seller.email if order.seller else None
    d["buyer_email"] = order.buyer.email if order.buyer else None
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
        None, pattern=r"^(pending|funded|released|cancelled|disputed|resolved)$"
    ),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    filters: dict = {}
    if status:
        filters["status"] = status
    items, total = await repo.list_by_buyer(
        buyer_id=current_user.id, limit=limit, offset=offset
    )
    if filters:
        filtered = [o for o in items if o.status == status]
        total = len(filtered)
        items = filtered[offset : offset + limit]
    order_list = [_order_to_dict(o) for o in items]
    return success_response(data=order_list, total=total, limit=limit, offset=offset)


@router.get("/sold", response_model=dict)
async def list_sold_orders(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    status: str | None = Query(
        None, pattern=r"^(pending|funded|released|cancelled|disputed|resolved)$"
    ),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    filters: dict = {}
    if status:
        filters["status"] = status
    items, total = await repo.list_by_seller(
        seller_id=current_user.id, limit=limit, offset=offset
    )
    if filters:
        filtered = [o for o in items if o.status == status]
        total = len(filtered)
        items = filtered[offset : offset + limit]
    order_list = [_order_to_dict(o) for o in items]
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
    return success_response(data=_order_to_dict(order))


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
    return success_response(data=_order_to_dict(order))


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

    prev_status = order.status
    updated = await repo.update_status(order, status=payload.status)

    if payload.status == OrderStatus.cancelled.value and prev_status in (
        OrderStatus.funded.value, OrderStatus.in_progress.value,
    ):
        from api.v1.escrow_proxy import _cancel_escrow_internal
        await _cancel_escrow_internal(order, session)

    return success_response(data=_order_to_dict(updated))
