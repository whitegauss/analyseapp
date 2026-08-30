"""Python analysis worker (PDR.md section 3/9). Numeric analysis only --
drawing happens client-side (PDR.md section 6). A gRPC front may replace
this HTTP one in later work.
"""

import time
import uuid

import structlog
from fastapi import FastAPI, Request

from app import analysis, envelope
from app.logging_config import configure_logging, get_logger
from app.schemas import AnalysisRequest

configure_logging()
log = get_logger()

app = FastAPI(title="analyseapp-worker")
envelope.register_exception_handlers(app)


@app.middleware("http")
async def trace_and_log(request: Request, call_next):
    """Middleware that adds distributed tracing support and logs request metrics.

    Extracts or generates a trace ID, binds it to the request context, measures
    request duration, and logs the completed request with status and timing.
    """
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
    """Health check endpoint for container orchestration and monitoring."""
    return envelope.ok({"status": "ok"})


@app.post("/analyze")
async def analyze(req: AnalysisRequest):
    """Execute a numeric analysis on the provided data series.

    Dispatches to the appropriate analysis function based on the request type,
    runs the analysis with the given parameters, and returns the result.
    """
    result = analysis.run(req.type, req.data, req.params)
    log.info(
        "analysis run",
        type=req.type,
        n=len(next(iter(req.data.columns.values()), [])),
    )
    return envelope.ok({"type": req.type, "result": result})
