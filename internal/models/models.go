package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ResourceRef struct {
	ClusterID  string `json:"cluster_id"`
	APIVersion string `json:"api_version"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	UID        string `json:"uid,omitempty"`
}

func (r ResourceRef) ID() string {
	if r.UID != "" {
		return r.UID
	}
	return r.ClusterID + "/" + r.APIVersion + "/" + r.Kind + "/" + r.Namespace + "/" + r.Name
}

type GraphNode struct {
	ID          uuid.UUID         `json:"id"`
	ClusterID   string            `json:"cluster_id"`
	ResourceUID string            `json:"resource_uid,omitempty"`
	APIVersion  string            `json:"api_version"`
	Kind        string            `json:"kind"`
	Namespace   string            `json:"namespace"`
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Status      string            `json:"status"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type GraphEdge struct {
	ID        uuid.UUID      `json:"id"`
	ClusterID string         `json:"cluster_id"`
	SourceID  uuid.UUID      `json:"source_id"`
	TargetID  uuid.UUID      `json:"target_id"`
	EdgeType  string         `json:"edge_type"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type ChangeEvent struct {
	ID          uuid.UUID      `json:"id"`
	ClusterID   string         `json:"cluster_id"`
	ResourceRef ResourceRef    `json:"resource"`
	Verb        string         `json:"verb"`
	Actor       string         `json:"actor"`
	Source      string         `json:"source"`
	DiffSummary string         `json:"diff_summary"`
	Payload     map[string]any `json:"payload,omitempty"`
	OccurredAt  time.Time      `json:"occurred_at"`
}

type ChangeEventInput struct {
	ClusterID   string         `json:"cluster_id"`
	APIVersion  string         `json:"api_version"`
	Kind        string         `json:"kind"`
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	ResourceUID string         `json:"resource_uid,omitempty"`
	Verb        string         `json:"verb"`
	Actor       string         `json:"actor"`
	Source      string         `json:"source"`
	DiffSummary string         `json:"diff_summary"`
	Payload     map[string]any `json:"payload,omitempty"`
	OccurredAt  *time.Time     `json:"occurred_at,omitempty"`
}

type GraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type Incident struct {
	ID           uuid.UUID      `json:"id"`
	ClusterID    string         `json:"cluster_id"`
	Title        string         `json:"title"`
	Status       string         `json:"status"`
	ResourceKind string         `json:"resource_kind,omitempty"`
	ResourceNS   string         `json:"resource_namespace,omitempty"`
	ResourceName string         `json:"resource_name,omitempty"`
	AlertLabels  map[string]any `json:"alert_labels,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
}

type AlertmanagerWebhook struct {
	Status string `json:"status"`
	Alerts []struct {
		Status       string            `json:"status"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		StartsAt     time.Time         `json:"startsAt"`
		GeneratorURL string            `json:"generatorURL"`
	} `json:"alerts"`
}

func LabelsToMap(labels map[string]string) map[string]any {
	out := make(map[string]any, len(labels))
	for k, v := range labels {
		out[k] = v
	}
	return out
}

func RawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
