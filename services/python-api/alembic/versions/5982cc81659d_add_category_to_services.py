"""add_category_to_services
Revision ID: 5982cc81659d
Revises: cf2d0e3a7b49
Create Date: 2026-06-01 19:41:33.330846
"""

from alembic import op
import sqlalchemy as sa

revision = '5982cc81659d'
down_revision = 'cf2d0e3a7b49'
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.add_column('services', sa.Column('category', sa.String(length=50), nullable=True))


def downgrade() -> None:
    op.drop_column('services', 'category')
