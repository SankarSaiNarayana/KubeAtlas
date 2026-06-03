import ChangeTimeline from "../components/ChangeTimeline";
import { useDashboardContext } from "../context/DashboardContext";

export default function ChangesPage() {
  const { changes } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>Change timeline</h2>
        <p className="muted">
          {changes.length} changes in the last 24 hours — audit, GitOps, kubectl
        </p>
      </header>

      <section className="panel">
        <ChangeTimeline changes={changes} />
      </section>
    </div>
  );
}
