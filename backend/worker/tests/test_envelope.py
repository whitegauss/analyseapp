"""The {data, error, meta} envelope (PDR.md section 8), tested directly.

The Go API passes the worker's body through verbatim, so this shape is a
contract with internal/response on the Go side, not an internal detail:
every response has exactly these three keys, success or not.
"""

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from pydantic import BaseModel

from app import envelope
from app.analysis import AnalysisError, InsufficientDataError

KEYS = {"data", "error", "meta"}


@pytest.mark.parametrize("data", [{"slope": 2.0}, [1, 2], "text", 0, None])
def test_ok_wraps_any_payload_in_exactly_three_keys(data):
    body = envelope.ok(data)

    assert body == {"data": data, "error": None, "meta": {}}
    assert set(body) == KEYS


def test_error_body_has_no_data_and_a_code_and_message():
    body = envelope.error_body("insufficient_data", "at least 2 data points are required")

    assert body == {
        "data": None,
        "error": {"code": "insufficient_data", "message": "at least 2 data points are required"},
        "meta": {},
    }
    assert set(body) == KEYS


def test_meta_is_a_fresh_dict_per_response():
    # A module-level {} shared between calls would let one response's meta
    # leak into every later one. It is a literal today; this catches the
    # "tidy it into a constant" refactor.
    first, second = envelope.ok(1), envelope.error_body("c", "m")
    first["meta"]["page"] = 1

    assert second["meta"] == {}
    assert envelope.ok(2)["meta"] == {}


# --- the exception handlers, on a minimal app of their own ---


class _Body(BaseModel):
    n: int


def _app_raising(exc: Exception) -> TestClient:
    app = FastAPI()
    envelope.register_exception_handlers(app)

    @app.get("/raise")
    async def raise_it():
        raise exc

    @app.post("/validate")
    async def validate(body: _Body):
        return envelope.ok(body.n)

    return TestClient(app, raise_server_exceptions=False)


@pytest.mark.parametrize(
    "exc, code",
    [
        (AnalysisError("generic failure"), "analysis_failed"),
        (InsufficientDataError("at least 2 data points are required"), "insufficient_data"),
    ],
)
def test_analysis_errors_become_400_envelopes_carrying_their_code(exc, code):
    res = _app_raising(exc).get("/raise")

    assert res.status_code == 400
    assert res.json() == {"data": None, "error": {"code": code, "message": str(exc)}, "meta": {}}


def test_request_validation_errors_become_400_invalid_request():
    res = _app_raising(AnalysisError("unused")).post("/validate", json={"n": "not a number"})

    assert res.status_code == 400
    body = res.json()
    assert set(body) == KEYS
    assert body["data"] is None
    assert body["error"]["code"] == "invalid_request"
    # The message names the field, or the caller cannot tell what to fix.
    assert "n" in body["error"]["message"]


def test_other_exceptions_are_not_turned_into_400s():
    # Only the client's mistakes are 400s. Anything else is a bug in the
    # worker and must surface as a server error, not be dressed up as the
    # caller's fault.
    res = _app_raising(RuntimeError("bug")).get("/raise")

    assert res.status_code == 500
