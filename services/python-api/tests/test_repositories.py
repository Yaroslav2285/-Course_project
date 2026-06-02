import pytest
from uuid import uuid4

from repositories.wallets import WalletRepository, TransactionRepository
from repositories.services import ServiceRepository


@pytest.mark.asyncio
async def test_service_repo_list_published(session):
    repo = ServiceRepository(session)
    from models.services import Service
    from models.users import User
    from core.security import hash_password
    user = User(id=uuid4(), email="svc_owner@test.com", hashed_password=hash_password("pass"), role="provider")
    session.add(user)
    await session.flush()
    s1 = Service(provider_id=user.id, title="Pub", price="10.0000", status="published")
    s2 = Service(provider_id=user.id, title="Draft", price="20.0000", status="draft")
    session.add_all([s1, s2])
    await session.flush()
    items, total = await repo.list_published()
    assert total == 1
    assert items[0].title == "Pub"


@pytest.mark.asyncio
async def test_service_repo_update_no_changes(session):
    repo = ServiceRepository(session)
    from models.services import Service
    from models.users import User
    from core.security import hash_password
    user = User(id=uuid4(), email="svc_owner2@test.com", hashed_password=hash_password("pass"), role="provider")
    session.add(user)
    await session.flush()
    svc = Service(provider_id=user.id, title="No Change", price="30.0000", status="draft")
    session.add(svc)
    await session.flush()
    result = await repo.update_service(svc)
    assert result.title == "No Change"


@pytest.mark.asyncio
async def test_wallet_get_or_create_race(session):
    repo = WalletRepository(session)
    user_id = uuid4()
    w1 = await repo.get_or_create(user_id)
    assert w1.balance == 0
    w2 = await repo.get_or_create(user_id)
    assert w2.id == w1.id


@pytest.mark.asyncio
async def test_transaction_get_by_reference(session):
    wallet_repo = WalletRepository(session)
    txn_repo = TransactionRepository(session)
    user_id = uuid4()
    wallet = await wallet_repo.get_or_create(user_id)
    txn = await txn_repo.create_transaction(
        wallet_id=wallet.id, type="transfer", amount=50, status="success", reference_id=uuid4(),
    )
    found = await txn_repo.get_by_reference(
        reference_id=txn.reference_id, txn_type="transfer", status="success",
    )
    assert found is not None
    assert found.id == txn.id


@pytest.mark.asyncio
async def test_user_repo_update_with_password(session):
    from repositories.users import UserRepository
    from models.users import User
    from core.security import hash_password
    repo = UserRepository(session)
    user_id = uuid4()
    user = User(id=user_id, email="upd_pass@test.com", hashed_password=hash_password("oldpass"), role="client")
    session.add(user)
    await session.flush()
    old_hash = user.hashed_password
    result = await repo.update_user(user, email="upd_pass@test.com", password="newpass")
    assert result.hashed_password != old_hash


@pytest.mark.asyncio
async def test_user_repo_update_no_changes(session):
    from repositories.users import UserRepository
    from models.users import User
    from core.security import hash_password
    repo = UserRepository(session)
    user = User(id=uuid4(), email="nochange@test.com", hashed_password=hash_password("pass"), role="client")
    session.add(user)
    await session.flush()
    result = await repo.update_user(user, email="nochange@test.com")
    assert result.email == "nochange@test.com"


@pytest.mark.asyncio
async def test_order_repo_list_by_seller_status(session):
    from repositories.orders import OrderRepository
    from models.orders import Order
    from models.users import User
    from models.services import Service
    from core.security import hash_password
    buyer = User(id=uuid4(), email="buyer@test.com", hashed_password=hash_password("pass"), role="client")
    seller = User(id=uuid4(), email="seller@test.com", hashed_password=hash_password("pass"), role="provider")
    session.add_all([buyer, seller])
    await session.flush()
    svc = Service(provider_id=seller.id, title="Test Svc", price="10.0000", status="published")
    session.add(svc)
    await session.flush()
    o1 = Order(service_id=svc.id, buyer_id=buyer.id, seller_id=seller.id, amount="10.0000", status="pending")
    o2 = Order(service_id=svc.id, buyer_id=buyer.id, seller_id=seller.id, amount="20.0000", status="funded")
    session.add_all([o1, o2])
    await session.flush()
    repo = OrderRepository(session)
    items, total = await repo.list_by_seller(seller_id=seller.id, status="pending")
    assert total == 1
    assert items[0].status == "pending"
