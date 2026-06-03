package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kube-dashboard/kube_dashboard/internal/ingest"
	"github.com/kube-dashboard/kube_dashboard/internal/models"
)

func (h *Handler) GitOpsWebhook(w http.ResponseWriter, r *http.Request) {
	var payload ingest.GitOpsWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if payload.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if payload.Kind == "" {
		payload.Kind = "Application"
	}
	if payload.SyncedBy == "" {
		payload.SyncedBy = "gitops"
	}

	change := ingest.GitOpsToChange(payload, h.config.ClusterID)
	event, err := h.store.InsertChangeEvent(r.Context(), change)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

// GenericChangeWebhook accepts simplified change payloads from Robusta, scripts, or CI.
func (h *Handler) GenericChangeWebhook(w http.ResponseWriter, r *http.Request, defaultSource string) {
	var in models.ChangeEventInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if in.Kind == "" || in.Name == "" {
		writeError(w, http.StatusBadRequest, "kind and name are required")
		return
	}
	if in.ClusterID == "" {
		in.ClusterID = h.config.ClusterID
	}
	if in.Source == "" {
		in.Source = defaultSource
	}
	if in.Verb == "" {
		in.Verb = "update"
	}
	if in.OccurredAt == nil {
		now := time.Now().UTC()
		in.OccurredAt = &now
	}

	event, err := h.store.InsertChangeEvent(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, event)
}
