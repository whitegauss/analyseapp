"""Structured JSON logging setup, mirroring the Go API's zerolog output."""

import logging
import sys

import structlog


def configure_logging() -> None:
    """Configure structured JSON logging for the worker application.

    Sets up stdlib logging to output to stdout and configures structlog with
    JSON rendering, timestamp and log level processors, and context variable merging.
    """
    logging.basicConfig(format="%(message)s", stream=sys.stdout, level=logging.INFO)

    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.add_log_level,
            structlog.processors.JSONRenderer(),
        ],
        logger_factory=structlog.stdlib.LoggerFactory(),
        cache_logger_on_first_use=True,
    )


def get_logger(name: str = "worker") -> structlog.stdlib.BoundLogger:
    """Get a configured structlog logger instance with the given name."""
    return structlog.get_logger(name)
