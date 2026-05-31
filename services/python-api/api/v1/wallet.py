# LR #2: Modern Python
# LR #4: Async/Web
from fastapi import APIRouter, Depends, Query
from sqlalchemy.ext.asyncio import AsyncSession

from core.db import get_db
from core.deps import get_current_user
from core.exceptions import NotFoundException
from core.responses import success_response
from models.wallet import TransactionStatus
from repositories.wallets import WalletRepository, TransactionRepository
from schemas.wallet import TopUpRequest, WalletRead, TransactionRead
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
