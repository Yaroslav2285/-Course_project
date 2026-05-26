# LR #6: Web/DB
# LR #3: OOP/FP
import enum
import uuid

from sqlalchemy import Column, Enum, ForeignKey, Numeric, String, Text, TIMESTAMP, text
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from .base import Base


class ServiceStatus(str, enum.Enum):
    draft = "draft"
    published = "published"
    archived = "archived"


class Service(Base):
    __tablename__ = "services"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    provider_id = Column(UUID(as_uuid=True), ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    title = Column(String(200), nullable=False)
    description = Column(Text, nullable=True)
    price = Column(Numeric(19, 4), nullable=False)
    status = Column(Enum(ServiceStatus, name="service_status", native_enum=False), nullable=False, default=ServiceStatus.draft)
    created_at = Column(TIMESTAMP(timezone=True), server_default=text("now()"), nullable=False)
    updated_at = Column(
        TIMESTAMP(timezone=True),
        server_default=text("now()"),
        onupdate=text("now()"),
        nullable=False,
    )

    provider = relationship("User", lazy="selectin")
