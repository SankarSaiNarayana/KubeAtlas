import { useDashboardContext } from "../context/DashboardContext";

export default function ConnectionBanner() {
  const { connected, error, loading, isEmpty, refresh, overview } = useDashboardContext();

  if (loading) {
    return (
      <div className="banner banner-loading">
        <span className="pulse" />
        Connecting to API…
      </div>
    );
  }

  if (!connected) {
    return (
      <div className="banner banner-error">
        <div>
          <strong>API not connected</strong>
          <p>{error ?? "Cannot reach the API"}</p>
          <ol className="banner-steps">
            <li>
              <code>docker compose -f deploy/docker-compose.yml up -d postgres</code>
            </li>
            <li>
              <code>go run ./cmd/api</code>
            </li>
            <li>
              <code>export AI_SERVICE_URL=http://localhost:8090 && make run-ai</code> (optional)
            </li>
            <li>
              <code>go run ./cmd/worker</code> (cluster watch)
            </li>
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
          <strong>Connected — waiting for cluster data</strong>
          <p>
            Resources: {overview?.total_resources ?? 0}. Run the worker against a Kubernetes
            cluster to populate health, incidents, and AI analysis.
          </p>
        </div>
      </div>
    );
  }

  return null;
}
