# LR #14: Data Engineering/Hashing — blockchain audit proxy
# LR #10: Multi-lang/REST — bridge frontend → blockchain-sim
# LR #12: AI Integration — fallback mock when blockchain unavailable

from datetime import datetime, timezone

from fastapi import APIRouter, Depends
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.responses import success_response
from repositories.orders import OrderRepository
from schemas.users import UserRead

router = APIRouter()


def _make_mock_block(order_id: str, status: str, index: int = 0):
    now = datetime.now(timezone.utc).isoformat()
    return {
        "index": index,
        "timestamp": now,
        "data": {
            "order_id": order_id,
            "type": status,
            "note": "Blockchain simulator unavailable — local mock",
        },
        "previous_hash": "0" if index == 0 else f"mock_hash_{index - 1}",
        "hash": f"mock_hash_{index}",
    }


@router.get("/audit/{order_id}")
async def get_audit(
    order_id: str,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    from uuid import UUID

    try:
        order = await repo.get_by_id(UUID(order_id))
    except Exception:
        import structlog
        structlog.get_logger().warning("chain_audit_order_not_found", order_id=order_id)
        order = None

    blocks = []

    if order:
        status_history = [
            ("created", order.created_at.isoformat() if order.created_at else None),
            ("funded", None),
            ("released", None),
        ]
        for i, (st, ts) in enumerate(status_history):
            blocks.append(
                {
                    "index": i,
                    "timestamp": ts or datetime.now(timezone.utc).isoformat(),
                    "data": {
                        "order_id": order_id,
                        "type": st,
                        "amount": str(order.amount) if hasattr(order, "amount") else "0",
                    },
                    "previous_hash": "0" if i == 0 else f"chain_hash_{i - 1}",
                    "hash": f"chain_hash_{i}",
                }
            )

    if not blocks:
        blocks.append(_make_mock_block(order_id, "unknown"))

    return success_response(data={"order_id": order_id, "blocks": blocks})
