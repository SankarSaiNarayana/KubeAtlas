package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kube-dashboard/kube_dashboard/internal/config"
	"github.com/kube-dashboard/kube_dashboard/internal/execution"
	"github.com/kube-dashboard/kube_dashboard/internal/models"
	"github.com/kube-dashboard/kube_dashboard/internal/realtime"
	"github.com/kube-dashboard/kube_dashboard/internal/store"
)

type Handler struct {
	store    *store.Store
	config   config.Config
	eventHub *realtime.Hub
	executor *execution.Executor
}

func New(st *store.Store, cfg config.Config) *Handler {
	return &Handler{store: st, config: cfg}
}

func NewWithAtlas(st *store.Store, cfg config.Config, hub *realtime.Hub, exec *execution.Executor) *Handler {
	return &Handler{store: st, config: cfg, eventHub: hub, executor: exec}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/v1/status", h.Status)
	// mux.HandleFunc("/api/v1/demo/seed", h.SeedDemo)
	mux.HandleFunc("/api/v1/graph", h.GetGraph)
	mux.HandleFunc("/api/v1/changes", h.ChangeRouter)
	mux.HandleFunc("/api/v1/incidents", h.IncidentRouter)
	mux.HandleFunc("/api/v1/webhooks/robusta", h.RobustaWebhook)
	mux.HandleFunc("/api/v1/webhooks/gitops", h.GitOpsWebhook)
	mux.HandleFunc("/api/v1/resources/", h.ResourceRouter)
	h.RegisterAtlas(mux)
}

func (h *Handler) ChangeRouter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListChanges(w, r)
	case http.MethodPost:
		h.CreateChange(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) IncidentRouter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListIncidents(w, r)
	case http.MethodPost:
		h.CreateIncidentFromAlert(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "ok",
		"cluster_id": h.config.ClusterID,
		"service":    "kube-dashboard-api",
	})
}

func (h *Handler) GetGraph(w http.ResponseWriter, r *http.Request) {
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	namespace := r.URL.Query().Get("namespace")
	includeDemo := strings.ToLower(r.URL.Query().Get("demo")) == "true"

	graph, err := h.store.GetGraph(r.Context(), clusterID, namespace, includeDemo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Ensure JSON arrays are never `null` (frontend expects arrays).
	if graph.Nodes == nil {
		graph.Nodes = []models.GraphNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []models.GraphEdge{}
	}
	writeJSON(w, http.StatusOK, graph)
}

func (h *Handler) ListChanges(w http.ResponseWriter, r *http.Request) {
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	includeDemo := strings.ToLower(r.URL.Query().Get("demo")) == "true"
	since := time.Now().UTC().Add(-24 * time.Hour)
	if raw := r.URL.Query().Get("since"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			since = time.Now().UTC().Add(-d)
		} else if t, err := time.Parse(time.RFC3339, raw); err == nil {
			since = t.UTC()
		}
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	events, err := h.store.ListChanges(
		r.Context(),
		clusterID,
		r.URL.Query().Get("namespace"),
		r.URL.Query().Get("kind"),
		r.URL.Query().Get("name"),
		r.URL.Query().Get("actor"),
		r.URL.Query().Get("source"),
		r.URL.Query().Get("verb"),
		since,
		limit,
		includeDemo,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []models.ChangeEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": events})
}

func (h *Handler) CreateChange(w http.ResponseWriter, r *http.Request) {
	var in models.ChangeEventInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if in.Kind == "" || in.Name == "" || in.Verb == "" {
		writeError(w, http.StatusBadRequest, "kind, name, and verb are required")
		return
	}
	if in.ClusterID == "" {
		in.ClusterID = h.config.ClusterID
	}
	if in.Source == "" {
		in.Source = "api"
	}
	if in.Actor == "" {
		in.Actor = "unknown"
	}

	event, err := h.store.InsertChangeEvent(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (h *Handler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	includeDemo := strings.ToLower(r.URL.Query().Get("demo")) == "true"
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	status := r.URL.Query().Get("status")
	incidents, err := h.store.ListIncidents(r.Context(), clusterID, status, limit, includeDemo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if incidents == nil {
		incidents = []models.Incident{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"incidents": incidents})
}

func (h *Handler) CreateIncidentFromAlert(w http.ResponseWriter, r *http.Request) {
	var webhook models.AlertmanagerWebhook
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		writeError(w, http.StatusBadRequest, "invalid alertmanager webhook payload")
		return
	}
	if len(webhook.Alerts) == 0 {
		writeError(w, http.StatusBadRequest, "no alerts in payload")
		return
	}
	alert := webhook.Alerts[0]
	title := alert.Annotations["summary"]
	if title == "" {
		title = alert.Labels["alertname"]
	}

	inc := models.Incident{
		ClusterID:    h.config.ClusterID,
		Title:        title,
		Status:       "open",
		ResourceKind: alert.Labels["kind"],
		ResourceNS:   alert.Labels["namespace"],
		ResourceName: alert.Labels["pod"],
		AlertLabels:  models.LabelsToMap(alert.Labels),
		StartedAt:    alert.StartsAt,
	}
	if inc.ResourceName == "" {
		inc.ResourceName = alert.Labels["deployment"]
	}

	created, err := h.store.CreateIncident(r.Context(), inc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) ResourceRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/api/v1/resources/")
	parts := strings.Split(path, "/")
	if path == "" || len(parts) == 0 {
		writeError(w, http.StatusNotFound, "resource endpoint not found")
		return
	}

	switch len(parts) {
	case 1:
		if r.Method == http.MethodGet {
			h.GetResource(w, r, parts[0])
			return
		}
	case 2:
		if r.Method == http.MethodGet {
			switch parts[1] {
			case "neighbors":
				h.GetResourceNeighbors(w, r, parts[0])
				return
			case "changes":
				h.GetResourceChanges(w, r, parts[0])
				return
			}
		}
	}
	writeError(w, http.StatusNotFound, "resource endpoint not found")
}

func (h *Handler) GetResource(w http.ResponseWriter, r *http.Request, id string) {
	nodeID, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resource id")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	node, err := h.store.GetGraphNode(r.Context(), clusterID, nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if node.ID == uuid.Nil {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}

	neighbors, err := h.store.GetResourceNeighbors(r.Context(), clusterID, nodeID, 1, "both")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	changes, err := h.store.ListResourceChanges(r.Context(), clusterID, nodeID, time.Now().UTC().Add(-24*time.Hour), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"resource":  node,
		"neighbors": neighbors,
		"changes":   changes,
	})
}

func (h *Handler) GetResourceNeighbors(w http.ResponseWriter, r *http.Request, id string) {
	nodeID, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resource id")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	depth := 1
	if raw := r.URL.Query().Get("depth"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			depth = n
		}
	}
	direction := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("direction")))
	if direction == "" {
		direction = "both"
	}

	neighbors, err := h.store.GetResourceNeighbors(r.Context(), clusterID, nodeID, depth, direction)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, neighbors)
}

func (h *Handler) GetResourceChanges(w http.ResponseWriter, r *http.Request, id string) {
	nodeID, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resource id")
		return
	}
	clusterID := queryOrDefault(r, "cluster_id", h.config.ClusterID)
	since := time.Now().UTC().Add(-1 * time.Hour)
	if raw := r.URL.Query().Get("since"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			since = time.Now().UTC().Add(-d)
		} else if t, err := time.Parse(time.RFC3339, raw); err == nil {
			since = t.UTC()
		}
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	changes, err := h.store.ListResourceChanges(r.Context(), clusterID, nodeID, since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if changes == nil {
		changes = []models.ChangeEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": changes})
}

func (h *Handler) RobustaWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	in := models.ChangeEventInput{
		ClusterID:   h.config.ClusterID,
		Source:      "robusta",
		Actor:       stringValue(payload, "user", "robusta"),
		Verb:        stringValue(payload, "verb", "update"),
		DiffSummary: stringValue(payload, "diff", stringValue(payload, "description", "")),
		Payload:     payload,
	}
	in.Kind = stringValue(payload, "kind", stringValue(payload, "resource_kind", ""))
	in.Namespace = stringValue(payload, "namespace", "")
	in.Name = stringValue(payload, "name", stringValue(payload, "resource_name", ""))
	in.APIVersion = stringValue(payload, "api_version", "v1")

	if in.Kind == "" || in.Name == "" {
		writeError(w, http.StatusBadRequest, "robusta payload missing kind/name")
		return
	}

	event, err := h.store.InsertChangeEvent(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryOrDefault(r *http.Request, key, fallback string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return fallback
}

func stringValue(m map[string]any, key, fallback string) string {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return strings.Trim(string(b), `"`)
	}
}
