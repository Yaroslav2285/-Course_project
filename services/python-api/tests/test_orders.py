# LR #2: Modern Python
# LR #4: Async/Web
from uuid import UUID

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


@pytest.mark.asyncio
async def test_debug_orders(client, auth_headers, session):
    from models.users import User
    from core.security import hash_password
    seller_id = UUID("26ccbf6a-bac2-46b3-8ce5-d2e68f981d4e")
    seller = User(id=seller_id, email="debug_seller@test.com", hashed_password=hash_password("pass"), role="provider")
    session.add(seller)
    await session.commit()
    response = await client.get("/v1/orders/debug", headers=auth_headers)
    assert response.status_code == 200
    data = response.json()
    assert "current_user" in data
    assert "orders_count" in data
    assert data["seller_26ccbf6a"] is not None


@pytest.mark.asyncio
async def test_update_order_status_not_found(client, auth_headers):
    response = await client.patch(
        "/v1/orders/00000000-0000-0000-0000-000000000000/status",
        headers=auth_headers,
        json={"status": "funded"},
    )
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_create_order_with_notes(client, auth_headers, test_user):
    svc_resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": "Notes Service", "price": "50.0000"},
    )
    service_id = svc_resp.json()["data"]["id"]
    response = await client.post(
        "/v1/orders/",
        headers=auth_headers,
        json={
            "service_id": service_id,
            "buyer_id": str(test_user.id),
            "seller_id": str(test_user.id),
            "amount": "50.0000",
            "notes": "Please handle with care",
        },
    )
    assert response.status_code == 201
    assert response.json()["data"]["notes"] == "Please handle with care"


@pytest.mark.asyncio
async def test_order_status_filter(client, auth_headers, test_user):
    svc_resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": "Filter Svc", "price": "10.0000"},
    )
    service_id = svc_resp.json()["data"]["id"]
    await client.post(
        "/v1/orders/",
        headers=auth_headers,
        json={
            "service_id": service_id,
            "buyer_id": str(test_user.id),
            "seller_id": str(test_user.id),
            "amount": "10.0000",
        },
    )
    response = await client.get("/v1/orders/?status=pending", headers=auth_headers)
    assert response.status_code == 200
    assert response.json()["meta"]["total"] >= 1
