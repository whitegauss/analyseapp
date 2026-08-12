"""Python analysis worker (PDR.md section 3/9). HTTP skeleton for now;
numeric analysis endpoints and a possible gRPC front are added in later work.
"""

import time
import uuid

import structlog
from fastapi import FastAPI, Request

from app.logging_config import configure_logging, get_logger

configure_logging()
log = get_logger()

app = FastAPI(title="analyseapp-worker")


@app.middleware("http")
async def trace_and_log(request: Request, call_next):
    trace_id = request.headers.get("x-trace-id", str(uuid.uuid4()))
    structlog.contextvars.bind_contextvars(trace_id=trace_id)

    start = time.perf_counter()
    response = await call_next(request)
    duration_ms = (time.perf_counter() - start) * 1000

    response.headers["X-Trace-Id"] = trace_id
    log.info(
        "request handled",
        method=request.method,
        path=request.url.path,
        status=response.status_code,
        duration_ms=round(duration_ms, 2),
    )
    structlog.contextvars.clear_contextvars()
    return response


@app.get("/healthz")
async def healthz():
    return {"data": {"status": "ok"}, "error": None, "meta": {}}
