# LR #2: Modern Python
# LR #4: Async/Web
import uuid

from fastapi import APIRouter, Depends, Query
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException, BadRequestException
from core.responses import success_response
from models.wallet import TransactionStatus
from models.orders import OrderStatus
from repositories.wallets import WalletRepository, TransactionRepository, ESCROW_USER_ID
from repositories.orders import OrderRepository
from schemas.wallet import TopUpRequest, PayRequest, WalletRead, TransactionRead
from schemas.users import UserRead
from app.services.escrow_client import EscrowClient, EscrowClientError
from app.services.escrow_cache import cache_set, get_escrow_id

router = APIRouter()


@router.get("/balance", response_model=dict)
async def get_wallet_balance(
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    repo = WalletRepository(session)
    wallet = await repo.get_or_create(current_user.id)
    return success_response(data=WalletRead.model_validate(wallet).model_dump())


@router.post("/pay", response_model=dict, status_code=201)
async def pay_order(
    payload: PayRequest,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    wallet_repo = WalletRepository(session)
    order_repo = OrderRepository(session)

    order = await order_repo.get_by_id(payload.order_id)
    if not order:
        raise NotFoundException("Order not found")
    if str(order.buyer_id) != str(current_user.id):
        raise NotFoundException("Order not found")
    if order.status != OrderStatus.pending.value:
        raise BadRequestException("Order is not in pending status")

    wallet = await wallet_repo.get_or_create(current_user.id)
    if wallet.balance < order.amount:
        raise BadRequestException("Insufficient wallet balance")

    debit_txn, credit_txn = await wallet_repo.transfer(
        from_user_id=current_user.id,
        to_user_id=ESCROW_USER_ID,
        amount=order.amount,
        reference_id=order.id,
        txn_type="escrow_fund",
        description=f"Payment for order {order.id}",
    )

    idempotency_key = f"pay_{order.id}"
    ec = EscrowClient()
    go_ok = False
    try:
        escrow_id = await get_escrow_id(str(order.id))
        if not escrow_id:
            result = await ec.create_escrow(
                order_id=str(order.id),
                amount=str(order.amount),
                idempotency_key=idempotency_key,
            )
            escrow_id = result.get("id") or result.get("escrow_id") or str(uuid.uuid4())
            await cache_set(str(order.id), escrow_id, result)
        await ec.fund_escrow(
            escrow_id=escrow_id,
            amount=str(order.amount),
            idempotency_key=idempotency_key,
        )
    except EscrowClientError as exc:
        if exc.code == "SERVICE_UNAVAILABLE":
            pass
        else:
            await wallet_repo.transfer(
                from_user_id=ESCROW_USER_ID,
                to_user_id=current_user.id,
                amount=order.amount,
                reference_id=order.id,
                txn_type="escrow_fund_rollback",
                description=f"Rollback payment for order {order.id}",
            )
            raise BadRequestException(
                f"Escrow service error ({exc.code}), payment rolled back"
            )

    order = await order_repo.update_status(order, OrderStatus.funded.value)
    wallet = await wallet_repo.get_by_user_id(current_user.id)

    return success_response(
        data={
            "balance": str(wallet.balance),
            "transaction": TransactionRead.model_validate(debit_txn).model_dump(),
        }
    )


@router.post("/topup", response_model=dict, status_code=201)
async def topup_wallet(
    payload: TopUpRequest,
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    wallet_repo = WalletRepository(session)
    txn_repo = TransactionRepository(session)

    wallet = await wallet_repo.get_or_create(current_user.id)
    wallet = await wallet_repo.update_balance(wallet, payload.amount)

    txn = await txn_repo.create_transaction(
        wallet_id=wallet.id,
        type="topup",
        amount=payload.amount,
        status=TransactionStatus.success.value,
    )

    return success_response(
        data={
            "balance": str(wallet.balance),
            "transaction": TransactionRead.model_validate(txn).model_dump(),
        }
    )


@router.get("/transactions", response_model=dict)
async def get_wallet_transactions(
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    current_user: UserRead = Depends(get_current_user),
    session: AsyncSession = Depends(get_db),
):
    wallet_repo = WalletRepository(session)
    txn_repo = TransactionRepository(session)

    wallet = await wallet_repo.get_or_create(current_user.id)
    items, total = await txn_repo.list_by_wallet(wallet.id, limit=limit, offset=offset)
    txn_list = [TransactionRead.model_validate(t).model_dump() for t in items]
    return success_response(data=txn_list, total=total, limit=limit, offset=offset)
