package handlers

import (
	"net/http"
	"strings"

	"github.com/kube-dashboard/kube_dashboard/internal/domain"
)

// RegisterAtlas registers the atlas-related HTTP routes for the API server.
// These handlers expose cluster overview, resources, incidents, investigations,
// remediation actions, and execution records to the frontend.
func (h *Handler) RegisterAtlas(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/atlas/overview", h.AtlasOverview)
	mux.HandleFunc("/api/v1/atlas/resources", h.AtlasListResources)
	mux.HandleFunc("/api/v1/atlas/incidents", h.AtlasListIncidents)
	mux.HandleFunc("/api/v1/atlas/investigations", h.AtlasListInvestigations)
	mux.HandleFunc("/api/v1/atlas/remediations", h.AtlasListRemediations)
	mux.HandleFunc("/api/v1/atlas/actions/pending", h.AtlasPendingActions)
	mux.HandleFunc("/api/v1/atlas/executions", h.AtlasExecutions)
	mux.HandleFunc("/api/v1/atlas/", h.AtlasRouter)
	if h.eventHub != nil {
		mux.Handle("/api/v1/events/stream", h.eventHub)
	}
}

// AtlasOverview returns dashboard overview statistics for the given cluster.
// It uses store.GetOverview and responds with a JSON payload containing counts
// and summaries for the cluster.
func (h *Handler) AtlasOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	stats, err := h.store.GetOverview(r.Context(), clusterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// AtlasListResources returns a filtered list of cluster resources.
// It supports optional query parameters for kind and namespace, and returns
// up to 1000 resources from the database.
func (h *Handler) AtlasListResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	resources, err := h.store.ListResources(r.Context(), clusterID, r.URL.Query().Get("kind"), r.URL.Query().Get("namespace"), 1000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if resources == nil {
		resources = []domain.ClusterResource{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"resources": resources})
}

// AtlasListIncidents returns incident records for the cluster.
// By default it returns open incidents, but the status query parameter can select all statuses.
func (h *Handler) AtlasListIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}
	incidents, err := h.store.ListAtlasIncidents(r.Context(), clusterID, status, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if incidents == nil {
		incidents = []domain.AtlasIncident{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"incidents": incidents})
}

// AtlasListInvestigations returns all incidents along with attached investigation summaries.
// It loads all incidents for the cluster and then looks up an investigation record for each incident.
func (h *Handler) AtlasListInvestigations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	incidents, err := h.store.ListAtlasIncidents(r.Context(), clusterID, "all", 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type row struct {
		Incident      domain.AtlasIncident      `json:"incident"`
		Investigation *domain.AIInvestigation `json:"investigation,omitempty"`
	}
	var out []row
	for i := range incidents {
		inv, _ := h.store.GetInvestigation(r.Context(), incidents[i].ID)
		out = append(out, row{Incident: incidents[i], Investigation: inv})
	}
	if out == nil {
		out = []row{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"investigations": out})
}

// AtlasListRemediations fetches remediation recommendations for a cluster.
// It supports filtering by status (pending or approved) and returns the matching list.
func (h *Handler) AtlasListRemediations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	st := domain.ActionPending
	if raw := r.URL.Query().Get("status"); raw != "" {
		st = domain.ActionStatus(raw)
	}
	recs, err := h.store.ListByStatus(r.Context(), clusterID, st, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if recs == nil {
		recs = []domain.RemediationRecommendation{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"remediations": recs})
}

// AtlasPendingActions is an alias endpoint for pending remediation actions.
// It forwards the request to AtlasListRemediations to keep the API surface stable.
func (h *Handler) AtlasPendingActions(w http.ResponseWriter, r *http.Request) {
	h.AtlasListRemediations(w, r)
}

// AtlasExecutions returns execution records for remediation or investigation workflows.
// The frontend uses this data to show completed or in-progress automation actions.
func (h *Handler) AtlasExecutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	execs, err := h.store.ListExecutions(r.Context(), clusterID, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if execs == nil {
		execs = []domain.ExecutionRecord{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": execs})
}

// AtlasRouter handles nested atlas routes for individual resources, incidents, and remediations.
// It dispatches requests to the correct sub-handler based on path segments.
func (h *Handler) AtlasRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/api/v1/atlas/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		if len(parts) == 1 && parts[0] == "resources" && r.Method == http.MethodGet {
			h.AtlasListResources(w, r)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch parts[0] {
	case "resources":
		if r.Method == http.MethodGet && len(parts) == 2 {
			h.AtlasGetResource(w, r, parts[1])
			return
		}
	case "incidents":
		h.atlasIncidentRoutes(w, r, parts[1:])
	case "remediations":
		h.atlasRemediationRoutes(w, r, parts[1:])
	}
	writeError(w, http.StatusNotFound, "not found")
}

// AtlasGetResource returns resource details and current health for a specific resource id.
// It is used by the UI resource details page.
func (h *Handler) AtlasGetResource(w http.ResponseWriter, r *http.Request, id string) {
	res, err := h.store.GetResource(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	var healthOut any
	if health, err := h.store.GetHealth(r.Context(), id); err == nil {
		healthOut = health
	}
	writeJSON(w, http.StatusOK, map[string]any{"resource": res, "health": healthOut})
}

// atlasIncidentRoutes handles incident-specific nested routes for detail, context, investigation, and remediation lists.
func (h *Handler) atlasIncidentRoutes(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 1 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		inc, err := h.store.GetAtlasIncident(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		res, _ := h.store.GetResource(r.Context(), inc.ResourceID)
		inc.Resource = res
		writeJSON(w, http.StatusOK, inc)
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "context":
			if r.Method != http.MethodGet {
				break
			}
			ctx, err := h.store.GetContext(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, "context not found")
				return
			}
			writeJSON(w, http.StatusOK, ctx)
			return
		case "investigation":
			if r.Method != http.MethodGet {
				break
			}
			inv, err := h.store.GetInvestigation(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, "investigation not found")
				return
			}
			writeJSON(w, http.StatusOK, inv)
			return
		case "remediations":
			if r.Method != http.MethodGet {
				break
			}
			recs, err := h.store.ListRecommendations(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"remediations": recs})
			return
		}
	}
	writeError(w, http.StatusNotFound, "not found")
}

// atlasRemediationRoutes handles approval actions for remediation recommendations.
// It validates the action path and updates the recommendation status using store.Approve.
func (h *Handler) atlasRemediationRoutes(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id := parts[0]
	action := parts[1]
	approvedBy := r.Header.Get(h.config.EmailHeader)
	if approvedBy == "" {
		approvedBy = r.Header.Get("X-User-Email")
	}
	if approvedBy == "" {
		approvedBy = "operator"
	}
	switch action {
	case "approve":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := h.store.Approve(r.Context(), id, approvedBy); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if h.eventHub != nil {
			h.eventHub.Publish("action.approved", map[string]string{"id": id})
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
		return
	case "reject":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		_ = h.store.UpdateRecommendationStatus(r.Context(), id, domain.ActionRejected)
		writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
		return
	case "execute":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if h.executor == nil {
			writeError(w, http.StatusServiceUnavailable, "execution not available: start worker with cluster access")
			return
		}
		rec, err := h.executor.ExecuteApproved(r.Context(), id, approvedBy)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rec)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}
