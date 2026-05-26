# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from models.users import User
from repositories.base import RepositoryBase
from core.security import hash_password


class UserRepository(RepositoryBase[User]):
    def __init__(self, session: AsyncSession):
        super().__init__(User, session)

    async def get_by_id(self, user_id: UUID) -> User | None:
        return await self.get(id=user_id)

    async def get_by_email(self, email: str) -> User | None:
        stmt = select(User).where(User.email == email)
        result = await self.session.execute(stmt)
        return result.scalar_one_or_none()

    async def create_user(
        self, email: str, password: str, role: str = "client"
    ) -> User:
        return await self.create(
            email=email,
            hashed_password=hash_password(password),
            role=role,
        )

    async def update_user(
        self, user: User, email: str | None = None, password: str | None = None
    ) -> User:
        updates = {}
        if email is not None and email != user.email:
            updates["email"] = email
        if password is not None:
            updates["hashed_password"] = hash_password(password)
        if updates:
            return await self.update(user, **updates)
        return user
