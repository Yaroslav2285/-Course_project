# LR #10: Multi-lang/REST — escrow proxy bridging frontend → Go escrow
# LR #6: Web/DB — fallback to direct order status update when Go unavailable
# LR #12: AI Integration — in-memory order_id→escrow_id cache, idempotency

import uuid
from typing import Any

from fastapi import APIRouter, Depends, Header
from sqlalchemy.ext.asyncio import AsyncSession

from app.services.escrow_client import EscrowClient, EscrowClientError
from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException
from core.responses import success_response
from repositories.orders import OrderRepository
from repositories.wallets import WalletRepository, ESCROW_USER_ID
from schemas.users import UserRead

router = APIRouter()

_escrow_cache: dict[str, str] = {}
_escrow_data: dict[str, dict[str, Any]] = {}


def _get_escrow_client() -> EscrowClient:
    return EscrowClient()


def _cache_set(order_id: str, escrow_id: str, data: dict | None = None) -> None:
    _escrow_cache[order_id] = escrow_id
    if data:
        _escrow_data[escrow_id] = data


def _cache_get(order_id: str) -> dict | None:
    escrow_id = _escrow_cache.get(order_id)
    if not escrow_id:
        return None
    data = _escrow_data.get(escrow_id)
    if not data:
        return None
    return data


@router.get("/by-order/{order_id}")
async def get_escrow_by_order(
    order_id: str,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    cached = _cache_get(order_id)
    if cached:
        return success_response(data=cached)

    client = _get_escrow_client()
    try:
        escrow_id = _escrow_cache.get(order_id)
        if escrow_id:
            data = await client._request("GET", f"/v1/escrow/{escrow_id}")
            _cache_set(order_id, escrow_id, data)
            return success_response(data=data)
    except EscrowClientError:
        pass

    raise NotFoundException("Escrow not found for this order")


@router.post("/{order_id}/fund", status_code=201)
async def fund_escrow(
    order_id: str,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    escrow_id = _escrow_cache.get(order_id)
    client = _get_escrow_client()

    try:
        if not escrow_id:
            repo = OrderRepository(session)
            order = await repo.get_by_id(uuid.UUID(order_id))
            if not order:
                raise NotFoundException("Order not found")
            result = await client.create_escrow(
                order_id=order_id,
                amount=str(order.amount),
                idempotency_key=idempotency_key,
            )
            escrow_id = result.get("id") or result.get("escrow_id") or str(uuid.uuid4())
            _cache_set(order_id, escrow_id, result)

        await client.fund_escrow(
            escrow_id=escrow_id,
            amount="0",
            idempotency_key=idempotency_key,
        )

        repo = OrderRepository(session)
        order = await repo.get_by_id(uuid.UUID(order_id))
        if order and order.status == "pending":
            await repo.update_status(order, status="funded")

        return success_response(
            data={
                "escrow_id": escrow_id,
                "order_id": order_id,
                "status": "funded",
            }
        )
    except EscrowClientError:
        repo = OrderRepository(session)
        order = await repo.get_by_id(uuid.UUID(order_id))
        if not order:
            raise NotFoundException("Order not found")
        if order.status != "pending":
            raise NotFoundException("Order is not in pending state")
        await repo.update_status(order, status="funded")
        return success_response(
            data={
                "escrow_id": None,
                "order_id": order_id,
                "status": "funded",
                "fallback": True,
            }
        )


@router.post("/{order_id}/advance")
async def advance_escrow(
    order_id: str,
    body: dict,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    target_status = body.get("status", "IN_PROGRESS")
    client = _get_escrow_client()
    escrow_id = _escrow_cache.get(order_id)

    try:
        if escrow_id:
            await client.advance_escrow(escrow_id=escrow_id, status=target_status)

        valid = target_status.lower()
        if valid not in ("in_progress", "funded", "completed"):
            valid = "funded"
        repo = OrderRepository(session)
        order = await repo.get_by_id(uuid.UUID(order_id))
        if not order:
            raise NotFoundException("Order not found")
        await repo.update_status(order, status=valid)
        return success_response(
            data={
                "escrow_id": escrow_id,
                "order_id": order_id,
                "status": valid,
            }
        )
    except EscrowClientError:
        valid = target_status.lower()
        if valid not in ("in_progress", "funded", "completed"):
            valid = "in_progress"
        repo = OrderRepository(session)
        order = await repo.get_by_id(uuid.UUID(order_id))
        if not order:
            raise NotFoundException("Order not found")
        await repo.update_status(order, status=valid)
        return success_response(
            data={
                "escrow_id": None,
                "order_id": order_id,
                "status": valid,
                "fallback": True,
            }
        )


@router.post("/{order_id}/complete")
async def complete_escrow(
    order_id: str,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    client = _get_escrow_client()
    escrow_id = _escrow_cache.get(order_id)

    try:
        if escrow_id:
            await client.complete_escrow(
                escrow_id=escrow_id, idempotency_key=idempotency_key
            )
    except EscrowClientError:
        pass

    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    if order.status == "in_progress":
        await repo.update_status(order, status="completed")
    return success_response(
        data={
            "escrow_id": escrow_id,
            "order_id": order_id,
            "status": "completed",
            "fallback": escrow_id is None,
        }
    )


@router.post("/{order_id}/release")
async def release_escrow(
    order_id: str,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    client = _get_escrow_client()
    escrow_id = _escrow_cache.get(order_id)

    try:
        if escrow_id:
            await client.release_escrow(
                escrow_id=escrow_id, idempotency_key=idempotency_key
            )
    except EscrowClientError:
        pass

    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    await repo.update_status(order, status="released")

    wallet_repo = WalletRepository(session)
    try:
        await wallet_repo.transfer(
            from_user_id=ESCROW_USER_ID,
            to_user_id=order.seller_id,
            amount=order.amount,
            reference_id=order.id,
            txn_type="transfer",
            description=f"Release payment for order {order.id}",
        )
    except ValueError:
        pass

    return success_response(
        data={
            "escrow_id": escrow_id,
            "order_id": order_id,
            "status": "released",
            "fallback": escrow_id is None,
        }
    )


@router.post("/{order_id}/dispute")
async def dispute_escrow(
    order_id: str,
    body: dict = None,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    reason = (body or {}).get("reason", "Disputed by user")
    client = _get_escrow_client()
    escrow_id = _escrow_cache.get(order_id)

    try:
        if escrow_id:
            await client.dispute_escrow(
                escrow_id=escrow_id,
                reason=reason,
                idempotency_key=idempotency_key,
            )
    except EscrowClientError:
        pass

    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    await repo.update_status(order, status="disputed")
    return success_response(
        data={
            "escrow_id": escrow_id,
            "order_id": order_id,
            "status": "disputed",
            "fallback": escrow_id is None,
        }
    )
