# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from fastapi import Depends, Header, Query
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.exceptions import UnauthorizedException
from core.security import decode_token
from repositories.users import UserRepository
from schemas.users import UserRead


async def get_current_user(
    authorization: str = Header(""),
    session: AsyncSession = Depends(get_db),
) -> UserRead:
    if not authorization.startswith("Bearer "):
        raise UnauthorizedException("Invalid authorization header")
    token = authorization[7:]
    payload = decode_token(token)
    user_id = payload.get("sub")
    if not user_id:
        raise UnauthorizedException("Invalid or expired token")
    repo = UserRepository(session)
    user = await repo.get_by_id(UUID(user_id))
    if not user:
        raise UnauthorizedException("User not found")
    return UserRead.model_validate(user)


async def pagination_params(
    limit: int = Query(20, ge=1, le=100, description="Items per page"),
    offset: int = Query(0, ge=0, description="Offset for pagination"),
) -> dict:
    return {"limit": limit, "offset": offset}
