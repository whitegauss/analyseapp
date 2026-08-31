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


# --------------------------------------------------------------------------
# 境界値が envelope でどう見えるか (KAN-45)
#
# 既定の TestClient は未処理例外を送出してしまうので、実サーバと同じ 500 応答
# として観測するためのクライアントを別に用意する。
# --------------------------------------------------------------------------

server_error_client = TestClient(app, raise_server_exceptions=False)


@pytest.mark.parametrize(
    "columns",
    [
        {"x": [1.0, 1.0, 1.0], "y": [1.0, 2.0, 3.0]},
        {"x": [1.0, 2.0], "y": [1.0, 2.0]},
        {"x": [0.0, 1.0, 2.0], "y": [1.0, 3.0, 5.0], "y_error": [0.1, 0.0, 0.1]},
    ],
    ids=["x-zero-variance", "exactly-two-points", "y-error-contains-zero"],
)
def test_analyze_degenerate_input_returns_bare_500_not_envelope(columns):
    res = server_error_client.post(
        "/analyze",
        json={"type": "linear_regression", "data": {"columns": columns}},
    )

    # BUG(現状固定): numpy の例外が AnalysisError ではないため envelope の
    # ハンドラに捕まらず、{data, error, meta} ですらない素の 500 が返る。
    assert res.status_code == 500
    assert res.text == "Internal Server Error"


def test_analyze_nan_result_fails_during_serialization():
    # NaN は JSON リテラルとしては送れてしまうため pydantic を通過し、解析も
    # 成功する。落ちるのは NaN 入りの結果を json.dumps する応答生成の段階。
    res = server_error_client.post(
        "/analyze",
        content=b'{"type":"linear_regression","data":{"columns":{"x":[1,2,3],"y":[1,NaN,3]}}}',
        headers={"content-type": "application/json"},
    )

    assert res.status_code == 500
    assert res.text == "Internal Server Error"


def test_analyze_single_point_returns_400_with_log_scale_wording():
    res = client.post(
        "/analyze",
        json={"type": "linear_regression", "data": {"columns": {"x": [1.0], "y": [1.0]}}},
    )

    assert res.status_code == 400
    body = res.json()
    assert body["error"]["code"] == "insufficient_data"
    # BUG(現状固定): log スケールを要求していないのに log 前提の文言が返る。
    assert "log-scale fit" in body["error"]["message"]
