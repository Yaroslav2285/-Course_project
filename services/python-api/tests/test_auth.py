# LR #2: Modern Python
# LR #4: Async/Web
import pytest


@pytest.mark.asyncio
async def test_register_success(client):
    response = await client.post(
        "/v1/auth/register",
        json={
            "email": "newuser@example.com",
            "password": "securepass123",
            "role": "client",
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert data["errors"] is None
    assert data["data"]["access_token"]
    assert data["data"]["refresh_token"]
    assert data["data"]["user"]["email"] == "newuser@example.com"


@pytest.mark.asyncio
async def test_register_duplicate(client, test_user):
    response = await client.post(
        "/v1/auth/register",
        json={
            "email": "test@example.com",
            "password": "password123",
            "role": "client",
        },
    )
    assert response.status_code == 409


@pytest.mark.asyncio
async def test_login_success(client, test_user):
    response = await client.post(
        "/v1/auth/login",
        json={"email": "test@example.com", "password": "password123"},
    )
    assert response.status_code == 200
    data = response.json()
    assert data["data"]["access_token"]
    assert data["data"]["token_type"] == "bearer"


@pytest.mark.asyncio
async def test_login_invalid_password(client, test_user):
    response = await client.post(
        "/v1/auth/login",
        json={"email": "test@example.com", "password": "wrongpass"},
    )
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_login_nonexistent(client):
    response = await client.post(
        "/v1/auth/login",
        json={"email": "nobody@example.com", "password": "password123"},
    )
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_refresh_token(client, test_user):
    login_resp = await client.post(
        "/v1/auth/login",
        json={"email": "test@example.com", "password": "password123"},
    )
    refresh_token = login_resp.json()["data"]["refresh_token"]
    response = await client.post(
        "/v1/auth/refresh",
        json={"refresh_token": refresh_token},
    )
    assert response.status_code == 200
    data = response.json()
    assert data["data"]["access_token"]


@pytest.mark.asyncio
async def test_refresh_invalid(client):
    response = await client.post(
        "/v1/auth/refresh",
        json={"refresh_token": "invalid_token"},
    )
    assert response.status_code == 401
