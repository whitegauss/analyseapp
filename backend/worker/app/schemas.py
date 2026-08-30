"""Request/response data types for the analysis worker.

Experiment data is modeled as a dict of named columns rather than fixed
x/y/error fields, so new series (asymmetric error bars, weights, auxiliary
variables) can be added without a schema change -- individual analysis
functions simply declare which column names they expect.
"""

from typing import Any

from pydantic import BaseModel, model_validator


class DataSeries(BaseModel):
    columns: dict[str, list[float]]

    @model_validator(mode="after")
    def check_equal_length(self) -> "DataSeries":
        """Validate that all columns in the data series have the same length."""
        lengths = {name: len(values) for name, values in self.columns.items()}
        if len(set(lengths.values())) > 1:
            raise ValueError(f"columns must all have the same length, got {lengths}")
        return self


class AnalysisRequest(BaseModel):
    type: str
    data: DataSeries
    params: dict[str, Any] = {}
