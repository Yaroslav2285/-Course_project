# LR #2: Modern Python
# LR #4: Async/Web
from fastapi import APIRouter

from api.v1.auth import router as auth_router
from api.v1.users import router as users_router
from api.v1.services import router as services_router
from api.v1.orders import router as orders_router
from api.v1.escrow_proxy import router as escrow_proxy_router
from api.v1.chain_proxy import router as chain_proxy_router
from api.v1.wallet import router as wallet_router

router = APIRouter(prefix="/v1")
router.include_router(auth_router, prefix="/auth", tags=["auth"])
router.include_router(users_router, prefix="/users", tags=["users"])
router.include_router(services_router, prefix="/services", tags=["services"])
router.include_router(orders_router, prefix="/orders", tags=["orders"])
router.include_router(escrow_proxy_router, prefix="/escrow", tags=["escrow"])
router.include_router(chain_proxy_router, prefix="/chain", tags=["chain"])
router.include_router(wallet_router, prefix="/wallet", tags=["wallet"])
