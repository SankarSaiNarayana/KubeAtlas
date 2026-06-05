from fastapi.testclient import TestClient

from app.main import app
from app.schemas import (
    AIInvestigationRef,
    IncidentContextPayload,
    IncidentPayload,
    InvestigateRequest,
    RemediateRequest,
    ResourcePayload,
)

client = TestClient(app)


def test_health():
    r = client.get("/health")
    assert r.status_code == 200
    assert r.json()["status"] == "ok"


def test_investigate_crashloop():
    req = InvestigateRequest(
        incident=IncidentPayload(id="i1", reason="CrashLoopBackOff", severity="critical"),
        resource=ResourcePayload(kind="Pod", namespace="default", name="bad"),
        context=IncidentContextPayload(
            restart_count=12,
            events=[{"reason": "BackOff", "message": "restarting failed container"}],
        ),
    )
    r = client.post("/v1/investigate", json=req.model_dump())
    assert r.status_code == 200
    body = r.json()
    assert "crash" in body["root_cause"].lower() or "back" in body["root_cause"].lower()
    assert 0 <= body["confidence_score"] <= 1


def test_remediate_pod():
    inv = AIInvestigationRef(root_cause="Container crash loop")
    req = RemediateRequest(
        incident=IncidentPayload(id="i1", reason="CrashLoopBackOff"),
        resource=ResourcePayload(kind="Pod", namespace="default", name="bad"),
        investigation=inv,
    )
    r = client.post("/v1/remediate", json=req.model_dump())
    assert r.status_code == 200
    assert len(r.json()["recommendations"]) >= 1
