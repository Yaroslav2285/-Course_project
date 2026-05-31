# LR #2: Modern Python
# LR #4: Async/Web

import pytest


@pytest.mark.asyncio
async def test_get_balance_creates_wallet(client, auth_headers):
    response = await client.get("/v1/wallet/balance", headers=auth_headers)
    assert response.status_code == 200
    data = response.json()
    assert data["data"]["balance"] == "0.0000"
    assert "id" in data["data"]
    assert "user_id" in data["data"]


@pytest.mark.asyncio
async def test_get_balance_returns_existing_wallet(client, auth_headers):
    resp1 = await client.get("/v1/wallet/balance", headers=auth_headers)
    assert resp1.status_code == 200
    wallet_id = resp1.json()["data"]["id"]

    resp2 = await client.get("/v1/wallet/balance", headers=auth_headers)
    assert resp2.status_code == 200
    assert resp2.json()["data"]["id"] == wallet_id


@pytest.mark.asyncio
async def test_topup_increases_balance(client, auth_headers):
    await client.get("/v1/wallet/balance", headers=auth_headers)

    response = await client.post(
        "/v1/wallet/topup",
        headers=auth_headers,
        json={"amount": "100.0000"},
    )
    assert response.status_code == 201
    data = response.json()["data"]
    assert data["balance"] == "100.0000"
    assert data["transaction"]["status"] == "success"
    assert data["transaction"]["type"] == "topup"

    balance_resp = await client.get("/v1/wallet/balance", headers=auth_headers)
    assert balance_resp.json()["data"]["balance"] == "100.0000"


@pytest.mark.asyncio
async def test_topup_negative_amount_rejected(client, auth_headers):
    response = await client.post(
        "/v1/wallet/topup",
        headers=auth_headers,
        json={"amount": "-50.0000"},
    )
    assert response.status_code == 422


@pytest.mark.asyncio
async def test_topup_zero_amount_rejected(client, auth_headers):
    response = await client.post(
        "/v1/wallet/topup",
        headers=auth_headers,
        json={"amount": "0.0000"},
    )
    assert response.status_code == 422


@pytest.mark.asyncio
async def test_multiple_topups_sum(client, auth_headers):
    await client.get("/v1/wallet/balance", headers=auth_headers)

    await client.post(
        "/v1/wallet/topup", headers=auth_headers, json={"amount": "200.0000"}
    )
    await client.post(
        "/v1/wallet/topup", headers=auth_headers, json={"amount": "300.0000"}
    )

    balance_resp = await client.get("/v1/wallet/balance", headers=auth_headers)
    assert balance_resp.json()["data"]["balance"] == "500.0000"


@pytest.mark.asyncio
async def test_transactions_empty(client, auth_headers):
    await client.get("/v1/wallet/balance", headers=auth_headers)

    response = await client.get("/v1/wallet/transactions", headers=auth_headers)
    assert response.status_code == 200
    data = response.json()
    assert data["data"] == []
    assert data["meta"]["total"] == 0


@pytest.mark.asyncio
async def test_transactions_list(client, auth_headers):
    await client.get("/v1/wallet/balance", headers=auth_headers)

    await client.post(
        "/v1/wallet/topup", headers=auth_headers, json={"amount": "150.0000"}
    )

    response = await client.get("/v1/wallet/transactions", headers=auth_headers)
    assert response.status_code == 200
    data = response.json()
    assert len(data["data"]) == 1
    assert data["data"][0]["type"] == "topup"
    assert data["data"][0]["amount"] == "150.0000"
    assert data["data"][0]["status"] == "success"


@pytest.mark.asyncio
async def test_transactions_pagination(client, auth_headers):
    await client.get("/v1/wallet/balance", headers=auth_headers)

    for i in range(3):
        await client.post(
            "/v1/wallet/topup", headers=auth_headers, json={"amount": "10.0000"}
        )

    response = await client.get(
        "/v1/wallet/transactions?limit=2&offset=0", headers=auth_headers
    )
    assert response.status_code == 200
    data = response.json()
    assert len(data["data"]) == 2
    assert data["meta"]["total"] == 3


@pytest.mark.asyncio
async def test_wallet_requires_auth(client):
    response = await client.get("/v1/wallet/balance")
    assert response.status_code == 401

    response = await client.post("/v1/wallet/topup", json={"amount": "100.0000"})
    assert response.status_code == 401

    response = await client.get("/v1/wallet/transactions")
    assert response.status_code == 401
