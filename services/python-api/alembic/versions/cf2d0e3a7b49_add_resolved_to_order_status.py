"""add_resolved_to_order_status

Revision ID: cf2d0e3a7b49
Revises: 825161ca6ac3
Create Date: 2026-06-01 01:55:00.000000
"""

from alembic import op
import sqlalchemy as sa

revision = 'cf2d0e3a7b49'
down_revision = '825161ca6ac3'
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.alter_column('orders', 'status',
               existing_type=sa.VARCHAR(length=11),
               type_=sa.VARCHAR(length=12),
               existing_nullable=False)


def downgrade() -> None:
    op.alter_column('orders', 'status',
               existing_type=sa.VARCHAR(length=12),
               type_=sa.VARCHAR(length=11),
               existing_nullable=False)
