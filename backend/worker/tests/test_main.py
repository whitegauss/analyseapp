import pytest
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


def test_healthz():
    res = client.get("/healthz")
    assert res.status_code == 200
    body = res.json()
    assert body["data"] == {"status": "ok"}
    assert body["error"] is None


def test_analyze_linear_regression_success():
    res = client.post(
        "/analyze",
        json={
            "type": "linear_regression",
            "data": {"columns": {"x": [0, 1, 2, 3, 4], "y": [1, 3, 5, 7, 9]}},
        },
    )
    assert res.status_code == 200
    body = res.json()
    assert body["error"] is None
    assert body["data"]["type"] == "linear_regression"
    assert body["data"]["result"]["slope"] == pytest.approx(2.0)


def test_analyze_unknown_type_returns_400_envelope():
    res = client.post(
        "/analyze",
        json={"type": "not_a_real_type", "data": {"columns": {"x": [0], "y": [0]}}},
    )
    assert res.status_code == 400
    body = res.json()
    assert body["data"] is None
    assert body["error"]["code"] == "unknown_analysis_type"


def test_analyze_missing_column_returns_400_envelope():
    res = client.post(
        "/analyze",
        json={"type": "linear_regression", "data": {"columns": {"x": [0, 1]}}},
    )
    assert res.status_code == 400
    body = res.json()
    assert body["error"]["code"] == "missing_column"


def test_analyze_missing_data_field_returns_400_validation_envelope():
    res = client.post("/analyze", json={"type": "linear_regression"})
    assert res.status_code == 400
    body = res.json()
    assert body["error"]["code"] == "invalid_request"
