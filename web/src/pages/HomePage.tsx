import { Link } from "react-router-dom";
import ChangeTimeline from "../components/ChangeTimeline";
import StatCard from "../components/StatCard";
import { useDashboardContext } from "../context/DashboardContext";

export default function HomePage() {
  const { status, graph, changes, incidents, connected } = useDashboardContext();
  const stats = status?.stats;
  const failingResources = graph.nodes.filter(
    (n) => n.status !== "ready" && n.status !== "active",
  );

  return (
    <div className="page">
      <header className="page-header">
        <div>
          <h2>Operations overview</h2>
          <p className="muted">
            Recent changes and failing resources, without the dependency graph.
          </p>
        </div>
      </header>

      {connected && stats && (
        <div className="stat-grid">
          <StatCard
            label="Changes (24h)"
            value={stats.changes_24h}
            accent="amber"
            hint="who changed what"
          />
          <StatCard
            label="Open incidents"
            value={stats.open_incidents}
            accent={stats.open_incidents > 0 ? "red" : "green"}
          />
        </div>
      )}

      <div className="dashboard-grid">
        <section className="panel">
          <div className="panel-head">
            <h3>Failing resources</h3>
            <Link to="/incidents" className="link-sm">
              View incidents →
            </Link>
          </div>
          {failingResources.length === 0 ? (
            <p className="muted empty-inline">No failing resources detected</p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Kind</th>
                    <th>Namespace</th>
                    <th>Name</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {failingResources.map((resource) => (
                    <tr key={resource.id}>
                      <td>
                        <span className="kind-tag">{resource.kind}</span>
                      </td>
                      <td>{resource.namespace || "—"}</td>
                      <td className="mono">{resource.name}</td>
                      <td>
                        <span className={`badge ${resource.status}`}>
                          {resource.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="panel">
          <div className="panel-head">
            <h3>Recent changes</h3>
            <Link to="/changes" className="link-sm">
              Timeline →
            </Link>
          </div>
          <ChangeTimeline changes={changes.slice(0, 5)} compact />
        </section>

        <section className="panel">
          <div className="panel-head">
            <h3>Active incidents</h3>
            <Link to="/incidents" className="link-sm">
              All →
            </Link>
          </div>
          {incidents.length === 0 ? (
            <p className="muted empty-inline">No open incidents</p>
          ) : (
            <ul className="incident-list">
              {incidents.slice(0, 3).map((inc) => (
                <li key={inc.id} className="incident-card">
                  <span className="incident-pulse" />
                  <div>
                    <strong>{inc.title}</strong>
                    <span className="muted">
                      {inc.resource_kind}/{inc.resource_namespace}/{inc.resource_name}
                    </span>
                  </div>
                  <span className="badge open">{inc.status}</span>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}
