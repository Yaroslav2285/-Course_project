# LR #2: Modern Python
# LR #4: Async/Web
from fastapi import APIRouter, Depends, Query
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException, BadRequestException
from core.responses import success_response
from models.wallet import TransactionStatus
from models.orders import OrderStatus
from repositories.wallets import WalletRepository, TransactionRepository
from repositories.orders import OrderRepository
from schemas.wallet import TopUpRequest, PayRequest, WalletRead, TransactionRead
from schemas.users import UserRead

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
    txn_repo = TransactionRepository(session)
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

    wallet = await wallet_repo.update_balance(wallet, -order.amount)
    txn = await txn_repo.create_transaction(
        wallet_id=wallet.id,
        type="escrow_fund",
        amount=-order.amount,
        status=TransactionStatus.success.value,
        reference_id=order.id,
        description=f"Payment for order {order.id}",
    )
    order = await order_repo.update_status(order, OrderStatus.funded.value)

    return success_response(
        data={
            "balance": str(wallet.balance),
            "transaction": TransactionRead.model_validate(txn).model_dump(),
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
