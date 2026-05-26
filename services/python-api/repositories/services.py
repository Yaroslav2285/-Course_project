# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from sqlalchemy.ext.asyncio import AsyncSession

from models.services import Service, ServiceStatus
from repositories.base import RepositoryBase


class ServiceRepository(RepositoryBase[Service]):
    def __init__(self, session: AsyncSession):
        super().__init__(Service, session)

    async def get_by_id(self, service_id: UUID) -> Service | None:
        return await self.get(id=service_id)

    async def list_by_provider(
        self, provider_id: UUID, limit: int = 20, offset: int = 0
    ) -> tuple[list[Service], int]:
        return await self.list(limit=limit, offset=offset, provider_id=provider_id)

    async def list_published(
        self, limit: int = 20, offset: int = 0
    ) -> tuple[list[Service], int]:
        return await self.list(limit=limit, offset=offset, status=ServiceStatus.published)

    async def create_service(
        self,
        provider_id: UUID,
        title: str,
        price: str,
        description: str | None = None,
        status: str = "draft",
    ) -> Service:
        return await self.create(
            provider_id=provider_id,
            title=title,
            price=price,
            description=description,
            status=status,
        )

    async def update_service(
        self,
        service: Service,
        title: str | None = None,
        description: str | None = None,
        price: str | None = None,
        status: str | None = None,
    ) -> Service:
        updates = {}
        if title is not None:
            updates["title"] = title
        if description is not None:
            updates["description"] = description
        if price is not None:
            updates["price"] = price
        if status is not None:
            updates["status"] = status
        if updates:
            return await self.update(service, **updates)
        return service
