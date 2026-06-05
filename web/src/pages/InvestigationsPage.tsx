import { useDashboardContext } from "../context/DashboardContext";

export default function InvestigationsPage() {
  const { investigations } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>AI Investigations</h2>
        <p className="muted">Evidence-based root cause analysis from incident context</p>
      </header>
      {investigations.length === 0 ? (
        <p className="muted">No investigations yet — open an incident in the cluster</p>
      ) : (
        <div className="card-grid">
          {investigations.map(({ incident, investigation }) => (
            <article key={incident.id} className="panel investigation-card">
              <h3>{incident.title}</h3>
              {!investigation ? (
                <p className="muted">Investigation pending…</p>
              ) : (
                <>
                  <p>{investigation.summary}</p>
                  <dl className="investigation-meta">
                    <dt>Root cause</dt>
                    <dd>{investigation.root_cause}</dd>
                    <dt>Confidence</dt>
                    <dd>{(investigation.confidence_score * 100).toFixed(0)}%</dd>
                    <dt>Impact</dt>
                    <dd>{investigation.impact_assessment}</dd>
                    <dt>Recommended fix</dt>
                    <dd>{investigation.recommended_fix}</dd>
                  </dl>
                </>
              )}
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
