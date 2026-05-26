# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

from fastapi import APIRouter, Depends, Query
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.exceptions import ForbiddenException, NotFoundException
from core.responses import success_response
from models.services import ServiceStatus
from repositories.services import ServiceRepository
from schemas.services import ServiceCreate, ServiceRead, ServiceUpdate
from schemas.users import UserRead

router = APIRouter()


@router.get("/", response_model=dict)
async def list_services(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    status: str | None = Query(None, pattern=r"^(draft|published|archived)$"),
    session: AsyncSession = Depends(get_db),
):
    repo = ServiceRepository(session)
    filters = {}
    if status:
        filters["status"] = status
    else:
        filters["status"] = ServiceStatus.published.value
    items, total = await repo.list(limit=limit, offset=offset, **filters)
    service_list = [ServiceRead.model_validate(s).model_dump() for s in items]
    return success_response(data=service_list, total=total, limit=limit, offset=offset)


@router.get("/my", response_model=dict)
async def list_my_services(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = ServiceRepository(session)
    items, total = await repo.list_by_provider(
        provider_id=current_user.id, limit=limit, offset=offset
    )
    service_list = [ServiceRead.model_validate(s).model_dump() for s in items]
    return success_response(data=service_list, total=total, limit=limit, offset=offset)


@router.get("/{service_id}", response_model=dict)
async def get_service(
    service_id: UUID,
    session: AsyncSession = Depends(get_db),
):
    repo = ServiceRepository(session)
    service = await repo.get_by_id(service_id)
    if not service:
        raise NotFoundException("Service not found")
    return success_response(data=ServiceRead.model_validate(service).model_dump())


@router.post("/", response_model=dict, status_code=201)
async def create_service(
    payload: ServiceCreate,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = ServiceRepository(session)
    service = await repo.create_service(
        provider_id=current_user.id,
        title=payload.title,
        description=payload.description,
        price=str(payload.price),
    )
    return success_response(data=ServiceRead.model_validate(service).model_dump())


@router.put("/{service_id}", response_model=dict)
async def update_service(
    service_id: UUID,
    payload: ServiceUpdate,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = ServiceRepository(session)
    service = await repo.get_by_id(service_id)
    if not service:
        raise NotFoundException("Service not found")
    if service.provider_id != current_user.id:
        raise ForbiddenException("You can only edit your own services")
    updated = await repo.update_service(
        service,
        title=payload.title,
        description=payload.description,
        price=str(payload.price) if payload.price else None,
        status=payload.status,
    )
    return success_response(data=ServiceRead.model_validate(updated).model_dump())


@router.delete("/{service_id}", response_model=dict)
async def delete_service(
    service_id: UUID,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = ServiceRepository(session)
    service = await repo.get_by_id(service_id)
    if not service:
        raise NotFoundException("Service not found")
    if service.provider_id != current_user.id:
        raise ForbiddenException("You can only delete your own services")
    await repo.delete(service)
    return success_response(data={"message": "Service deleted successfully"})
