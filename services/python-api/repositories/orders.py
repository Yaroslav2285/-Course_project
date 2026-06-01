# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from sqlalchemy import select, func
from sqlalchemy.ext.asyncio import AsyncSession

from models.orders import Order, OrderStatus
from repositories.base import RepositoryBase


class OrderRepository(RepositoryBase[Order]):
    def __init__(self, session: AsyncSession):
        super().__init__(Order, session)

    async def get_by_id(self, order_id: UUID) -> Order | None:
        return await self.get(id=order_id)

    async def list_by_buyer(
        self, buyer_id: UUID, limit: int = 20, offset: int = 0, status: str | None = None
    ) -> tuple[list[Order], int]:
        filters = {"buyer_id": buyer_id}
        if status is not None:
            filters["status"] = status
        return await self._list_ordered(limit=limit, offset=offset, **filters)

    async def list_by_seller(
        self, seller_id: UUID, limit: int = 20, offset: int = 0, status: str | None = None
    ) -> tuple[list[Order], int]:
        filters = {"seller_id": seller_id}
        if status is not None:
            filters["status"] = status
        return await self._list_ordered(limit=limit, offset=offset, **filters)

    async def create_order(
        self,
        service_id: UUID,
        buyer_id: UUID,
        seller_id: UUID,
        amount: str,
        notes: str | None = None,
    ) -> Order:
        return await self.create(
            service_id=service_id,
            buyer_id=buyer_id,
            seller_id=seller_id,
            amount=amount,
            status=OrderStatus.pending.value,
            notes=notes,
        )

    async def list_disputes(
        self, limit: int = 50, offset: int = 0
    ) -> tuple[list[Order], int]:
        return await self._list_ordered(
            limit=limit, offset=offset, status=OrderStatus.disputed.value
        )

    async def _list_ordered(
        self, limit: int = 20, offset: int = 0, **filters
    ) -> tuple[list[Order], int]:
        count_stmt = select(func.count()).select_from(self.model)
        if filters:
            count_stmt = count_stmt.filter_by(**filters)
        count_result = await self.session.execute(count_stmt)
        total = count_result.scalar() or 0

        stmt = (
            select(self.model)
            .filter_by(**filters)
            .order_by(self.model.created_at.desc())
            .offset(offset)
            .limit(limit)
        )
        result = await self.session.execute(stmt)
        items = list(result.scalars().all())
        return items, total

    async def update_status(self, order: Order, status: str) -> Order:
        return await self.update(order, status=status)
