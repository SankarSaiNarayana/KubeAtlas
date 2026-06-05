import { NavLink, Route, Routes } from "react-router-dom";
import ConnectionBanner from "./components/ConnectionBanner";
import ErrorBoundary from "./components/ErrorBoundary";
import { DashboardProvider, useDashboardContext } from "./context/DashboardContext";
import AtlasIncidentsPage from "./pages/AtlasIncidentsPage";
import AtlasOverviewPage from "./pages/AtlasOverviewPage";
import ExecutionsPage from "./pages/ExecutionsPage";
import InvestigationsPage from "./pages/InvestigationsPage";
import RemediationPage from "./pages/RemediationPage";
import ResourcesPage from "./pages/ResourcesPage";

function Sidebar() {
  const { connected, refresh, loading } = useDashboardContext();

  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-icon">◇</div>
        <div>
          <h1>KubeAtlas</h1>
          <p className="tagline">Incident intelligence</p>
        </div>
      </div>

      <div className={`conn-pill ${connected ? "conn-ok" : "conn-off"}`}>
        <span className="conn-dot" />
        {connected ? "API connected" : "Disconnected"}
      </div>

      <nav>
        <NavLink to="/" end>
          <span className="nav-icon">▣</span> Overview
        </NavLink>
        <NavLink to="/resources">
          <span className="nav-icon">◫</span> Resources
        </NavLink>
        <NavLink to="/incidents">
          <span className="nav-icon">!</span> Incidents
        </NavLink>
        <NavLink to="/investigations">
          <span className="nav-icon">⌕</span> Investigations
        </NavLink>
        <NavLink to="/remediation">
          <span className="nav-icon">⚙</span> Remediation
        </NavLink>
        <NavLink to="/executions">
          <span className="nav-icon">↯</span> Executions
        </NavLink>
      </nav>

      <div className="sidebar-footer">
        <button
          type="button"
          className="btn btn-ghost btn-block"
          disabled={loading}
          onClick={() => refresh()}
        >
          Refresh
        </button>
      </div>
    </aside>
  );
}

function AppShell() {
  return (
    <div className="app">
      <Sidebar />
      <div className="main">
        <ConnectionBanner />
        <main className="content">
          <Routes>
            <Route path="/" element={<AtlasOverviewPage />} />
            <Route path="/resources" element={<ResourcesPage />} />
            <Route path="/incidents" element={<AtlasIncidentsPage />} />
            <Route path="/investigations" element={<InvestigationsPage />} />
            <Route path="/remediation" element={<RemediationPage />} />
            <Route path="/executions" element={<ExecutionsPage />} />
          </Routes>
        </main>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <DashboardProvider>
      <ErrorBoundary>
        <AppShell />
      </ErrorBoundary>
    </DashboardProvider>
  );
}
