import { useState } from "react";
import { runInvestigation, verifyIncident } from "../api/atlas";
import type { IncidentWorkflow, RemediationRecommendation } from "../types/atlas";

type Props = {
  workflow: IncidentWorkflow;
  onUpdate: () => Promise<void>;
};

function statusLabel(status: string) {
  switch (status) {
    case "open":
      return "Needs investigation";
    case "investigating":
      return "Analyzing";
    case "awaiting_approval":
      return "Ready";
    default:
      return status;
  }
}

function kubectlCommand(r: RemediationRecommendation): string {
  if (r.parameters?.kubectl_command) {
    return r.parameters.kubectl_command;
  }
  const ns = r.parameters?.namespace || "default";
  const name = r.parameters?.name || "";
  switch (r.action_type) {
    case "delete_failed_pod":
    case "restart_pod":
      return `kubectl delete pod ${name} -n ${ns}`;
    case "restart_deployment":
      return `kubectl rollout restart deployment/${name} -n ${ns}`;
    case "rollback_deployment":
      return `kubectl rollout undo deployment/${name} -n ${ns}`;
    case "scale_deployment":
      return `kubectl scale deployment/${name} -n ${ns} --replicas=2`;
    default:
      return `kubectl get pod ${name} -n ${ns}`;
  }
}

export default function IncidentWorkflowCard({ workflow, onUpdate }: Props) {
  const { incident } = workflow;
  const [busy, setBusy] = useState<string | null>(null);
  const [local, setLocal] = useState<IncidentWorkflow | null>(null);
  const [copied, setCopied] = useState(false);
  const data = local ?? workflow;
  const health = data.resource_health?.health ?? data.incident.health_after;
  const investigated = data.incident.status !== "open" && !!data.investigation;
  const suggestion = investigated ? data.remediations[0] : undefined;

  async function onInvestigate() {
    setBusy("investigate");
    try {
      const res = await runInvestigation(incident.id);
      setLocal(res.workflow);
      await onUpdate();
    } finally {
      setBusy(null);
    }
  }

  async function onVerify() {
    setBusy("verify");
    try {
      await verifyIncident(incident.id);
      setLocal(null);
      await onUpdate();
    } finally {
      setBusy(null);
    }
  }

  async function onCopy(cmd: string) {
    try {
      await navigator.clipboard.writeText(cmd);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* ignore */
    }
  }

  return (
    <article className="incident-row">
      <div className="incident-row-main">
        <span className={`health-pill health-${health.toLowerCase()}`}>{health}</span>
        <div className="incident-row-info">
          <strong>{data.incident.title}</strong>
          <span className="muted incident-row-reason">{data.incident.reason}</span>
        </div>
        <span className="badge step-badge">{statusLabel(data.incident.status)}</span>
        <div className="incident-row-actions">
          {!investigated && (
            <button
              type="button"
              className="btn btn-primary btn-sm"
              disabled={busy === "investigate"}
              onClick={onInvestigate}
            >
              {busy === "investigate" ? "Running…" : "Investigate"}
            </button>
          )}
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={busy === "verify"}
            onClick={onVerify}
          >
            {busy === "verify" ? "Closing…" : "Verify & close"}
          </button>
        </div>
      </div>

      {investigated && (
        <details className="incident-details">
          <summary>View recommendation</summary>
          <div className="incident-details-body">
            <p className="investigation-summary">{data.investigation!.summary}</p>
            <dl className="investigation-meta compact-meta">
              <dt>Root cause</dt>
              <dd>{data.investigation!.root_cause}</dd>
              <dt>Confidence</dt>
              <dd>{(data.investigation!.confidence_score * 100).toFixed(0)}%</dd>
            </dl>

            {suggestion ? (
              <div className="suggestion-box">
                <div className="suggestion-head">
                  <strong>{suggestion.action_type.replace(/_/g, " ")}</strong>
                  <span className="muted">
                    Confidence {(suggestion.confidence_score * 100).toFixed(0)}%
                  </span>
                </div>
                <p>{suggestion.reason}</p>
                <p className="muted">{suggestion.expected_outcome}</p>
                <div className="command-box">
                  <code className="mono">{kubectlCommand(suggestion)}</code>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => onCopy(kubectlCommand(suggestion))}
                  >
                    {copied ? "Copied" : "Copy"}
                  </button>
                </div>
              </div>
            ) : (
              <p className="muted">No automated fix suggested. Review investigation and handle manually.</p>
            )}
          </div>
        </details>
      )}
    </article>
  );
}
