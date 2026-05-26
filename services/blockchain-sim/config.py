from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    db_path: str = "blockchain.db"
    log_level: str = "INFO"
    service_name: str = "blockchain-sim"
    api_prefix: str = "/v1/chain"

    model_config = {"env_prefix": "BLOCKCHAIN_"}


settings = Settings()
