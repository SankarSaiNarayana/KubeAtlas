import { useDashboardContext } from "../context/DashboardContext";

function formatTime(iso: string) {
  return new Date(iso).toLocaleString();
}

export default function IncidentsPage() {
  const { incidents } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>Incidents</h2>
        <p className="muted">Alert-driven issues correlated with cluster changes</p>
      </header>

      {incidents.length === 0 ? (
        <section className="panel panel-empty">
          <p>No incidents recorded</p>
          <p className="muted">
            Wire Alertmanager to <code>POST /api/v1/incidents</code> or load demo data
          </p>
        </section>
      ) : (
        <div className="incident-grid">
          {incidents.map((inc) => (
            <article key={inc.id} className="incident-detail-card">
              <div className="incident-detail-head">
                <span className="badge open">{inc.status}</span>
                <time>{formatTime(inc.started_at)}</time>
              </div>
              <h3>{inc.title}</h3>
              {(inc.resource_kind || inc.resource_name) && (
                <p className="mono resource-line">
                  {inc.resource_kind}/{inc.resource_namespace}/{inc.resource_name}
                </p>
              )}
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
