# LR #2: Modern Python
# LR #4: Async/Web
from decimal import Decimal
from uuid import UUID

from sqlalchemy import select, func
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession

from models.wallet import Wallet, Transaction, TransactionType, TransactionStatus
from models.users import User
from repositories.base import RepositoryBase

ESCROW_USER_ID = UUID("1133d650-e7d4-41de-8877-c359682903a4")


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
        wallet.balance = wallet.balance + delta
        await self.session.flush()
        return wallet

    async def get_escrow_wallet(self) -> Wallet:
        escrow_user = await self.session.get(User, ESCROW_USER_ID)
        if not escrow_user:
            escrow_user = User(
                id=ESCROW_USER_ID,
                email="escrow@marketplace.local",
                hashed_password="*",
                role="admin",
            )
            self.session.add(escrow_user)
            await self.session.flush()
        wallet = await self.get_by_user_id(ESCROW_USER_ID)
        if not wallet:
            from uuid import uuid4 as _uuid4
            wallet = Wallet(id=_uuid4(), user_id=ESCROW_USER_ID, balance=Decimal("0"))
            self.session.add(wallet)
            await self.session.flush()
            return wallet
        return wallet

    async def transfer(
        self,
        from_user_id: UUID,
        to_user_id: UUID,
        amount: Decimal,
        reference_id: UUID | None = None,
        txn_type: str = "transfer",
        description: str | None = None,
    ) -> tuple[Transaction, Transaction]:
        from_wallet = (
            await self.get_escrow_wallet()
            if from_user_id == ESCROW_USER_ID
            else await self.get_or_create(from_user_id)
        )
        to_wallet = (
            await self.get_escrow_wallet()
            if to_user_id == ESCROW_USER_ID
            else await self.get_or_create(to_user_id)
        )

        if from_wallet.balance < amount:
            raise ValueError("Insufficient balance")

        from_wallet = await self.update_balance(from_wallet, -amount)
        to_wallet = await self.update_balance(to_wallet, amount)

        txn_repo = TransactionRepository(self.session)
        debit_txn = await txn_repo.create_transaction(
            wallet_id=from_wallet.id,
            type=txn_type,
            amount=-amount,
            status=TransactionStatus.success.value,
            reference_id=reference_id,
            description=description,
        )
        credit_txn = await txn_repo.create_transaction(
            wallet_id=to_wallet.id,
            type=txn_type,
            amount=amount,
            status=TransactionStatus.success.value,
            reference_id=reference_id,
            description=description,
        )
        return debit_txn, credit_txn


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
