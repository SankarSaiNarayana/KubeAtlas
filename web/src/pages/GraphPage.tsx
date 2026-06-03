import GraphCanvas from "../components/GraphCanvas";
import { useDashboardContext } from "../context/DashboardContext";

export default function GraphPage() {
  const { graph, connected } = useDashboardContext();

  return (
    <div className="page">
      <header className="page-header">
        <h2>Operational graph</h2>
        <p className="muted">
          {graph.nodes.length} resources · {graph.edges.length} dependency edges
        </p>
      </header>

      {connected && (
        <section className="panel panel-graph panel-full">
          <GraphCanvas nodes={graph.nodes} edges={graph.edges} />
        </section>
      )}

      <section className="panel">
        <h3 className="panel-title">Resources</h3>
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
              {graph.nodes.map((n) => (
                <tr key={n.id}>
                  <td>
                    <span className="kind-tag">{n.kind}</span>
                  </td>
                  <td>{n.namespace || "—"}</td>
                  <td className="mono">{n.name}</td>
                  <td>
                    <span className={`badge ${n.status}`}>{n.status}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
