# LR #2: Modern Python
# LR #4: Async/Web
import pytest


@pytest.mark.asyncio
async def test_create_order(client, auth_headers, test_user):
    svc_resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": "Order Service", "price": "150.0000"},
    )
    service_id = svc_resp.json()["data"]["id"]

    response = await client.post(
        "/v1/orders/",
        headers=auth_headers,
        json={
            "service_id": service_id,
            "buyer_id": str(test_user.id),
            "seller_id": str(test_user.id),
            "amount": "150.0000",
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert data["data"]["status"] == "pending"
    assert data["data"]["amount"] == "150.0000"


@pytest.mark.asyncio
async def test_list_orders(client, auth_headers, test_user):
    response = await client.get("/v1/orders/", headers=auth_headers)
    assert response.status_code == 200
    data = response.json()
    assert "data" in data


@pytest.mark.asyncio
async def test_get_order_not_found(client, auth_headers):
    response = await client.get(
        "/v1/orders/00000000-0000-0000-0000-000000000000",
        headers=auth_headers,
    )
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_update_order_status(client, auth_headers, test_user):
    svc_resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": "Status Test", "price": "200.0000"},
    )
    service_id = svc_resp.json()["data"]["id"]

    order_resp = await client.post(
        "/v1/orders/",
        headers=auth_headers,
        json={
            "service_id": service_id,
            "buyer_id": str(test_user.id),
            "seller_id": str(test_user.id),
            "amount": "200.0000",
        },
    )
    order_id = order_resp.json()["data"]["id"]

    response = await client.patch(
        f"/v1/orders/{order_id}/status",
        headers=auth_headers,
        json={"status": "funded"},
    )
    assert response.status_code == 200
    assert response.json()["data"]["status"] == "funded"


@pytest.mark.asyncio
async def test_list_sold_orders(client, auth_headers, test_user):
    response = await client.get("/v1/orders/sold", headers=auth_headers)
    assert response.status_code == 200
