import json
import sqlite3
import re

from config import settings
from database import load_chain
from main import blockchain


class TestBlockchainSim:
    def test_submit_block(self, client):
        response = client.post(
            "/v1/chain/submit",
            json={
                "order_id": "ord-001",
                "action": "created",
                "data": {"amount": 100},
            },
        )
        assert response.status_code == 201
        data = response.json()
        assert data["block_index"] == 1
        assert len(data["tx_hash"]) == 64
        assert len(data["prev_hash"]) == 64
        assert data["prev_hash"] != "0" * 64

    def test_verify_valid_chain(self, client):
        client.post(
            "/v1/chain/submit", json={"order_id": "ord-002", "action": "funded"}
        )
        response = client.get("/v1/chain/verify")
        assert response.status_code == 200
        body = response.json()
        assert body["valid"] is True
        assert body["blocks_count"] >= 2

    def test_audit_log_single(self, client):
        oid = "audit-001"
        client.post("/v1/chain/submit", json={"order_id": oid, "action": "created"})
        client.post("/v1/chain/submit", json={"order_id": oid, "action": "funded"})
        client.post("/v1/chain/submit", json={"order_id": "other", "action": "created"})

        response = client.get(f"/v1/chain/audit/{oid}")
        assert response.status_code == 200
        blocks = response.json()
        assert len(blocks) == 2
        for b in blocks:
            txs = b["transactions"]
            assert any(tx.get("order_id") == oid for tx in txs)

    def test_audit_log_not_found_404(self, client):
        response = client.get("/v1/chain/audit/nonexistent")
        assert response.status_code == 404

    def test_submit_invalid_schema_422(self, client):
        response = client.post("/v1/chain/submit", json={"invalid": "data"})
        assert response.status_code == 422

    def test_submit_empty_body_422(self, client):
        response = client.post("/v1/chain/submit", json={})
        assert response.status_code == 422

    def test_submit_with_tampered_chain_returns_400(self, client):
        client.post(
            "/v1/chain/submit", json={"order_id": "ord-003", "action": "created"}
        )

        from main import blockchain as bc

        bc.chain[0].hash = "tampered"

        response = client.post(
            "/v1/chain/submit",
            json={
                "order_id": "ord-004",
                "action": "funded",
            },
        )
        assert response.status_code == 400
        assert "integrity" in response.json()["detail"].lower()

    def test_chain_invalid_after_tamper(self, client):
        client.post(
            "/v1/chain/submit", json={"order_id": "tamper-1", "action": "created"}
        )

        from main import blockchain as bc

        bc.chain[1].transactions = [{"malicious": "data"}]

        response = client.get("/v1/chain/verify")
        assert response.status_code == 200
        assert response.json()["valid"] is False

    def test_health_endpoint(self, client):
        response = client.get("/health")
        assert response.status_code == 200
        assert response.json()["status"] == "ok"

    def test_verify_empty_chain(self, client):
        response = client.get("/v1/chain/verify")
        assert response.status_code == 200
        assert response.json()["valid"] is True
        assert response.json()["blocks_count"] == 1

    def test_hash_chain_linkage(self, client):
        client.post("/v1/chain/submit", json={"order_id": "link-1", "action": "a"})
        client.post("/v1/chain/submit", json={"order_id": "link-1", "action": "b"})
        client.post("/v1/chain/submit", json={"order_id": "link-1", "action": "c"})

        response = client.get("/v1/chain/audit/link-1")
        blocks = response.json()
        assert len(blocks) == 3
        for i in range(1, len(blocks)):
            assert blocks[i]["prev_hash"] == blocks[i - 1]["hash"]

    def test_timestamp_iso8601(self, client):
        client.post("/v1/chain/submit", json={"order_id": "ts-1", "action": "test"})

        response = client.get("/v1/chain/audit/ts-1")
        ts = response.json()[0]["timestamp"]
        assert re.match(
            r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}", ts
        ), f"Timestamp not ISO8601: {ts}"

    def test_genesis_block_exists(self, client):
        response = client.get("/v1/chain/verify")
        assert response.json()["blocks_count"] >= 1
        genesis = blockchain.chain[0]
        assert genesis.index == 0
        assert genesis.prev_hash == "0" * 64
        assert genesis.transactions == [{"genesis": True}]
        assert genesis.nonce == 0

    def test_block_index_increments(self, client):
        r1 = client.post(
            "/v1/chain/submit", json={"order_id": "idx-1", "action": "a"}
        ).json()
        r2 = client.post(
            "/v1/chain/submit", json={"order_id": "idx-1", "action": "b"}
        ).json()
        r3 = client.post(
            "/v1/chain/submit", json={"order_id": "idx-1", "action": "c"}
        ).json()

        assert r1["block_index"] == 1
        assert r2["block_index"] == 2
        assert r3["block_index"] == 3

    def test_audit_multiple_orders_separate(self, client):
        client.post("/v1/chain/submit", json={"order_id": "A", "action": "created"})
        client.post("/v1/chain/submit", json={"order_id": "B", "action": "created"})
        client.post("/v1/chain/submit", json={"order_id": "A", "action": "funded"})
        client.post("/v1/chain/submit", json={"order_id": "C", "action": "created"})

        audit_a = client.get("/v1/chain/audit/A").json()
        audit_b = client.get("/v1/chain/audit/B").json()
        audit_c = client.get("/v1/chain/audit/C").json()

        assert len(audit_a) == 2
        assert len(audit_b) == 1
        assert len(audit_c) == 1

        for b in audit_a:
            txs = b["transactions"]
            assert all(tx.get("order_id") == "A" for tx in txs)

    def test_submit_response_has_nonce(self, client):
        response = client.post(
            "/v1/chain/submit", json={"order_id": "nonce-1", "action": "test"}
        )
        assert response.status_code == 201

        oid_resp = client.get("/v1/chain/audit/nonce-1").json()
        assert oid_resp[0]["nonce"] == 0

    def test_db_persistence(self, client):
        client.post("/v1/chain/submit", json={"order_id": "persist-1", "action": "a"})
        client.post("/v1/chain/submit", json={"order_id": "persist-1", "action": "b"})

        saved = load_chain()
        assert len(saved) == 3

        from blockchain import Blockchain

        recovered = Blockchain()
        recovered.load_from_db(saved)
        assert recovered.is_valid_chain()

    def test_db_tamper_detected(self, client):
        client.post(
            "/v1/chain/submit", json={"order_id": "tamper-db", "action": "created"}
        )

        path = settings.db_path
        conn = sqlite3.connect(path)
        rows = conn.execute("SELECT data FROM blocks ORDER BY id").fetchall()
        blocks_data = [json.loads(r[0]) for r in rows]
        blocks_data[1]["transactions"] = [{"malicious": "db_tamper"}]
        conn.execute("DELETE FROM blocks")
        for b in blocks_data:
            conn.execute("INSERT INTO blocks (data) VALUES (?)", (json.dumps(b),))
        conn.commit()
        conn.close()

        from main import blockchain as bc

        reloaded = load_chain()
        bc.load_from_db(reloaded)

        response = client.get("/v1/chain/verify")
        assert response.status_code == 200
        assert response.json()["valid"] is False

    def test_submit_422_missing_order_id(self, client):
        response = client.post("/v1/chain/submit", json={"action": "test"})
        assert response.status_code == 422
        errors = response.json()["detail"]
        assert any(e["loc"][-1] == "order_id" for e in errors)

    def test_submit_422_missing_action(self, client):
        response = client.post("/v1/chain/submit", json={"order_id": "test"})
        assert response.status_code == 422
        errors = response.json()["detail"]
        assert any(e["loc"][-1] == "action" for e in errors)

    def test_submit_minimal_order_id(self, client):
        response = client.post(
            "/v1/chain/submit", json={"order_id": "a", "action": "x"}
        )
        assert response.status_code == 201

    def test_submit_special_chars_in_action(self, client):
        response = client.post(
            "/v1/chain/submit", json={"order_id": "spec", "action": "!@#$%^&*()"}
        )
        assert response.status_code == 201

    def test_submit_nested_data(self, client):
        response = client.post(
            "/v1/chain/submit",
            json={
                "order_id": "nest",
                "action": "test",
                "data": {"nested": {"deep": [1, 2, 3]}},
            },
        )
        assert response.status_code == 201

    def test_submit_preserves_data_unchanged(self, client):
        client.post(
            "/v1/chain/submit",
            json={"order_id": "data-1", "action": "test", "data": {"key": "value"}},
        )
        resp = client.get("/v1/chain/audit/data-1").json()
        assert resp[0]["transactions"][0]["data"] == {"key": "value"}
        assert resp[0]["transactions"][0]["action"] == "test"
        assert resp[0]["transactions"][0]["order_id"] == "data-1"

    def test_env_config_override(self, monkeypatch, tmp_path):
        monkeypatch.setattr("config.settings.db_path", str(tmp_path / "custom.db"))
        monkeypatch.setattr("config.settings.log_level", "DEBUG")

        from config import settings as cfg

        assert "custom.db" in cfg.db_path
        assert cfg.log_level == "DEBUG"

    def test_concurrent_blocks_stay_valid(self, client):
        for i in range(5):
            client.post(
                "/v1/chain/submit", json={"order_id": f"conc-{i}", "action": "test"}
            )
        response = client.get("/v1/chain/verify")
        assert response.json()["valid"] is True
        assert response.json()["blocks_count"] == 6
