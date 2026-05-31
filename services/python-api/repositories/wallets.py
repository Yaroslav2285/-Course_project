# LR #2: Modern Python
# LR #4: Async/Web
from decimal import Decimal
from uuid import UUID

from sqlalchemy import select, func
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession

from models.wallet import Wallet, Transaction, TransactionType, TransactionStatus
from repositories.base import RepositoryBase


class WalletRepository(RepositoryBase[Wallet]):
    def __init__(self, session: AsyncSession):
        super().__init__(Wallet, session)

    async def get_by_user_id(self, user_id: UUID) -> Wallet | None:
        return await self.get(user_id=user_id)

    async def get_or_create(self, user_id: UUID) -> Wallet:
        wallet = await self.get_by_user_id(user_id)
        if wallet:
            return wallet
        try:
            return await self.create(user_id=user_id, balance=Decimal("0"))
        except IntegrityError:
            await self.session.rollback()
            wallet = await self.get_by_user_id(user_id)
            if wallet:
                return wallet
            raise

    async def update_balance(self, wallet: Wallet, delta: Decimal) -> Wallet:
        return await self.update(wallet, balance=wallet.balance + delta)


class TransactionRepository(RepositoryBase[Transaction]):
    def __init__(self, session: AsyncSession):
        super().__init__(Transaction, session)

    async def create_transaction(
        self,
        wallet_id: UUID,
        type: str,
        amount: Decimal,
        status: str = TransactionStatus.pending.value,
        reference_id: UUID | None = None,
        description: str | None = None,
    ) -> Transaction:
        return await self.create(
            wallet_id=wallet_id,
            type=type,
            amount=amount,
            status=status,
            reference_id=reference_id,
            description=description,
        )

    async def list_by_wallet(
        self, wallet_id: UUID, limit: int = 20, offset: int = 0
    ) -> tuple[list[Transaction], int]:
        count_stmt = (
            select(func.count())
            .select_from(Transaction)
            .where(Transaction.wallet_id == wallet_id)
        )
        count_result = await self.session.execute(count_stmt)
        total = count_result.scalar() or 0

        stmt = (
            select(Transaction)
            .where(Transaction.wallet_id == wallet_id)
            .order_by(Transaction.created_at.desc())
            .offset(offset)
            .limit(limit)
        )
        result = await self.session.execute(stmt)
        items = list(result.scalars().all())
        return items, total
