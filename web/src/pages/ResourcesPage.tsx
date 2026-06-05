import { useDashboardContext } from "../context/DashboardContext";

function healthClass(h?: string) {
  if (h === "CRITICAL") return "health-critical";
  if (h === "WARNING") return "health-warning";
  return "health-healthy";
}

export default function ResourcesPage() {
  const { resources, loading } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>Resources</h2>
        <p className="muted">Discovered cluster resources with live health state</p>
      </header>
      {loading ? (
        <p className="muted">Loading…</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Health</th>
                <th>Kind</th>
                <th>Namespace</th>
                <th>Name</th>
                <th>Reason</th>
              </tr>
            </thead>
            <tbody>
              {resources.map((r) => (
                <tr key={r.id}>
                  <td>
                    <span className={`health-pill ${healthClass(r.health?.health)}`}>
                      {r.health?.health ?? "—"}
                    </span>
                  </td>
                  <td>
                    <span className="kind-tag">{r.kind}</span>
                  </td>
                  <td>{r.namespace || "—"}</td>
                  <td className="mono">{r.name}</td>
                  <td className="muted">{r.health?.reason ?? "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
