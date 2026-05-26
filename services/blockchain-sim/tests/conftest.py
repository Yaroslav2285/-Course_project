import pytest
from fastapi.testclient import TestClient

from main import app, blockchain
from blockchain import Blockchain
from database import init_db, save_chain


@pytest.fixture()
def client(monkeypatch, tmp_path):
    db_file = tmp_path / "test_blockchain.db"
    monkeypatch.setattr("config.settings.db_path", str(db_file))

    new_bc = Blockchain()
    blockchain.chain = new_bc.chain

    init_db()
    save_chain(blockchain.to_dict_list())

    with TestClient(app) as c:
        yield c
