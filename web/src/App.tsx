import { NavLink, Route, Routes } from "react-router-dom";
import ConnectionBanner from "./components/ConnectionBanner";
import ErrorBoundary from "./components/ErrorBoundary";
import { DashboardProvider, useDashboardContext } from "./context/DashboardContext";
import ChangesPage from "./pages/ChangesPage";
import HomePage from "./pages/HomePage";
import IncidentsPage from "./pages/IncidentsPage";

function Sidebar() {
  const { connected, status, refresh, loading } = useDashboardContext();
  const cluster = status?.cluster_id ?? "—";

  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-icon">◇</div>
        <div>
          <h1>Kube Dashboard</h1>
          <p className="tagline">Operational intelligence</p>
        </div>
      </div>

      <div className={`conn-pill ${connected ? "conn-ok" : "conn-off"}`}>
        <span className="conn-dot" />
        {connected ? `Connected · ${cluster}` : "Disconnected"}
      </div>

      <nav>
        <NavLink to="/" end>
          <span className="nav-icon">▣</span> Overview
        </NavLink>
        <NavLink to="/changes">
          <span className="nav-icon">↻</span> Changes
        </NavLink>
        <NavLink to="/incidents">
          <span className="nav-icon">!</span> Incidents
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
            <Route path="/" element={<HomePage />} />
            <Route path="/changes" element={<ChangesPage />} />
            <Route path="/incidents" element={<IncidentsPage />} />
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
