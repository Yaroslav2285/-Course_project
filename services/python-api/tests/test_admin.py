import pytest
from uuid import uuid4

from models.orders import Order
from models.services import Service
from models.users import User
from core.security import hash_password


@pytest.mark.asyncio
async def test_admin_list_disputes(client, session):
    admin = User(
        id=uuid4(), email="admin@test.com",
        hashed_password=hash_password("admin123"), role="admin",
    )
    session.add(admin)
    buyer = User(
        id=uuid4(), email="buyer@test.com",
        hashed_password=hash_password("pass123"), role="client",
    )
    session.add(buyer)
    seller = User(
        id=uuid4(), email="seller@test.com",
        hashed_password=hash_password("pass123"), role="provider",
    )
    session.add(seller)
    svc = Service(provider_id=seller.id, title="Test", price="100.0000", status="published")
    session.add(svc)
    await session.flush()
    order = Order(service_id=svc.id, buyer_id=buyer.id, seller_id=seller.id, amount="100.0000", status="disputed")
    session.add(order)
    await session.commit()

    admin_resp = await client.post(
        "/v1/auth/login", json={"email": "admin@test.com", "password": "admin123"}
    )
    admin_token = admin_resp.json()["data"]["access_token"]
    admin_headers = {"Authorization": f"Bearer {admin_token}"}

    response = await client.get("/v1/admin/disputes", headers=admin_headers)
    assert response.status_code == 200
    data = response.json()
    assert data["meta"]["total"] >= 1
    assert data["data"][0]["status"] == "disputed"


@pytest.mark.asyncio
async def test_admin_disputes_forbidden(client, auth_headers):
    response = await client.get("/v1/admin/disputes", headers=auth_headers)
    assert response.status_code == 403
