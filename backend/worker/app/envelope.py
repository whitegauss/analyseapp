"""Unified {data, error, meta} response envelope (PDR.md section 8),
mirroring the Go API's internal/response package.
"""

from typing import Any

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from app.analysis import AnalysisError


def ok(data: Any) -> dict:
    """Create a successful response envelope with the given data payload."""
    return {"data": data, "error": None, "meta": {}}


def error_body(code: str, message: str) -> dict:
    """Create an error response envelope with the given error code and message."""
    return {"data": None, "error": {"code": code, "message": message}, "meta": {}}


def register_exception_handlers(app: FastAPI) -> None:
    """Register FastAPI exception handlers that convert errors to envelope format.

    Catches AnalysisError and RequestValidationError exceptions and returns them
    as structured error envelopes with appropriate HTTP status codes.
    """
    @app.exception_handler(AnalysisError)
    async def handle_analysis_error(request: Request, exc: AnalysisError):
        """Handle analysis-specific errors with a 400 Bad Request response."""
        return JSONResponse(status_code=400, content=error_body(exc.code, str(exc)))

    @app.exception_handler(RequestValidationError)
    async def handle_validation_error(request: Request, exc: RequestValidationError):
        """Handle Pydantic validation errors with a 400 Bad Request response."""
        return JSONResponse(
            status_code=400,
            content=error_body("invalid_request", str(exc)),
        )
