# LR #6: Web/DB
# LR #3: OOP/FP
import enum
import uuid

from sqlalchemy import Column, Enum, ForeignKey, Numeric, TIMESTAMP, Text, text
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from .base import Base


class TransactionType(str, enum.Enum):
    topup = "topup"
    escrow_fund = "escrow_fund"
    escrow_fund_rollback = "escrow_fund_rollback"
    transfer = "transfer"
    refund = "refund"
    withdrawal = "withdrawal"


class TransactionStatus(str, enum.Enum):
    pending = "pending"
    success = "success"
    failed = "failed"


class Wallet(Base):
    __tablename__ = "wallets"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id = Column(UUID(as_uuid=True), ForeignKey("users.id", ondelete="CASCADE"), nullable=False, unique=True)
    balance = Column(Numeric(19, 4), nullable=False, default=0)
    created_at = Column(TIMESTAMP(timezone=True), server_default=text("now()"), nullable=False)
    updated_at = Column(
        TIMESTAMP(timezone=True),
        server_default=text("now()"),
        onupdate=text("now()"),
        nullable=False,
    )

    user = relationship("User", lazy="selectin")
    transactions = relationship("Transaction", back_populates="wallet", lazy="selectin", order_by="Transaction.created_at.desc()")


class Transaction(Base):
    __tablename__ = "transactions"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    wallet_id = Column(UUID(as_uuid=True), ForeignKey("wallets.id", ondelete="CASCADE"), nullable=False)
    type = Column(Enum(TransactionType, name="transaction_type", native_enum=False), nullable=False)
    amount = Column(Numeric(19, 4), nullable=False)
    status = Column(Enum(TransactionStatus, name="transaction_status", native_enum=False), nullable=False, default=TransactionStatus.pending)
    reference_id = Column(UUID(as_uuid=True), nullable=True)
    description = Column(Text, nullable=True)
    created_at = Column(TIMESTAMP(timezone=True), server_default=text("now()"), nullable=False)

    wallet = relationship("Wallet", back_populates="transactions")
