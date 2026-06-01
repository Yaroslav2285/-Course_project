import uuid

import structlog
from fastapi import APIRouter, Depends, Header, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession

from app.services.escrow_client import EscrowClient, EscrowClientError
from app.services.escrow_cache import cache_set, cache_get, get_escrow_id
from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException
from core.responses import success_response
from models.orders import Order
from repositories.orders import OrderRepository
from repositories.wallets import WalletRepository, ESCROW_USER_ID
from schemas.users import UserRead

router = APIRouter()


def _get_escrow_client() -> EscrowClient:
    return EscrowClient()


async def _cache_set(order_id: str, escrow_id: str, data: dict | None = None) -> None:
    await cache_set(order_id, escrow_id, data)


async def _cache_get(order_id: str) -> dict | None:
    return await cache_get(order_id)


@router.get("/by-order/{order_id}")
async def get_escrow_by_order(
    order_id: str,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    cached = await _cache_get(order_id)
    if cached:
        return success_response(data=cached)

    client = _get_escrow_client()
    try:
        escrow_id = await get_escrow_id(order_id)
        if escrow_id:
            data = await client._request("GET", f"/v1/escrow/{escrow_id}")
            await _cache_set(order_id, escrow_id, data)
            return success_response(data=data)
    except EscrowClientError:
        logger = structlog.get_logger()
        logger.warning("escrow_get_fallback", order_id=order_id)

    raise NotFoundException("Escrow not found for this order")


@router.post("/{order_id}/fund", status_code=201)
async def fund_escrow(
    order_id: str,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    if order.status == "disputed":
        raise HTTPException(status_code=409, detail="Cannot fund order while disputed")

    escrow_id = await get_escrow_id(order_id)
    client = _get_escrow_client()
    go_ok = True

    if not escrow_id:
        try:
            result = await client.create_escrow(
                order_id=order_id,
                amount=str(order.amount),
                idempotency_key=idempotency_key,
            )
            escrow_id = result.get("id") or result.get("escrow_id") or str(uuid.uuid4())
            await _cache_set(order_id, escrow_id, result)
        except EscrowClientError:
            logger = structlog.get_logger()
            logger.warning("escrow_create_fallback", order_id=order_id)
            go_ok = False

    if go_ok and escrow_id:
        try:
            await client.fund_escrow(
                escrow_id=escrow_id,
                amount=str(order.amount),
                idempotency_key=idempotency_key,
            )
        except EscrowClientError:
            logger = structlog.get_logger()
            logger.warning("escrow_fund_fallback", order_id=order_id, escrow_id=escrow_id)
            go_ok = False

    if order.status == "pending":
        await repo.update_status(order, status="funded")

    return success_response(
        data={
            "escrow_id": escrow_id if go_ok else None,
            "order_id": order_id,
            "status": "funded",
            "fallback": not go_ok,
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
    escrow_id = await get_escrow_id(order_id)

    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    if order.status == "disputed":
        raise HTTPException(status_code=409, detail="Cannot advance order while disputed")
    go_ok = True

    if escrow_id:
        try:
            await client.advance_escrow(escrow_id=escrow_id, status=target_status)
        except EscrowClientError:
            logger = structlog.get_logger()
            logger.warning("escrow_advance_fallback", order_id=order_id, target=target_status)
            go_ok = False

    valid = target_status.lower()
    if valid not in ("in_progress", "funded", "completed"):
        valid = "funded"

    await repo.update_status(order, status=valid)
    return success_response(
        data={
            "escrow_id": escrow_id if go_ok else None,
            "order_id": order_id,
            "status": valid,
            "fallback": not go_ok,
        }
    )


@router.post("/{order_id}/complete")
async def complete_escrow(
    order_id: str,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    if order.status == "disputed":
        raise HTTPException(status_code=409, detail="Cannot complete order while disputed")

    client = _get_escrow_client()
    escrow_id = await get_escrow_id(order_id)

    try:
        if escrow_id:
            await client.advance_escrow(
                escrow_id=escrow_id,

                status="COMPLETED",
                idempotency_key=idempotency_key,
            )
    except EscrowClientError:
        logger = structlog.get_logger()
        logger.warning("escrow_complete_fallback", order_id=order_id)

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
    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    if order.status == "disputed":
        raise HTTPException(status_code=409, detail="Cannot release order while disputed")

    client = _get_escrow_client()
    escrow_id = await get_escrow_id(order_id)

    try:
        if escrow_id:
            await client.release_escrow(
                escrow_id=escrow_id, idempotency_key=idempotency_key
            )
    except EscrowClientError:
        logger = structlog.get_logger()
        logger.warning("escrow_release_fallback", order_id=order_id)

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
    except ValueError as exc:
        logger = structlog.get_logger()
        logger.warning("escrow_release_transfer_failed", order_id=order_id, error=str(exc))

    return success_response(
        data={
            "escrow_id": escrow_id,
            "order_id": order_id,
            "status": "released",
            "fallback": escrow_id is None,
        }
    )


@router.post("/{order_id}/resolve")
async def resolve_escrow(
    order_id: str,
    body: dict = None,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    action = (body or {}).get("action", "release")
    if action not in ("refund", "release"):
        raise HTTPException(status_code=422, detail="action must be 'refund' or 'release'")

    client = _get_escrow_client()
    escrow_id = await get_escrow_id(order_id)

    try:
        if escrow_id:
            await client.resolve_escrow(
                escrow_id=escrow_id,
                idempotency_key=idempotency_key,
            )
    except EscrowClientError:
        logger = structlog.get_logger()
        logger.warning("escrow_resolve_fallback", order_id=order_id)

    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")

    target_status = "resolved_refund" if action == "refund" else "resolved_release"
    await repo.update_status(order, status=target_status)

    wallet_repo = WalletRepository(session)
    to_user_id = order.buyer_id if action == "refund" else order.seller_id
    try:
        await wallet_repo.transfer(
            from_user_id=ESCROW_USER_ID,
            to_user_id=to_user_id,
            amount=order.amount,
            reference_id=order.id,
            txn_type="transfer",
            description=f"Resolve dispute - {action} for order {order.id}",
        )
    except ValueError as exc:
        logger = structlog.get_logger()
        logger.warning("escrow_resolve_transfer_failed", order_id=order_id, error=str(exc))

    return success_response(
        data={
            "escrow_id": escrow_id,
            "order_id": order_id,
            "status": target_status,
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
    escrow_id = await get_escrow_id(order_id)

    try:
        if escrow_id:
            await client.dispute_escrow(
                escrow_id=escrow_id,
                reason=reason,
                idempotency_key=idempotency_key,
            )
    except EscrowClientError:
        logger = structlog.get_logger()
        logger.warning("escrow_dispute_fallback", order_id=order_id)

    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")
    await repo.update_status(order, status="disputed")
    dispute_note = ("Dispute reason: " + reason) if reason else "Dispute reason: Disputed by user"
    await repo.update(order, notes=dispute_note)
    return success_response(
        data={
            "escrow_id": escrow_id,
            "order_id": order_id,
            "status": "disputed",
            "fallback": escrow_id is None,
        }
    )


async def _cancel_escrow_internal(
    order: Order,
    session: AsyncSession,
    idempotency_key: str | None = None,
) -> dict:
    if order.status == "disputed":
        raise HTTPException(status_code=409, detail="Cannot cancel order while disputed")
    client = _get_escrow_client()
    escrow_id = await get_escrow_id(str(order.id))

    try:
        if escrow_id:
            await client.cancel_escrow(
                escrow_id=escrow_id, idempotency_key=idempotency_key
            )
    except EscrowClientError:
        logger = structlog.get_logger()
        logger.warning("escrow_cancel_fallback", order_id=str(order.id), escrow_id=escrow_id)

    repo = OrderRepository(session)
    await repo.update_status(order, status="cancelled")

    wallet_repo = WalletRepository(session)
    try:
        await wallet_repo.transfer(
            from_user_id=ESCROW_USER_ID,
            to_user_id=order.buyer_id,
            amount=order.amount,
            reference_id=order.id,
            txn_type="refund",
            description=f"Refund for cancelled order {order.id}",
        )
    except ValueError as exc:
        logger = structlog.get_logger()
        logger.warning("escrow_refund_failed", order_id=str(order.id), error=str(exc))

    return {
        "escrow_id": escrow_id,
        "order_id": str(order.id),
        "status": "cancelled",
        "fallback": escrow_id is None,
    }


@router.post("/{order_id}/cancel")
async def cancel_proxy(
    order_id: str,
    idempotency_key: str | None = Header(None),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = OrderRepository(session)
    order = await repo.get_by_id(uuid.UUID(order_id))
    if not order:
        raise NotFoundException("Order not found")

    result = await _cancel_escrow_internal(order, session, idempotency_key)
    return success_response(data=result)
