# LR #6: Web/DB
# LR #3: OOP/FP
"""add_wallet
Revision ID: 8b57a29864d5
Revises: fff36bd858b0
Create Date: 2026-05-31 22:35:32.256308
"""

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision = '8b57a29864d5'
down_revision = 'fff36bd858b0'
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        'wallets',
        sa.Column('id', sa.UUID(), nullable=False),
        sa.Column('user_id', sa.UUID(), nullable=False),
        sa.Column('balance', sa.Numeric(19, 4), nullable=False, server_default=sa.text('0')),
        sa.Column('created_at', sa.TIMESTAMP(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.Column('updated_at', sa.TIMESTAMP(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ondelete='CASCADE'),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('user_id'),
    )
    op.create_table(
        'transactions',
        sa.Column('id', sa.UUID(), nullable=False),
        sa.Column('wallet_id', sa.UUID(), nullable=False),
        sa.Column('type', sa.Enum('topup', 'escrow_fund', 'refund', 'withdrawal', name='transaction_type', native_enum=False), nullable=False),
        sa.Column('amount', sa.Numeric(19, 4), nullable=False),
        sa.Column('status', sa.Enum('pending', 'success', 'failed', name='transaction_status', native_enum=False), nullable=False, server_default=sa.text("'pending'")),
        sa.Column('reference_id', sa.UUID(), nullable=True),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('created_at', sa.TIMESTAMP(timezone=True), server_default=sa.text("now()"), nullable=False),
        sa.ForeignKeyConstraint(['wallet_id'], ['wallets.id'], ondelete='CASCADE'),
        sa.PrimaryKeyConstraint('id'),
    )


def downgrade() -> None:
    op.drop_table('transactions')
    op.drop_table('wallets')
