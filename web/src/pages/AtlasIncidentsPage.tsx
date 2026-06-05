import { Link } from "react-router-dom";
import { useDashboardContext } from "../context/DashboardContext";

export default function AtlasIncidentsPage() {
  const { atlasIncidents } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>Incidents</h2>
        <p className="muted">Auto-created when health degrades from HEALTHY</p>
      </header>
      {atlasIncidents.length === 0 ? (
        <p className="muted">No open incidents</p>
      ) : (
        <ul className="incident-list">
          {atlasIncidents.map((inc) => (
            <li key={inc.id} className="incident-card">
              <span className={`incident-pulse ${inc.severity}`} />
              <div>
                <strong>{inc.title}</strong>
                <span className="muted">{inc.reason}</span>
              </div>
              <span className={`badge ${inc.severity}`}>{inc.severity}</span>
              <span className="badge open">{inc.status}</span>
              <Link to="/investigations" className="link-sm">
                Investigate →
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
