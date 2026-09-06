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
    "columns, expected_code",
    [
        ({"x": [1.0, 1.0, 1.0], "y": [1.0, 2.0, 3.0]}, "degenerate_input"),
        (
            {"x": [0.0, 1.0, 2.0], "y": [1.0, 3.0, 5.0], "y_error": [0.1, 0.0, 0.1]},
            "degenerate_input",
        ),
        (
            {"x": [0.0, 1.0, 2.0], "y": [1.0, 3.0, 5.0], "y_error": [0.1, -0.1, 0.1]},
            "degenerate_input",
        ),
        ({"x": [1.0, 2.0, 3.0], "y": [1.0, 2.0, 3.0], "y_error": [0.1, 0.1, 0.1]}, None),
    ],
    ids=[
        "x-zero-variance",
        "y-error-contains-zero",
        "y-error-contains-negative",
        "control-well-formed",
    ],
)
def test_analyze_degenerate_input_returns_400_in_the_envelope(columns, expected_code):
    # These are all failures of what the client sent, so they belong in the
    # envelope as a 400. Reaching numpy instead produced a bare LinAlgError,
    # which is not an AnalysisError: it escaped the handler and came back as
    # a body that was not even {data, error, meta} (KAN-57 / KAN-60).
    res = server_error_client.post(
        "/analyze",
        json={"type": "linear_regression", "data": {"columns": columns}},
    )

    body = res.json()
    if expected_code is None:
        assert res.status_code == 200, body
        assert body["error"] is None
        return

    assert res.status_code == 400, body
    assert body["error"]["code"] == expected_code
    assert body["data"] is None
    # The message has to say which column and which constraint, or the user
    # cannot tell which of their columns to go and fix.
    assert "y_error" in body["error"]["message"] or "'x'" in body["error"]["message"]


def test_analyze_exactly_two_points_returns_the_fit_without_standard_errors():
    # Two points define a line exactly; what they cannot give is a spread
    # around it. Refusing the whole fit would be less useful than reporting
    # the line and saying the uncertainty is unknown (KAN-57).
    res = client.post(
        "/analyze",
        json={
            "type": "linear_regression",
            "data": {"columns": {"x": [1.0, 2.0], "y": [1.0, 3.0]}},
        },
    )

    assert res.status_code == 200, res.text
    data = res.json()["data"]["result"]
    assert data["slope"] == pytest.approx(2.0)
    assert data["intercept"] == pytest.approx(-1.0)
    assert data["slope_stderr"] is None
    assert data["intercept_stderr"] is None


def test_analyze_non_finite_input_is_rejected_before_serialization():
    # NaN is not valid JSON but is accepted by the parser, so it used to
    # reach the analysis, produce a NaN-filled result, and only fail when the
    # response could not be serialised -- as a bare 500 (KAN-57).
    res = server_error_client.post(
        "/analyze",
        content=b'{"type":"linear_regression","data":{"columns":{"x":[1,2,3],"y":[1,NaN,3]}}}',
        headers={"content-type": "application/json"},
    )

    assert res.status_code == 400, res.text
    body = res.json()
    assert body["error"]["code"] == "degenerate_input"
    assert "'y'" in body["error"]["message"]


def test_analyze_single_point_does_not_mention_log_scale():
    res = client.post(
        "/analyze",
        json={"type": "linear_regression", "data": {"columns": {"x": [1.0], "y": [1.0]}}},
    )

    assert res.status_code == 400
    body = res.json()
    assert body["error"]["code"] == "insufficient_data"
    # No log scale was asked for, so naming one sends the user looking for a
    # setting they never touched (KAN-59).
    assert "log" not in body["error"]["message"]
    assert "at least 2 data points" in body["error"]["message"]


def test_analyze_single_point_with_log_still_explains_the_filter():
    res = client.post(
        "/analyze",
        json={
            "type": "linear_regression",
            "data": {"columns": {"x": [1.0, -1.0], "y": [1.0, 2.0]}},
            "params": {"x_log": True},
        },
    )

    assert res.status_code == 400
    body = res.json()
    assert body["error"]["code"] == "insufficient_data"
    # Here the log filter really is why a point went missing, and saying so
    # is the useful part of the old wording.
    assert "log-scale fit" in body["error"]["message"]
