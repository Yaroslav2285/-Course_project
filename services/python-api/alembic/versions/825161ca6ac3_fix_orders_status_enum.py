# LR #6: Web/DB
# LR #3: OOP/FP
"""fix_orders_status_enum
Revision ID: 825161ca6ac3
Revises: 8b57a29864d5
Create Date: 2026-05-31 20:16:14.407815
"""

from alembic import op
import sqlalchemy as sa

revision = '825161ca6ac3'
down_revision = '8b57a29864d5'
branch_labels = None
depends_on = None


def upgrade() -> None:
    # Widen orders.status from VARCHAR(9) to VARCHAR(11) to accommodate 'in_progress'.
    # The initial migration used Enum(native_enum=False) which creates a bare
    # VARCHAR column sized to the max enum value length, but the OrderStatus enum
    # later added 'in_progress' (11 chars) and 'completed' (9 chars).
    op.alter_column('orders', 'status',
               existing_type=sa.VARCHAR(length=9),
               type_=sa.VARCHAR(length=11),
               existing_nullable=False)


def downgrade() -> None:
    op.alter_column('orders', 'status',
               existing_type=sa.VARCHAR(length=11),
               type_=sa.VARCHAR(length=9),
               existing_nullable=False)
