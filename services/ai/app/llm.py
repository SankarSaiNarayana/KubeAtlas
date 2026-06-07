"""Optional LangChain-backed investigation and remediation."""

from __future__ import annotations

import json
import os
import sys
import traceback
from typing import Any

from .schemas import (
    IncidentContextPayload,
    IncidentPayload,
    InvestigateRequest,
    InvestigationResponse,
    RemediateRequest,
    RemediateResponse,
    RemediationItem,
    ResourcePayload,
)


def _provider() -> str:
    return "groq"


def _build_llm():
    groq_api_key = os.getenv("GROQ_API_KEY")

    if not groq_api_key:
        print("ERROR: GROQ_API_KEY environment variable not set", file=sys.stderr)
        raise ValueError("GROQ_API_KEY is required")

    from langchain_groq import ChatGroq

    return ChatGroq(
        model="qwen/qwen3-32b",
        api_key=groq_api_key,
        temperature=0.1,
    ), "groq"


def _compact_context(ctx: IncidentContextPayload) -> dict[str, Any]:
    return {
        "restart_count": ctx.restart_count,
        "events": ctx.events[:20],
        "logs": ctx.logs[-30:],
        "describe": ctx.describe_data,
        "images": ctx.image_details,
        "deployment_yaml_snippet": (ctx.deployment_yaml or "")[:4000],
        "node_info": ctx.node_info,
    }


INVESTIGATE_PROMPT = """You are a principal Kubernetes SRE investigating a cluster incident.
Analyze the incident, resource, and collected context. Respond ONLY with valid JSON matching:
{{
  "summary": "one paragraph",
  "root_cause": "specific technical root cause",
  "confidence_score": 0.0-1.0,
  "impact_assessment": "blast radius and user impact",
  "evidence": [{{"source": "event|log|health", "detail": "..."}}],
  "recommended_fix": "actionable fix steps(make sure to give the exact or relevant commandto fix the issue)"
}}

Incident: {incident}
Resource: {resource}
Context: {context}
"""

REMEDIATE_PROMPT = """You are a Kubernetes remediation advisor.
Given investigation results, suggest exactly ONE best remediation action as JSON:
{{
  "recommendations": [
    {{
      "action_type": "restart_pod|restart_deployment|rollback_deployment|scale_deployment|delete_failed_pod",
      "reason": "short why this is the best fix",
      "confidence_score": 0.0-1.0,
      "risk_score": 0.0-1.0,
      "expected_outcome": "what should happen",
      "parameters": {{
        "namespace": "...",
        "name": "...",
        "kind": "...",
        "uid": "...",
        "kubectl_command": "exact kubectl command the operator should run"
      }}
    }}
  ]
}}

Return only one item in recommendations. Include kubectl_command in parameters.
Allowed actions only: restart_pod, restart_deployment, rollback_deployment, scale_deployment, delete_failed_pod.
Never suggest destructive cluster-wide changes.

Incident: {incident}
Resource: {resource}
Investigation: {investigation}
"""


def investigate_llm(req: InvestigateRequest) -> InvestigationResponse | None:
    try:
        llm, provider = _build_llm()
    except Exception as e:
        print(f"ERROR: Failed to build LLM: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        return None
    
    from langchain_core.messages import HumanMessage

    prompt = INVESTIGATE_PROMPT.format(
        incident=req.incident.model_dump(),
        resource=req.resource.model_dump(),
        context=_compact_context(req.context),
    )
    try:
        resp = llm.invoke([HumanMessage(content=prompt)])
        text = resp.content if isinstance(resp.content, str) else str(resp.content)
        start = text.find("{")
        end = text.rfind("}") + 1
        if start < 0 or end <= start:
            print("ERROR: Failed to parse JSON response from LLM", file=sys.stderr)
            print(f"RESPONSE:\n{text}", file=sys.stderr)
            return None
        data = json.loads(text[start:end])
        return InvestigationResponse(
            summary=data["summary"],
            root_cause=data["root_cause"],
            confidence_score=float(data["confidence_score"]),
            impact_assessment=data["impact_assessment"],
            evidence=data.get("evidence", []),
            recommended_fix=data["recommended_fix"],
            model_version=f"kubeatlas-langchain-{provider}",
        )
    except Exception as e:
        print(f"ERROR: LLM investigation failed: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        return None


def remediate_llm(req: RemediateRequest) -> list[RemediationItem] | None:
    try:
        llm, provider = _build_llm()
    except Exception as e:
        print(f"ERROR: Failed to build LLM: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        return None
    
    from langchain_core.messages import HumanMessage

    prompt = REMEDIATE_PROMPT.format(
        incident=req.incident.model_dump(),
        resource=req.resource.model_dump(),
        investigation=req.investigation.model_dump(),
    )
    try:
        resp = llm.invoke([HumanMessage(content=prompt)])
        text = resp.content if isinstance(resp.content, str) else str(resp.content)
        start = text.find("{")
        end = text.rfind("}") + 1
        if start < 0 or end <= start:
            print("ERROR: Failed to parse JSON response from LLM", file=sys.stderr)
            print(f"RESPONSE:\n{text}", file=sys.stderr)
            return None
        data = json.loads(text[start:end])
        items = []
        for raw in data.get("recommendations", []):
            items.append(RemediationItem.model_validate(raw))
        if not items:
            return None
        best = max(items, key=lambda i: i.confidence_score)
        return [best]
    except Exception as e:
        print(f"ERROR: LLM remediation failed: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        return None


def investigate(req: InvestigateRequest) -> InvestigationResponse | None:
    return investigate_llm(req)


def remediate(req: RemediateRequest) -> RemediateResponse | None:
    llm_items = remediate_llm(req)
    if llm_items is not None:
        return RemediateResponse(recommendations=llm_items)
    return None
