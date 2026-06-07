import IncidentWorkflowCard from "../components/IncidentWorkflowCard";
import { useDashboardContext } from "../context/DashboardContext";

export default function AtlasIncidentsPage() {
  const { workflows, loading, refresh } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>Incidents</h2>
        <p className="muted">
          Unhealthy resources appear here. Run investigation for AI analysis, then verify when done.
        </p>
      </header>

      {loading ? (
        <p className="muted">Loading…</p>
      ) : workflows.length === 0 ? (
        <p className="muted">No active incidents</p>
      ) : (
        <div className="workflow-list">
          {workflows.map((wf) => (
            <IncidentWorkflowCard key={wf.incident.id} workflow={wf} onUpdate={refresh} />
          ))}
        </div>
      )}
    </div>
  );
}
