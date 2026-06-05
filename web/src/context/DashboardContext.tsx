import { createContext, useCallback, useContext, useMemo, type ReactNode } from "react";
import { useDashboard } from "../hooks/useDashboard";
import { useKubeAtlas } from "../hooks/useKubeAtlas";

type DashboardContextValue = ReturnType<typeof useDashboard> &
  ReturnType<typeof useKubeAtlas>;

const DashboardContext = createContext<DashboardContextValue | null>(null);

export function DashboardProvider({ children }: { children: ReactNode }) {
  const legacy = useDashboard();
  const atlas = useKubeAtlas();

  const refresh = useCallback(async () => {
    await Promise.all([legacy.refresh(), atlas.refresh()]);
  }, [legacy.refresh, atlas.refresh]);

  const value = useMemo(
    () => ({
      ...legacy,
      ...atlas,
      connected: atlas.connected || legacy.connected,
      loading: legacy.loading || atlas.loading,
      error: atlas.error ?? legacy.error,
      refresh,
      isEmpty:
        (atlas.connected || legacy.connected) &&
        (atlas.overview?.total_resources ?? 0) === 0 &&
        (legacy.status?.stats.graph_nodes ?? 0) === 0,
    }),
    [legacy, atlas, refresh],
  );

  return (
    <DashboardContext.Provider value={value}>{children}</DashboardContext.Provider>
  );
}

export function useDashboardContext() {
  const ctx = useContext(DashboardContext);
  if (!ctx) {
    throw new Error("useDashboardContext must be used within DashboardProvider");
  }
  return ctx;
}
