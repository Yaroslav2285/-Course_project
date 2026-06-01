"""add_discount_to_services
Revision ID: b9cc5c4019ff
Revises: 5982cc81659d
Create Date: 2026-06-01 19:44:00.000000
"""

from alembic import op
import sqlalchemy as sa

revision = 'b9cc5c4019ff'
down_revision = '5982cc81659d'
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.add_column('services', sa.Column('discount', sa.Integer(), nullable=True))


def downgrade() -> None:
    op.drop_column('services', 'discount')
