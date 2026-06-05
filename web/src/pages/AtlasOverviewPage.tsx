import { Link } from "react-router-dom";
import StatCard from "../components/StatCard";
import { useDashboardContext } from "../context/DashboardContext";

export default function AtlasOverviewPage() {
  const { overview, connected, atlasIncidents } = useDashboardContext();

  if (!connected || !overview) {
    return (
      <div className="page">
        <header className="page-header">
          <h2>KubeAtlas overview</h2>
          <p className="muted">Start API and worker to connect</p>
        </header>
      </div>
    );
  }

  return (
    <div className="page">
      <header className="page-header">
        <div>
          <h2>KubeAtlas overview</h2>
          <p className="muted">
            Real-time cluster health, incidents, and remediation pipeline
          </p>
        </div>
      </header>

      <div className="stat-grid">
        <StatCard label="Total resources" value={overview.total_resources} />
        <StatCard
          label="Healthy"
          value={overview.healthy_resources}
          accent="green"
        />
        <StatCard
          label="Warning"
          value={overview.warning_resources}
          accent="amber"
        />
        <StatCard
          label="Critical"
          value={overview.critical_resources}
          accent="red"
        />
        <StatCard label="Open incidents" value={overview.open_incidents} accent="red" />
        <StatCard
          label="Resolved"
          value={overview.resolved_incidents}
          accent="green"
        />
      </div>

      <section className="panel">
        <div className="panel-head">
          <h3>Recent incidents</h3>
          <Link to="/incidents" className="link-sm">
            All →
          </Link>
        </div>
        {atlasIncidents.length === 0 ? (
          <p className="muted empty-inline">No open incidents</p>
        ) : (
          <ul className="incident-list">
            {atlasIncidents.slice(0, 5).map((inc) => (
              <li key={inc.id} className="incident-card">
                <span className={`incident-pulse ${inc.severity}`} />
                <div>
                  <strong>{inc.title}</strong>
                  <span className="muted">{inc.reason}</span>
                </div>
                <span className="badge open">{inc.status}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
