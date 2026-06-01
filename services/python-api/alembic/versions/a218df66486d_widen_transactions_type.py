# LR #6: Web/DB
# LR #3: OOP/FP
"""widen_transactions_type

Revision ID: a218df66486d
Revises: b9cc5c4019ff
Create Date: 2026-06-02 00:31:01.805347
"""

from alembic import op
import sqlalchemy as sa

revision = 'a218df66486d'
down_revision = 'b9cc5c4019ff'
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.alter_column('transactions', 'type',
               existing_type=sa.VARCHAR(length=11),
               type_=sa.VARCHAR(length=20),
               existing_nullable=False)


def downgrade() -> None:
    op.alter_column('transactions', 'type',
               existing_type=sa.VARCHAR(length=20),
               type_=sa.VARCHAR(length=11),
               existing_nullable=False)
