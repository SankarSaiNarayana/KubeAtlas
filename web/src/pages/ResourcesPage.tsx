import { useMemo, useState } from "react";
import { useDashboardContext } from "../context/DashboardContext";

function healthClass(h?: string) {
  if (h === "CRITICAL") return "health-critical";
  if (h === "WARNING") return "health-warning";
  return "health-healthy";
}

export default function ResourcesPage() {
  const { resources, loading } = useDashboardContext();
  const [query, setQuery] = useState("");
  const [kindFilter, setKindFilter] = useState("all");
  const [namespaceFilter, setNamespaceFilter] = useState("all");
  const [healthFilter, setHealthFilter] = useState("all");

  const kinds = useMemo(
    () => [...new Set(resources.map((r) => r.kind))].sort(),
    [resources],
  );
  const namespaces = useMemo(
    () => [...new Set(resources.map((r) => r.namespace).filter(Boolean))].sort(),
    [resources],
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return resources.filter((r) => {
      if (healthFilter !== "all" && (r.health?.health ?? "") !== healthFilter) {
        return false;
      }
      if (kindFilter !== "all" && r.kind !== kindFilter) {
        return false;
      }
      if (namespaceFilter !== "all" && r.namespace !== namespaceFilter) {
        return false;
      }
      if (!q) {
        return true;
      }
      const haystack = [r.name, r.namespace, r.kind, r.health?.reason ?? ""]
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [resources, query, kindFilter, namespaceFilter, healthFilter]);

  return (
    <div className="page">
      <header className="page-header">
        <h2>Resources</h2>
        <p className="muted">Discovered cluster resources with live health state</p>
      </header>

      <div className="resource-toolbar">
        <input
          type="search"
          className="resource-search"
          placeholder="Search name, namespace, kind, reason…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <select
          className="resource-filter"
          value={healthFilter}
          onChange={(e) => setHealthFilter(e.target.value)}
          aria-label="Filter by health"
        >
          <option value="all">All health</option>
          <option value="HEALTHY">Healthy</option>
          <option value="WARNING">Warning</option>
          <option value="CRITICAL">Critical</option>
        </select>
        <select
          className="resource-filter"
          value={kindFilter}
          onChange={(e) => setKindFilter(e.target.value)}
          aria-label="Filter by kind"
        >
          <option value="all">All kinds</option>
          {kinds.map((k) => (
            <option key={k} value={k}>
              {k}
            </option>
          ))}
        </select>
        <select
          className="resource-filter"
          value={namespaceFilter}
          onChange={(e) => setNamespaceFilter(e.target.value)}
          aria-label="Filter by namespace"
        >
          <option value="all">All namespaces</option>
          {namespaces.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
        <span className="muted resource-count">
          {filtered.length} of {resources.length}
        </span>
      </div>

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
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={5} className="muted">
                    No resources match your search or filters
                  </td>
                </tr>
              ) : (
                filtered.map((r) => (
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
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
