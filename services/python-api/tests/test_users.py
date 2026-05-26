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
async def test_update_me_duplicate_email(client, auth_headers, test_user):
    response = await client.put(
        "/v1/users/me",
        headers=auth_headers,
        json={"email": "test@example.com"},
    )
    assert response.status_code in (200, 409)
