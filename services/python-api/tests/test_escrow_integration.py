# LR #13: Testing/Automation — интеграционные тесты межсервисного взаимодействия
# LR #10: Multi-lang/REST — мокирование Go Escrow ответов через respx
# LR #12: AI Integration — тесты сгенерированы AI, контракты в docs/contracts/

import uuid

import httpx
import pytest
import respx
from httpx import Response

from app.services.escrow_client import EscrowClient, EscrowClientError

BASE_URL = "http://test-escrow:8081"
ORDER_ID = str(uuid.uuid4())
ESCROW_ID = str(uuid.uuid4())
IDEM_KEY = str(uuid.uuid4())


@pytest.fixture
def client():
    return EscrowClient(base_url=BASE_URL, timeout=2.0, max_retries=1)


@pytest.mark.asyncio
async def test_create_escrow_success(client):
    expected = {
        "id": ESCROW_ID,
        "order_id": ORDER_ID,
        "balance": "0.0000",
        "status": "CREATED",
    }

    with respx.mock:
        route = respx.post(f"{BASE_URL}/v1/escrow").mock(
            return_value=Response(201, json={"data": expected})
        )
        result = await client.create_escrow(ORDER_ID, "500.0000")

    assert route.called
    assert result["id"] == ESCROW_ID
    assert result["status"] == "CREATED"


@pytest.mark.asyncio
async def test_fund_escrow_success(client):
    expected = {
        "id": ESCROW_ID,
        "order_id": ORDER_ID,
        "balance": "500.0000",
        "status": "FUNDED",
    }

    with respx.mock:
        route = respx.post(f"{BASE_URL}/v1/escrow/{ESCROW_ID}/fund").mock(
            return_value=Response(200, json={"data": expected})
        )
        result = await client.fund_escrow(ESCROW_ID, "500.0000")

    assert route.called
    assert result["status"] == "FUNDED"
    assert result["balance"] == "500.0000"


@pytest.mark.asyncio
async def test_release_escrow_success(client):
    expected = {
        "id": ESCROW_ID,
        "order_id": ORDER_ID,
        "balance": "500.0000",
        "status": "RELEASED",
    }

    with respx.mock:
        route = respx.post(f"{BASE_URL}/v1/escrow/{ESCROW_ID}/release").mock(
            return_value=Response(200, json={"data": expected})
        )
        result = await client.release_escrow(ESCROW_ID)

    assert route.called
    assert result["status"] == "RELEASED"


@pytest.mark.asyncio
async def test_dispute_escrow_success(client):
    expected = {
        "id": ESCROW_ID,
        "order_id": ORDER_ID,
        "balance": "500.0000",
        "status": "DISPUTED",
    }

    with respx.mock:
        route = respx.post(f"{BASE_URL}/v1/escrow/{ESCROW_ID}/dispute").mock(
            return_value=Response(200, json={"data": expected})
        )
        result = await client.dispute_escrow(ESCROW_ID, "Service not delivered")

    assert route.called
    assert result["status"] == "DISPUTED"


@pytest.mark.asyncio
async def test_escrow_not_found(client):
    unknown_id = str(uuid.uuid4())
    error_body = {"errors": [{"code": "NOT_FOUND", "detail": "Escrow account not found"}]}

    with respx.mock:
        respx.post(f"{BASE_URL}/v1/escrow/{unknown_id}/fund").mock(
            return_value=Response(404, json=error_body)
        )
        with pytest.raises(EscrowClientError) as exc_info:
            await client.fund_escrow(unknown_id, "100.0000")

    assert exc_info.value.status_code == 404
    assert exc_info.value.code == "NOT_FOUND"


@pytest.mark.asyncio
async def test_escrow_invalid_transition(client):
    error_body = {
        "errors": [{"code": "INVALID_TRANSITION", "detail": "invalid transition from CREATED to RELEASED"}]
    }

    with respx.mock:
        respx.post(f"{BASE_URL}/v1/escrow/{ESCROW_ID}/release").mock(
            return_value=Response(409, json=error_body)
        )
        with pytest.raises(EscrowClientError) as exc_info:
            await client.release_escrow(ESCROW_ID)

    assert exc_info.value.status_code == 409
    assert exc_info.value.code == "INVALID_TRANSITION"


@pytest.mark.asyncio
async def test_escrow_retry_on_timeout():
    retry_client = EscrowClient(base_url=BASE_URL, timeout=2.0, max_retries=2)
    with respx.mock:
        route = respx.post(f"{BASE_URL}/v1/escrow").mock(
            side_effect=[httpx.TimeoutException("timeout"), Response(201, json={"data": {"id": ESCROW_ID, "status": "CREATED"}})]
        )
        result = await retry_client.create_escrow(ORDER_ID, "500.0000")

    assert route.call_count == 2
    assert result["id"] == ESCROW_ID


@pytest.mark.asyncio
async def test_escrow_idempotency_key(client):
    expected = {"id": ESCROW_ID, "order_id": ORDER_ID, "balance": "0.0000", "status": "CREATED"}

    with respx.mock:
        route = respx.post(f"{BASE_URL}/v1/escrow").mock(
            return_value=Response(201, json={"data": expected})
        )
        result1 = await client.create_escrow(ORDER_ID, "500.0000", idempotency_key=IDEM_KEY)
        result2 = await client.create_escrow(ORDER_ID, "500.0000", idempotency_key=IDEM_KEY)

    assert route.call_count == 2
    assert result1["id"] == result2["id"]


@pytest.mark.asyncio
async def test_escrow_service_unavailable(client):
    with respx.mock:
        respx.post(f"{BASE_URL}/v1/escrow").mock(
            side_effect=httpx.ConnectError("connection refused")
        )
        with pytest.raises(EscrowClientError) as exc_info:
            await client.create_escrow(ORDER_ID, "500.0000")

    assert exc_info.value.code == "SERVICE_UNAVAILABLE"
