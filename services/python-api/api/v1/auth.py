# LR #2: Modern Python
# LR #4: Async/Web
from fastapi import APIRouter, Depends
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.exceptions import DuplicateException, UnauthorizedException
from core.responses import success_response
from core.security import (
    create_access_token,
    create_refresh_token,
    decode_token,
    verify_password,
)
from repositories.users import UserRepository
from schemas.users import TokenRefresh, TokenResponse, UserCreate, UserLogin, UserRead

router = APIRouter()


@router.post("/register", response_model=dict, status_code=201)
async def register(
    payload: UserCreate,
    session: AsyncSession = Depends(get_db),
):
    repo = UserRepository(session)
    existing = await repo.get_by_email(payload.email)
    if existing:
        raise DuplicateException("Email already registered")
    user = await repo.create_user(
        email=payload.email,
        password=payload.password,
        role=payload.role,
    )
    user_data = UserRead.model_validate(user)
    access_token = create_access_token(subject=str(user.id))
    refresh_token = create_refresh_token(subject=str(user.id))
    return success_response(
        data=TokenResponse(
            access_token=access_token,
            refresh_token=refresh_token,
            user=user_data,
        ).model_dump()
    )


@router.post("/login", response_model=dict)
async def login(
    payload: UserLogin,
    session: AsyncSession = Depends(get_db),
):
    repo = UserRepository(session)
    user = await repo.get_by_email(payload.email)
    if not user or not verify_password(payload.password, user.hashed_password):
        raise UnauthorizedException("Invalid email or password")
    user_data = UserRead.model_validate(user)
    access_token = create_access_token(subject=str(user.id))
    refresh_token = create_refresh_token(subject=str(user.id))
    return success_response(
        data=TokenResponse(
            access_token=access_token,
            refresh_token=refresh_token,
            user=user_data,
        ).model_dump()
    )


@router.post("/refresh", response_model=dict)
async def refresh(
    payload: TokenRefresh,
    session: AsyncSession = Depends(get_db),
):
    token_data = decode_token(payload.refresh_token)
    user_id = token_data.get("sub")
    token_type = token_data.get("type")
    if not user_id or token_type != "refresh":
        raise UnauthorizedException("Invalid or expired refresh token")
    repo = UserRepository(session)
    from uuid import UUID
    user = await repo.get_by_id(UUID(user_id))
    if not user:
        raise UnauthorizedException("User not found")
    user_data = UserRead.model_validate(user)
    new_access = create_access_token(subject=str(user.id))
    new_refresh = create_refresh_token(subject=str(user.id))
    return success_response(
        data=TokenResponse(
            access_token=new_access,
            refresh_token=new_refresh,
            user=user_data,
        ).model_dump()
    )
