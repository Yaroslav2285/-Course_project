import structlog
from fastapi import FastAPI, HTTPException
from contextlib import asynccontextmanager

from blockchain import Blockchain
from schemas import (
    SubmitRequest,
    SubmitResponse,
    VerifyResponse,
    BlockResponse,
    HealthResponse,
)
from database import init_db, save_chain, load_chain
from config import settings

logger = structlog.get_logger()

blockchain = Blockchain()


@asynccontextmanager
async def lifespan(app: FastAPI):
    init_db()
    blocks_data = load_chain()
    if blocks_data:
        blockchain.load_from_db(blocks_data)
        logger.info("chain_loaded", blocks=len(blockchain.chain))
    else:
        save_chain(blockchain.to_dict_list())
        logger.info("genesis_created")
    yield


app = FastAPI(
    title="Blockchain Simulator",
    version="1.0.0",
    lifespan=lifespan,
)


@app.get("/health", response_model=HealthResponse)
def health():
    return HealthResponse(status="ok")


# LR #14: Фиксация транзакций в блокчейне — добавление блока с SHA-256 хешем
# LR #15: Валидация целостности цепочки перед добавлением нового блока
@app.post(
    f"{settings.api_prefix}/submit", response_model=SubmitResponse, status_code=201
)
def submit(req: SubmitRequest):
    if not blockchain.is_valid_chain():
        raise HTTPException(
            status_code=400, detail="Chain integrity compromised: tampering detected"
        )

    result = blockchain.add_block(req.model_dump())
    save_chain(blockchain.to_dict_list())
    logger.info(
        "block_added", block_index=result["block_index"], tx_hash=result["tx_hash"]
    )
    return SubmitResponse(**result)


# LR #14: Верификация цепочки — проверка всех prev_hash и пересчёт хешей
@app.get(f"{settings.api_prefix}/verify", response_model=VerifyResponse)
def verify():
    valid = blockchain.is_valid_chain()
    return VerifyResponse(valid=valid, blocks_count=len(blockchain.chain))


# LR #14: Получение полного аудит-трейла для конкретного заказа
@app.get(f"{settings.api_prefix}/audit/{{order_id}}")
def audit(order_id: str):
    blocks = blockchain.get_audit_log(order_id)
    if not blocks:
        raise HTTPException(
            status_code=404, detail=f"No audit records found for order {order_id}"
        )
    return [BlockResponse(**b.to_dict()) for b in blocks]
