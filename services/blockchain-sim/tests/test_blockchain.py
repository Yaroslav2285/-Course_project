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
