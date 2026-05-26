# LR #2: Modern Python
# LR #4: Async/Web
from typing import Any, Generic, TypeVar

from sqlalchemy import select, func
from sqlalchemy.ext.asyncio import AsyncSession

from models.base import Base

ModelT = TypeVar("ModelT", bound=Base)


class RepositoryBase(Generic[ModelT]):
    def __init__(self, model: type[ModelT], session: AsyncSession):
        self.model = model
        self.session = session

    async def create(self, **kwargs: Any) -> ModelT:
        instance = self.model(**kwargs)
        self.session.add(instance)
        await self.session.flush()
        await self.session.refresh(instance)
        return instance

    async def get(self, **filters: Any) -> ModelT | None:
        stmt = select(self.model).filter_by(**filters)
        result = await self.session.execute(stmt)
        return result.scalar_one_or_none()

    async def list(
        self,
        limit: int = 20,
        offset: int = 0,
        **filters: Any,
    ) -> tuple[list[ModelT], int]:
        count_stmt = select(func.count()).select_from(self.model)
        if filters:
            count_stmt = count_stmt.filter_by(**filters)
        count_result = await self.session.execute(count_stmt)
        total = count_result.scalar() or 0

        stmt = select(self.model)
        if filters:
            stmt = stmt.filter_by(**filters)
        stmt = stmt.offset(offset).limit(limit)
        result = await self.session.execute(stmt)
        items = list(result.scalars().all())
        return items, total

    async def update(self, instance: ModelT, **kwargs: Any) -> ModelT:
        for key, value in kwargs.items():
            if value is not None:
                setattr(instance, key, value)
        await self.session.flush()
        await self.session.refresh(instance)
        return instance

    async def delete(self, instance: ModelT) -> None:
        await self.session.delete(instance)
        await self.session.flush()
