# LR #2: Modern Python
# LR #4: Async/Web
from pydantic_settings import BaseSettings, SettingsConfigDict
from typing import ClassVar


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    DB_URL: str = "postgresql+asyncpg://app_user:ChangeMe123!@localhost:5432/app_db"
    DB_ECHO: bool = False

    JWT_SECRET: str = "your-super-secret-jwt-key-change-in-production-min-32-chars"
    JWT_ALGORITHM: str = "HS256"
    JWT_EXPIRE_MINUTES: int = 30
    JWT_REFRESH_EXPIRE_DAYS: int = 7

    API_TITLE: str = "Service Marketplace API"
    API_VERSION: str = "1.0.0"
    API_ROOT_PATH: str = "/api"
    API_DOCS_URL: str = "/docs"
    API_REDOC_URL: str = "/redoc"

    SERVER_HOST: str = "0.0.0.0"
    SERVER_PORT: int = 8000
    SERVER_RELOAD: bool = True
    CORS_ORIGINS: str = "http://localhost:3000,http://localhost:8000"

    LOG_LEVEL: str = "INFO"

    @property
    def cors_origin_list(self) -> list[str]:
        return [o.strip() for o in self.CORS_ORIGINS.split(",") if o.strip()]

    MIN_PASSWORD_LENGTH: ClassVar[int] = 8


settings = Settings()
