# LR #4: Async/Web
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.responses import success_response
from models.orders import Order
from repositories.orders import OrderRepository
from schemas.users import UserRead

router = APIRouter()


def _order_to_dict(order: Order) -> dict:
    return {
        "id": str(order.id),
        "service_id": str(order.service_id),
        "buyer_id": str(order.buyer_id),
        "seller_id": str(order.seller_id),
        "buyer_email": order.buyer.email if order.buyer else None,
        "seller_email": order.seller.email if order.seller else None,
        "amount": str(order.amount),
        "status": order.status,
        "notes": order.notes,
        "created_at": order.created_at.isoformat() if order.created_at else None,
        "updated_at": order.updated_at.isoformat() if order.updated_at else None,
    }


@router.get("/disputes", response_model=dict)
async def list_disputes(
    limit: int = Query(50, ge=1, le=200),
    offset: int = Query(0, ge=0),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    if current_user.role != "admin":
        raise HTTPException(status_code=403, detail="Admin access required")

    repo = OrderRepository(session)
    items, total = await repo.list_disputes(limit=limit, offset=offset)

    order_list = [_order_to_dict(o) for o in items]
    return success_response(data=order_list, total=total, limit=limit, offset=offset)
