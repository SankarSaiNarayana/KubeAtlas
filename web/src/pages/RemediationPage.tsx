import { useState } from "react";
import {
  approveRemediation,
  executeRemediation,
  rejectRemediation,
} from "../api/atlas";
import { useDashboardContext } from "../context/DashboardContext";

export default function RemediationPage() {
  const { remediations, refresh } = useDashboardContext();
  const [busy, setBusy] = useState<string | null>(null);

  async function act(id: string, action: "approve" | "reject" | "execute") {
    setBusy(id);
    try {
      if (action === "approve") await approveRemediation(id);
      else if (action === "reject") await rejectRemediation(id);
      else await executeRemediation(id);
      await refresh();
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="page">
      <header className="page-header">
        <h2>Remediation</h2>
        <p className="muted">
          AI recommendations require human approval before execution
        </p>
      </header>
      {remediations.length === 0 ? (
        <p className="muted">No pending remediations</p>
      ) : (
        <div className="card-grid">
          {remediations.map((r) => (
            <article key={r.id} className="panel">
              <h3>{r.action_type}</h3>
              <p>{r.reason}</p>
              <p className="muted">
                Confidence {(r.confidence_score * 100).toFixed(0)}% · Risk{" "}
                {(r.risk_score * 100).toFixed(0)}%
              </p>
              <p className="muted">{r.expected_outcome}</p>
              <div className="btn-row">
                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={busy === r.id}
                  onClick={() => act(r.id, "approve")}
                >
                  Approve
                </button>
                <button
                  type="button"
                  className="btn btn-ghost"
                  disabled={busy === r.id}
                  onClick={() => act(r.id, "reject")}
                >
                  Reject
                </button>
                <button
                  type="button"
                  className="btn btn-ghost"
                  disabled={busy === r.id || (r.status !== "approved" && r.status !== "pending")}
                  title={
                    r.status === "pending"
                      ? "Approve first, then execute"
                      : "Run approved action on cluster"
                  }
                  onClick={async () => {
                    if (r.status === "pending") {
                      await act(r.id, "approve");
                      await refresh();
                    }
                    if (r.status === "approved" || r.status === "pending") {
                      await act(r.id, "execute");
                    }
                  }}
                >
                  {r.status === "approved" ? "Execute" : "Approve & execute"}
                </button>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
