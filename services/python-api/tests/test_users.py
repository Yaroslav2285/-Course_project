# LR #2: Modern Python
# LR #4: Async/Web
import pytest


@pytest.mark.asyncio
async def test_get_me(client, auth_headers):
    response = await client.get("/v1/users/me", headers=auth_headers)
    assert response.status_code == 200
    data = response.json()
    assert data["data"]["email"] == "test@example.com"
    assert data["data"]["role"] == "client"


@pytest.mark.asyncio
async def test_get_me_unauthorized(client):
    response = await client.get("/v1/users/me")
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_update_me(client, auth_headers):
    response = await client.put(
        "/v1/users/me",
        headers=auth_headers,
        json={"email": "updated@example.com"},
    )
    assert response.status_code == 200
    data = response.json()
    assert data["data"]["email"] == "updated@example.com"


@pytest.mark.asyncio
async def test_update_me_duplicate_email(client, auth_headers, test_user, session):
    from uuid import uuid4
    from models.users import User
    from core.security import hash_password
    other = User(
        id=uuid4(), email="other@test.com",
        hashed_password=hash_password("pass123"), role="client",
    )
    session.add(other)
    await session.commit()
    resp = await client.put(
        "/v1/users/me",
        headers=auth_headers,
        json={"email": "other@test.com"},
    )
    assert resp.status_code == 409


@pytest.mark.asyncio
async def test_get_me_invalid_token(client):
    response = await client.get("/v1/users/me", headers={"Authorization": "Bearer invalidtoken"})
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_get_user_by_id(client, auth_headers, test_user):
    response = await client.get(f"/v1/users/{test_user.id}", headers=auth_headers)
    assert response.status_code == 200
    assert response.json()["data"]["email"] == "test@example.com"


@pytest.mark.asyncio
async def test_get_user_not_found(client, auth_headers):
    from uuid import UUID
    response = await client.get(
        f"/v1/users/{UUID('{00000000-0000-0000-0000-000000000000}')}",
        headers=auth_headers,
    )
    assert response.status_code == 404
