# LR #10: Multi-lang/REST — межсервисный HTTP клиент Python → Go Escrow
# LR #13: Testing/Automation — идемпотентность, retry, экспоненциальная задержка
# LR #12: AI Integration — генерация через AI, контракты в docs/contracts/

import asyncio
import uuid
from typing import Any

import httpx
import structlog

from core.config import settings

logger = structlog.get_logger()

DEFAULT_TIMEOUT = 5.0
MAX_RETRIES = 3
BASE_DELAY = 0.1


class EscrowClientError(Exception):
    def __init__(self, status_code: int, code: str, detail: str):
        self.status_code = status_code
        self.code = code
        self.detail = detail
        super().__init__(f"[{code}] {detail}")


class EscrowClient:
    def __init__(
        self,
        base_url: str | None = None,
        timeout: float = DEFAULT_TIMEOUT,
        max_retries: int = MAX_RETRIES,
    ):
        self.base_url = (base_url or settings.GO_ESCROW_BASE_URL).rstrip("/")
        self.timeout = timeout
        self.max_retries = max_retries

    async def create_escrow(
        self, order_id: str, amount: str, idempotency_key: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            "POST",
            "/v1/escrow",
            json={"order_id": order_id, "amount": amount},
            idempotency_key=idempotency_key,
        )

    async def fund_escrow(
        self, escrow_id: str, amount: str, idempotency_key: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            "POST",
            f"/v1/escrow/{escrow_id}/fund",
            json={"amount": amount},
            idempotency_key=idempotency_key,
        )

    async def release_escrow(
        self, escrow_id: str, idempotency_key: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            "POST",
            f"/v1/escrow/{escrow_id}/release",
            json={},
            idempotency_key=idempotency_key,
        )

    async def dispute_escrow(
        self, escrow_id: str, reason: str, idempotency_key: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            "POST",
            f"/v1/escrow/{escrow_id}/dispute",
            json={"reason": reason},
            idempotency_key=idempotency_key,
        )

    async def _request(
        self,
        method: str,
        path: str,
        json: dict[str, Any] | None = None,
        idempotency_key: str | None = None,
    ) -> dict[str, Any]:
        url = f"{self.base_url}{path}"
        request_id = str(uuid.uuid4())
        headers = {
            "Content-Type": "application/json",
            "X-Request-ID": request_id,
        }
        if idempotency_key:
            headers["X-Idempotency-Key"] = idempotency_key

        for attempt in range(1, self.max_retries + 1):
            try:
                async with httpx.AsyncClient(timeout=self.timeout) as client:
                    response = await client.request(
                        method, url, json=json, headers=headers
                    )
                break
            except (httpx.TimeoutException, httpx.ConnectError) as exc:
                logger.warning(
                    "escrow_retry",
                    attempt=attempt,
                    max_retries=self.max_retries,
                    path=path,
                    error=str(exc),
                )
                if attempt < self.max_retries:
                    delay = BASE_DELAY * (2 ** (attempt - 1))
                    await asyncio.sleep(delay)
                else:
                    raise EscrowClientError(
                        status_code=0,
                        code="SERVICE_UNAVAILABLE",
                        detail=f"Escrow service unavailable after {self.max_retries} retries: {exc}",
                    ) from exc
        else:
            raise EscrowClientError(
                status_code=0,
                code="SERVICE_UNAVAILABLE",
                detail=f"Escrow service unavailable after {self.max_retries} retries",
            )

        return self._process_response(response)

    def _process_response(self, response: httpx.Response) -> dict[str, Any]:
        body = response.json()

        if response.is_success:
            return body.get("data") or body

        errors = body.get("errors", [])
        first_error = errors[0] if errors else {"code": "UNKNOWN", "detail": "Unknown error"}
        code = first_error.get("code", "UNKNOWN")
        detail = first_error.get("detail", "Escrow service error")

        raise EscrowClientError(
            status_code=response.status_code,
            code=code,
            detail=detail,
        )
