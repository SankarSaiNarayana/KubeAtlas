package handlers

import (
	"net/http"
)

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	stats, err := h.store.DashboardStats(r.Context(), clusterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected":  true,
		"cluster_id": clusterID,
		"database":   "ok",
		"stats":      stats,
	})
}

// func (h *Handler) SeedDemo(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		writeError(w, http.StatusMethodNotAllowed, "POST required")
// 		return
// 	}
// 	stats, err := h.store.SeedDemo(r.Context(), h.config.ClusterID)
// 	if err != nil {
// 		writeError(w, http.StatusInternalServerError, err.Error())
// 		return
// 	}
// 	writeJSON(w, http.StatusOK, map[string]any{
// 		"message": "demo data loaded",
// 		"stats":   stats,
// 	})
// }
