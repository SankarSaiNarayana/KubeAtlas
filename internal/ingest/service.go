package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kube-dashboard/kube_dashboard/internal/config"
	"github.com/kube-dashboard/kube_dashboard/internal/models"
	"github.com/kube-dashboard/kube_dashboard/internal/store"
)

// Service accepts change events from audit parsers and external webhooks.
type Service struct {
	cfg   config.Config
	store *store.Store
}

func NewService(cfg config.Config, st *store.Store) *Service {
	return &Service{cfg: cfg, store: st}
}

func (s *Service) Run(ctx context.Context) error {
	auditPath := os.Getenv("AUDIT_LOG_PATH")
	if auditPath == "" {
		log.Println("ingest: AUDIT_LOG_PATH not set; audit tail disabled (use POST /api/v1/changes or Robusta webhook)")
		<-ctx.Done()
		return ctx.Err()
	}

	log.Printf("ingest: tailing audit log at %s", auditPath)
	return tailAuditFile(ctx, auditPath, func(line []byte) error {
		event, err := ParseAuditLine(line)
		if err != nil {
			return nil
		}
		event.ClusterID = s.cfg.ClusterID
		_, err = s.store.InsertChangeEvent(ctx, event)
		return err
	})
}

// ParseAuditLine parses a Kubernetes audit log JSON line into a ChangeEventInput.
func ParseAuditLine(line []byte) (models.ChangeEventInput, error) {
	var entry auditEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return models.ChangeEventInput{}, err
	}
	if entry.ObjectRef == nil {
		return models.ChangeEventInput{}, fmt.Errorf("missing objectRef")
	}
	verb := entry.Verb
	if verb == "" {
		return models.ChangeEventInput{}, fmt.Errorf("missing verb")
	}
	if verb == "get" || verb == "list" || verb == "watch" {
		return models.ChangeEventInput{}, fmt.Errorf("skip read verb")
	}

	actor := "unknown"
	if entry.User.Username != "" {
		actor = entry.User.Username
	}

	occurred := time.Now().UTC()
	if !entry.RequestReceivedTimestamp.IsZero() {
		occurred = entry.RequestReceivedTimestamp.UTC()
	}

	apiVersion := entry.ObjectRef.APIVersion
	if entry.ObjectRef.APIGroup != "" {
		apiVersion = entry.ObjectRef.APIGroup + "/" + entry.ObjectRef.APIVersion
	}

	return models.ChangeEventInput{
		APIVersion:  apiVersion,
		Kind:        kindFromResource(entry.ObjectRef.Resource),
		Namespace:   entry.ObjectRef.Namespace,
		Name:        entry.ObjectRef.Name,
		Verb:        verb,
		Actor:       actor,
		Source:      "audit",
		DiffSummary: fmt.Sprintf("%s %s/%s", verb, entry.ObjectRef.Resource, entry.ObjectRef.Name),
		OccurredAt:  &occurred,
		Payload: map[string]any{
			"user_agent": entry.UserAgent,
			"source_ips": entry.SourceIPs,
		},
	}, nil
}

type auditEntry struct {
	Verb                     string    `json:"verb"`
	UserAgent                string    `json:"userAgent"`
	SourceIPs                []string  `json:"sourceIPs"`
	RequestReceivedTimestamp time.Time `json:"requestReceivedTimestamp"`
	User                     struct {
		Username string `json:"username"`
	} `json:"user"`
	ObjectRef *struct {
		APIGroup   string `json:"apiGroup"`
		APIVersion string `json:"apiVersion"`
		Resource   string `json:"resource"`
		Namespace  string `json:"namespace"`
		Name       string `json:"name"`
	} `json:"objectRef"`
}

// GitOpsWebhookPayload is a minimal Argo CD / Flux notification payload shape.
type GitOpsWebhookPayload struct {
	AppName   string `json:"app_name"`
	Namespace string `json:"namespace"`
	Revision  string `json:"revision"`
	SyncedBy  string `json:"synced_by"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

func GitOpsToChange(in GitOpsWebhookPayload, clusterID string) models.ChangeEventInput {
	now := time.Now().UTC()
	return models.ChangeEventInput{
		ClusterID:   clusterID,
		APIVersion:  "argoproj.io/v1alpha1",
		Kind:        in.Kind,
		Namespace:   in.Namespace,
		Name:        in.Name,
		Verb:        "sync",
		Actor:       in.SyncedBy,
		Source:      "gitops",
		DiffSummary: fmt.Sprintf("synced revision %s for app %s", in.Revision, in.AppName),
		OccurredAt:  &now,
	}
}

func HandleGitOpsWebhook(st *store.Store, clusterID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload GitOpsWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		change := GitOpsToChange(payload, clusterID)
		if _, err := st.InsertChangeEvent(r.Context(), change); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}
