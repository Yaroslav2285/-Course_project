# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from fastapi import APIRouter, Depends, Query
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException
from core.responses import success_response
from repositories.orders import OrderRepository
from schemas.orders import OrderCreate, OrderRead, OrderStatusUpdate
from schemas.users import UserRead

router = APIRouter()


@router.get("/", response_model=dict)
async def list_orders(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    status: str | None = Query(
        None, pattern=r"^(pending|funded|released|cancelled|disputed)$"
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
    order_list = [OrderRead.model_validate(o).model_dump() for o in items]
    return success_response(data=order_list, total=total, limit=limit, offset=offset)


@router.get("/sold", response_model=dict)
async def list_sold_orders(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    status: str | None = Query(
        None, pattern=r"^(pending|funded|released|cancelled|disputed)$"
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
    order_list = [OrderRead.model_validate(o).model_dump() for o in items]
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
    return success_response(data=OrderRead.model_validate(order).model_dump())


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
    return success_response(data=OrderRead.model_validate(order).model_dump())


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
    updated = await repo.update_status(order, status=payload.status)
    return success_response(data=OrderRead.model_validate(updated).model_dump())
