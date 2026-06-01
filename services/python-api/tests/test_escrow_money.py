# LR #2: Modern Python
# LR #4: Async/Web
# LR #3: Financial precision — escrow money flow tests

from decimal import Decimal
from uuid import uuid4

import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from core.db import get_db
from core.security import hash_password
from main import app
from models.orders import Order, OrderStatus
from models.services import Service
from models.users import User
from models.wallet import Wallet, Transaction as WalletTransaction
from repositories.orders import OrderRepository
from repositories.wallets import WalletRepository, ESCROW_USER_ID


@pytest_asyncio.fixture
async def seller(engine) -> User:
    session_factory = async_sessionmaker(
        engine, class_=AsyncSession, expire_on_commit=False
    )
    async with session_factory() as s:
        user = User(
            id=uuid4(),
            email="seller@test.com",
            hashed_password=hash_password("seller123"),
            role="provider",
        )
        s.add(user)
        await s.commit()
        await s.refresh(user)
        return user


async def _create_service(client, auth_headers, title="Test Svc", price="100.0000"):
    resp = await client.post(
        "/v1/services/",
        headers=auth_headers,
        json={"title": title, "price": price},
    )
    return resp.json()["data"]["id"]


async def _create_order(client, auth_headers, service_id, seller_id, buyer_id, amount="100.0000"):
    resp = await client.post(
        "/v1/orders/",
        headers=auth_headers,
        json={
            "service_id": service_id,
            "buyer_id": str(buyer_id),
            "seller_id": str(seller_id),
            "amount": amount,
        },
    )
    return resp.json()["data"]["id"]


async def _topup(client, auth_headers, amount="500.0000"):
    resp = await client.post(
        "/v1/wallet/topup", headers=auth_headers, json={"amount": amount}
    )
    return resp


async def _fund_order(client, auth_headers, order_id):
    resp = await client.post(
        "/v1/wallet/pay",
        headers=auth_headers,
        json={"order_id": order_id},
    )
    return resp


async def _advance_order(client, auth_headers, order_id, status):
    resp = await client.patch(
        f"/v1/orders/{order_id}/status",
        headers=auth_headers,
        json={"status": status},
    )
    return resp


async def _get_balance(client, auth_headers):
    resp = await client.get("/v1/wallet/balance", headers=auth_headers)
    return Decimal(resp.json()["data"]["balance"])


async def _get_wallet_balance_by_user_id(session, user_id):
    stmt = __import__("sqlalchemy").select(Wallet).where(Wallet.user_id == user_id)
    result = await session.execute(stmt)
    wallet = result.scalar_one_or_none()
    return wallet.balance if wallet else Decimal("0")


# === Pay / Fund ===


@pytest.mark.asyncio
async def test_pay_success(client, auth_headers, test_user, session):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, test_user.id, test_user.id)

    await _topup(client, auth_headers, "500.0000")
    balance_before = await _get_balance(client, auth_headers)
    assert balance_before == Decimal("500.0000")

    resp = await _fund_order(client, auth_headers, order_id)
    assert resp.status_code == 201
    data = resp.json()["data"]
    assert Decimal(data["balance"]) == Decimal("400.0000")

    order_resp = await client.get(f"/v1/orders/{order_id}", headers=auth_headers)
    assert order_resp.json()["data"]["status"] == "funded"

    escrow_wallet = await WalletRepository(session).get_escrow_wallet()
    assert escrow_wallet.balance == Decimal("100.0000")

    txns = await session.execute(
        __import__("sqlalchemy").select(WalletTransaction)
        .where(WalletTransaction.type == "escrow_fund")
    )
    txn_list = list(txns.scalars().all())
    assert len(txn_list) == 2


@pytest.mark.asyncio
async def test_pay_insufficient_balance(client, auth_headers, test_user):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, test_user.id, test_user.id)

    resp = await _fund_order(client, auth_headers, order_id)
    assert resp.status_code == 400


@pytest.mark.asyncio
async def test_pay_order_not_found(client, auth_headers):
    resp = await client.post(
        "/v1/wallet/pay",
        headers=auth_headers,
        json={"order_id": "00000000-0000-0000-0000-000000000000"},
    )
    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_pay_not_pending(client, auth_headers, test_user):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, test_user.id, test_user.id)
    await _topup(client, auth_headers, "500.0000")

    await _advance_order(client, auth_headers, order_id, "cancelled")

    resp = await _fund_order(client, auth_headers, order_id)
    assert resp.status_code == 400


@pytest.mark.asyncio
async def test_pay_wrong_buyer(client, auth_headers, test_user):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, test_user.id, test_user.id)
    await _topup(client, auth_headers, "500.0000")

    resp = await client.post(
        "/v1/auth/register",
        json={"email": "other@test.com", "password": "other123", "role": "client"},
    )
    other_token = resp.json()["data"]["access_token"]
    other_headers = {"Authorization": f"Bearer {other_token}"}

    resp = await _fund_order(client, other_headers, order_id)
    assert resp.status_code == 404


# === Cancel / Refund ===


@pytest.mark.asyncio
async def test_cancel_refund_from_funded(client, auth_headers, test_user, session):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, test_user.id, test_user.id)
    await _topup(client, auth_headers, "500.0000")

    await _fund_order(client, auth_headers, order_id)
    buyer_balance = await _get_balance(client, auth_headers)
    assert buyer_balance == Decimal("400.0000")

    resp = await _advance_order(client, auth_headers, order_id, "cancelled")
    assert resp.status_code == 200
    assert resp.json()["data"]["status"] == "cancelled"

    buyer_balance = await _get_balance(client, auth_headers)
    assert buyer_balance == Decimal("500.0000")

    escrow_wallet = await WalletRepository(session).get_escrow_wallet()
    assert escrow_wallet.balance == Decimal("0")


@pytest.mark.asyncio
async def test_cancel_from_pending_no_refund(client, auth_headers, test_user, session):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, test_user.id, test_user.id)

    resp = await _advance_order(client, auth_headers, order_id, "cancelled")
    assert resp.status_code == 200

    escrow_wallet = await WalletRepository(session).get_escrow_wallet()
    assert escrow_wallet.balance == Decimal("0")


# === Release / Payout ===


@pytest.mark.asyncio
async def test_release_payout_no_double_via_status_patch(client, auth_headers, test_user, seller, session):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, seller.id, test_user.id)
    await _topup(client, auth_headers, "500.0000")
    await _fund_order(client, auth_headers, order_id)

    seller_balance_before = await _get_wallet_balance_by_user_id(session, seller.id)

    resp = await _advance_order(client, auth_headers, order_id, "released")
    assert resp.status_code == 200

    seller_balance_after = await _get_wallet_balance_by_user_id(session, seller.id)
    assert seller_balance_after == seller_balance_before

    escrow_wallet = await WalletRepository(session).get_escrow_wallet()
    assert escrow_wallet.balance == Decimal("100.0000")


@pytest.mark.asyncio
async def test_release_payout_via_escrow_proxy(client, auth_headers, test_user, seller, session):
    svc_id = await _create_service(client, auth_headers)
    order_id = await _create_order(client, auth_headers, svc_id, seller.id, test_user.id)
    await _topup(client, auth_headers, "500.0000")

    await _fund_order(client, auth_headers, order_id)
    await _advance_order(client, auth_headers, order_id, "in_progress")
    await _advance_order(client, auth_headers, order_id, "completed")

    seller_balance_before = await _get_wallet_balance_by_user_id(session, seller.id)

    resp = await client.post(
        f"/v1/escrow/{order_id}/release",
        headers=auth_headers,
    )
    assert resp.status_code == 200
    assert resp.json()["data"]["status"] == "released"

    seller_balance_after = await _get_wallet_balance_by_user_id(session, seller.id)
    assert seller_balance_after == seller_balance_before + Decimal("100.0000")

    escrow_wallet = await WalletRepository(session).get_escrow_wallet()
    assert escrow_wallet.balance == Decimal("0")


# === Transfer unit test ===


@pytest.mark.asyncio
async def test_transfer_insufficient_balance(session):
    wallet_repo = WalletRepository(session)
    user_a = uuid4()
    user_b = uuid4()

    wallet_a = await wallet_repo.get_or_create(user_a)
    await wallet_repo.update_balance(wallet_a, Decimal("50.0000"))

    with pytest.raises(ValueError, match="Insufficient balance"):
        await wallet_repo.transfer(
            from_user_id=user_a,
            to_user_id=user_b,
            amount=Decimal("100.0000"),
        )


# === Auth required ===


@pytest.mark.asyncio
async def test_pay_requires_auth(client):
    resp = await client.post(
        "/v1/wallet/pay",
        json={"order_id": "00000000-0000-0000-0000-000000000000"},
    )
    assert resp.status_code == 401
