import { useState } from "react";
import { useDashboardContext } from "../context/DashboardContext";

export default function ConnectionBanner() {
  const { connected, error, loading, isEmpty, loadDemo, seeding, refresh, userEmail, setUserEmail } =
    useDashboardContext();
  const [draftEmail, setDraftEmail] = useState(userEmail);

  if (loading) {
    return (
      <div className="banner banner-loading">
        <span className="pulse" />
        Connecting to API…
      </div>
    );
  }

  if (!connected) {
    if (!userEmail) {
      return (
        <div className="banner banner-warn">
          <div>
            <strong>Sign in with your email</strong>
            <p>
              Enter the email address that is allowed to access the dashboard API.
            </p>
            {error ? <p className="muted">{error}</p> : null}
          </div>
          <div className="login-inline">
            <input
              type="email"
              placeholder="you@example.com"
              value={draftEmail}
              onChange={(event) => setDraftEmail(event.target.value)}
            />
            <button
              type="button"
              className="btn btn-primary"
              disabled={!draftEmail.trim()}
              onClick={() => setUserEmail(draftEmail.trim())}
            >
              Sign in
            </button>
          </div>
        </div>
      );
    }

    return (
      <div className="banner banner-error">
        <div>
          <strong>API not connected</strong>
          <p>{error}</p>
          <ol className="banner-steps">
            <li>
              <code>docker compose -f deploy/docker-compose.yml up -d postgres</code>
            </li>
            <li>
              <code>go run ./cmd/api</code>
            </li>
            <li>Refresh this page</li>
          </ol>
        </div>
        <button type="button" className="btn btn-primary" onClick={() => refresh()}>
          Retry
        </button>
      </div>
    );
  }

  if (isEmpty) {
    return (
      <div className="banner banner-warn">
        <div>
          <strong>Connected — no data yet</strong>
          <p>
            Load sample graph, changes, and an incident to explore the dashboard, or run{" "}
            <code>go run ./cmd/graph</code> against your cluster.
          </p>
        </div>
        <button
          type="button"
          className="btn btn-primary"
          disabled={seeding}
          onClick={() => loadDemo()}
        >
          {seeding ? "Loading…" : "Load demo data"}
        </button>
      </div>
    );
  }

  return null;
}
