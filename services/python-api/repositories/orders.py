# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from sqlalchemy.ext.asyncio import AsyncSession

from models.orders import Order, OrderStatus
from repositories.base import RepositoryBase


class OrderRepository(RepositoryBase[Order]):
    def __init__(self, session: AsyncSession):
        super().__init__(Order, session)

    async def get_by_id(self, order_id: UUID) -> Order | None:
        return await self.get(id=order_id)

    async def list_by_buyer(
        self, buyer_id: UUID, limit: int = 20, offset: int = 0
    ) -> tuple[list[Order], int]:
        return await self.list(limit=limit, offset=offset, buyer_id=buyer_id)

    async def list_by_seller(
        self, seller_id: UUID, limit: int = 20, offset: int = 0
    ) -> tuple[list[Order], int]:
        return await self.list(limit=limit, offset=offset, seller_id=seller_id)

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

    async def update_status(self, order: Order, status: str) -> Order:
        return await self.update(order, status=status)
