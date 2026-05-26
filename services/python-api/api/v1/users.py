# LR #2: Modern Python
# LR #4: Async/Web
from fastapi import APIRouter, Depends

from core.deps import get_current_user
from core.exceptions import DuplicateException
from core.responses import success_response
from repositories.users import UserRepository
from schemas.users import UserRead, UserUpdate
from core.db import get_db
from sqlalchemy.ext.asyncio import AsyncSession
from uuid import UUID

router = APIRouter()


@router.get("/me", response_model=dict)
async def get_me(current_user: UserRead = Depends(get_current_user)):
    return success_response(data=current_user.model_dump())


@router.put("/me", response_model=dict)
async def update_me(
    payload: UserUpdate,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = UserRepository(session)
    user = await repo.get_by_id(current_user.id)
    if not user:
        from core.exceptions import NotFoundException
        raise NotFoundException("User not found")
    if payload.email and payload.email != current_user.email:
        existing = await repo.get_by_email(payload.email)
        if existing:
            raise DuplicateException("Email already in use")
    updated = await repo.update_user(
        user,
        email=payload.email,
        password=payload.password,
    )
    return success_response(data=UserRead.model_validate(updated).model_dump())


@router.get("/{user_id}", response_model=dict)
async def get_user(
    user_id: UUID,
    session: AsyncSession = Depends(get_db),
    current_user: UserRead = Depends(get_current_user),
):
    repo = UserRepository(session)
    user = await repo.get_by_id(user_id)
    if not user:
        from core.exceptions import NotFoundException
        raise NotFoundException("User not found")
    return success_response(data=UserRead.model_validate(user).model_dump())
