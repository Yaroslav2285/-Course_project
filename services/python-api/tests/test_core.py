import pytest
from fastapi import HTTPException


class TestResponses:
    def test_error_response(self):
        from core.responses import error_response
        resp = error_response(code="TEST", detail="Test error", fields=[{"field": "x", "message": "required"}])
        assert resp["errors"][0]["code"] == "TEST"
        assert resp["data"] is None
        assert resp["errors"][0]["fields"] == [{"field": "x", "message": "required"}]

    def test_success_response_with_none(self):
        from core.responses import success_response
        resp = success_response(data=None, total=0, limit=10, offset=0)
        assert resp["data"] is None
        assert resp["meta"]["total"] == 0


@pytest.mark.asyncio
async def test_exception_handlers():
    from core.exceptions import http_exception_handler, unhandled_exception_handler
    from starlette.requests import Request
    scope = {"type": "http", "method": "GET", "path": "/test", "headers": []}
    req = Request(scope)
    exc = HTTPException(status_code=404, detail="Not found")
    resp = await http_exception_handler(req, exc)
    assert resp.status_code == 404

    exc2 = ValueError("test")
    resp2 = await unhandled_exception_handler(req, exc2)
    assert resp2.status_code == 500
