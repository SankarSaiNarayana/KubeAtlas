"""KubeAtlas AI service — investigation and remediation (LangChain optional)."""

from __future__ import annotations

import os

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware

from .k8s import list_cluster_resources
from .llm import _provider, investigate, remediate
from .schemas import (
    HealthResponse,
    InvestigateRequest,
    InvestigationResponse,
    RemediateRequest,
    RemediateResponse,
)

app = FastAPI(
    title="KubeAtlas AI",
    description="Investigation and remediation for Kubernetes incidents",
    version="1.0.0",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health")
def health() -> HealthResponse:
    provider = _provider()
    return HealthResponse(
        status="ok",
        llm_provider=provider,
        langchain_available=True,
    )


@app.post("/v1/investigate", response_model=InvestigationResponse)
def post_investigate(body: InvestigateRequest) -> InvestigationResponse:
    result = investigate(body)

    if result is None:
        raise HTTPException(
            status_code=500,
            detail="Investigation failed"
        )

    return result


@app.post("/v1/remediate", response_model=RemediateResponse)
def post_remediate(body: RemediateRequest) -> RemediateResponse:
    result = remediate(body)

    if result is None:
        raise HTTPException(
            status_code=500,
            detail="Remediation failed"
        )

    return result


@app.get("/v1/cluster/resources")
def get_cluster_resources() -> dict[str, list[dict[str, object]]]:
    try:
        return list_cluster_resources()
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))


def run() -> None:
    import uvicorn

    port = int(os.getenv("AI_PORT", "8090"))
    uvicorn.run("app.main:app", host="0.0.0.0", port=port, reload=False)


if __name__ == "__main__":
    run()
