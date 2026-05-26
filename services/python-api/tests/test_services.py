# LR #2: Modern Python
# LR #4: Async/Web
import pytest


@pytest.mark.asyncio
async def test_create_service(client, auth_headers):
    response = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={
            "title": "Test Service",
            "description": "A test service",
            "price": "99.9900",
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert data["data"]["title"] == "Test Service"
    assert data["data"]["status"] == "draft"
    assert data["data"]["price"] == "99.9900"


@pytest.mark.asyncio
async def test_list_services(client):
    response = await client.get("/v1/services/")
    assert response.status_code == 200
    data = response.json()
    assert "data" in data


@pytest.mark.asyncio
async def test_list_services_pagination(client):
    response = await client.get("/v1/services/?limit=5&offset=0")
    assert response.status_code == 200
    data = response.json()
    assert data["meta"]["limit"] == 5
    assert data["meta"]["offset"] == 0


@pytest.mark.asyncio
async def test_get_service_not_found(client):
    response = await client.get(
        "/v1/services/00000000-0000-0000-0000-000000000000"
    )
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_update_service(client, auth_headers):
    create_resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": "Old Title", "price": "50.0000"},
    )
    service_id = create_resp.json()["data"]["id"]
    response = await client.put(
        f"/v1/services/{service_id}",
        headers=auth_headers,
        json={"title": "New Title"},
    )
    assert response.status_code == 200
    assert response.json()["data"]["title"] == "New Title"


@pytest.mark.asyncio
async def test_delete_service(client, auth_headers):
    create_resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": "To Delete", "price": "10.0000"},
    )
    service_id = create_resp.json()["data"]["id"]
    response = await client.delete(
        f"/v1/services/{service_id}", headers=auth_headers
    )
    assert response.status_code == 200
    get_resp = await client.get(f"/v1/services/{service_id}")
    assert get_resp.status_code == 404


@pytest.mark.asyncio
async def test_cannot_update_others_service(client, auth_headers, test_user, session):
    create_resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": "My Service", "price": "30.0000"},
    )
    service_id = create_resp.json()["data"]["id"]
    from uuid import uuid4
    from models.users import User
    from core.security import hash_password
    another_user = User(
        id=uuid4(),
        email="other@example.com",
        hashed_password=hash_password("password123"),
        role="client",
    )
    session.add(another_user)
    await session.commit()
    other_resp = await client.post(
        "/v1/auth/login",
        json={"email": "other@example.com", "password": "password123"},
    )
    other_token = other_resp.json()["data"]["access_token"]
    other_headers = {"Authorization": f"Bearer {other_token}"}
    response = await client.put(
        f"/v1/services/{service_id}",
        headers=other_headers,
        json={"title": "Hacked Title"},
    )
    assert response.status_code == 403
