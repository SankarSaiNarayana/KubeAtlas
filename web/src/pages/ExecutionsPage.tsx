import { useDashboardContext } from "../context/DashboardContext";

export default function ExecutionsPage() {
  const { executions } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>Execution history</h2>
        <p className="muted">Self-healing actions after human approval</p>
      </header>
      {executions.length === 0 ? (
        <p className="muted">No executions yet</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Action</th>
                <th>Result</th>
                <th>Approved by</th>
                <th>Duration</th>
                <th>Started</th>
              </tr>
            </thead>
            <tbody>
              {executions.map((e) => (
                <tr key={e.id}>
                  <td className="mono">{e.action_type}</td>
                  <td>
                    <span className={`badge ${e.success ? "ready" : "failed"}`}>
                      {e.success ? "success" : "failed"}
                    </span>
                  </td>
                  <td>{e.approved_by}</td>
                  <td>{e.duration_ms != null ? `${e.duration_ms}ms` : "—"}</td>
                  <td className="muted">{new Date(e.started_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
