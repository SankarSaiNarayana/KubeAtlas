# Database Workflow & Schema Documentation

## Complete Data Flow with Database Involvement

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        KUBERNETES CLUSTER (K8s)                                 │
│                                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │ Deployment   │  │ Service      │  │ Pod          │  │ Ingress      │        │
│  │ (Observed)   │  │ (Observed)   │  │ (Observed)   │  │ (Observed)   │        │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘        │
│         │                 │                 │                 │                │
│         └─────────────────┼─────────────────┼─────────────────┘                │
│                           │ K8s Informers                                       │
│                           ▼                                                     │
│                    ┌────────────────┐                                           │
│                    │  Webhook/Event │                                           │
│                    │  (add/update)  │                                           │
│                    └────────┬───────┘                                           │
└─────────────────────────────┼───────────────────────────────────────────────────┘
                              │
                              │ (cmd/worker)
                              ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         GO COLLECTOR SERVICE                                    │
│                      (internal/collector/)                                      │
│                                                                                  │
│  1. Receives: Pod/Deployment/Service events from K8s Informers                 │
│  2. Converts: K8s objects → ClusterResource structs                            │
│  3. Extracts: spec, status, labels, owner references                           │
│  4. Inserts: Into PostgreSQL database                                          │
│                                                                                  │
└────────────┬─────────────────────────────────────────────────────────────────┬──┘
             │                                                                 │
             │ INSERT/UPDATE                                    Health check &
             │                                                   Incident creation
             ▼                                                   │
┌──────────────────────────────────────────────────────────────────────────────┐
│                         POSTGRESQL DATABASE                                  │
│                    (Port 5433, user: kube, db: kubedashboard)               │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ TABLE: cluster_resources                                             │   │
│  ├──────────────────────────────────────────────────────────────────────┤   │
│  │ id (UUID)              │ Auto-generated unique ID                    │   │
│  │ cluster_id             │ "local"                                     │   │
│  │ resource_uid           │ K8s unique ID (e.g., pod-12345)            │   │
│  │ kind                   │ Pod, Deployment, Service, Ingress, etc     │   │
│  │ namespace              │ demo, default, kube-system, etc            │   │
│  │ name                   │ nginx-pod-1, api-deployment, etc           │   │
│  │ api_version            │ v1, apps/v1, networking.k8s.io/v1         │   │
│  │ labels (JSONB)         │ {"app": "nginx", "env": "prod"}           │   │
│  │ spec_snapshot (JSONB)  │ Full K8s spec object (compressed)         │   │
│  │ status_snapshot (JSONB)│ Full K8s status object (compressed)       │   │
│  │ node_name              │ Node where pod runs (if applicable)        │   │
│  │ owner_kind/name        │ Deployment/ReplicaSet that owns this     │   │
│  │ created_at/updated_at  │ Timestamps                                │   │
│  │ deleted_at             │ Soft-delete timestamp                     │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ TABLE: resource_health                                               │   │
│  ├──────────────────────────────────────────────────────────────────────┤   │
│  │ id (UUID)              │ Auto-generated unique ID                    │   │
│  │ cluster_id             │ "local"                                     │   │
│  │ resource_id (FK)       │ Links to cluster_resources.id              │   │
│  │ health                 │ ENUM: HEALTHY | WARNING | CRITICAL        │   │
│  │ reason                 │ Why: "ImagePullBackOff", "CrashLoop", etc │   │
│  │ details (JSONB)        │ {"container_status": {...}, ...}          │   │
│  │ evaluated_at           │ Timestamp of evaluation                    │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ TABLE: atlas_incidents                                               │   │
│  ├──────────────────────────────────────────────────────────────────────┤   │
│  │ id (UUID)              │ Auto-generated unique ID                    │   │
│  │ cluster_id             │ "local"                                     │   │
│  │ resource_id (FK)       │ Links to cluster_resources.id              │   │
│  │ title                  │ "Pod nginx-pod-1 is in CrashLoopBackOff"  │   │
│  │ severity               │ ENUM: warning | critical                   │   │
│  │ status                 │ ENUM: open | investigating | awaiting_     │   │
│  │                        │       approval | resolved                  │   │
│  │ reason                 │ "Resource health transitioned from HEALTHY│   │
│  │                        │  to CRITICAL"                              │   │
│  │ health_before          │ HEALTHY (previous state)                   │   │
│  │ health_after           │ CRITICAL (current state)                   │   │
│  │ opened_at/resolved_at  │ Timestamps                                │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ TABLE: incident_context                                              │   │
│  ├──────────────────────────────────────────────────────────────────────┤   │
│  │ id (UUID)              │ Auto-generated unique ID                    │   │
│  │ incident_id (FK)       │ Links to atlas_incidents.id                │   │
│  │ logs (JSONB[])         │ Last 30 lines of pod container logs       │   │
│  │ events (JSONB[])       │ Recent K8s events related to pod          │   │
│  │ describe_data (JSONB)  │ kubectl describe output (parsed)          │   │
│  │ deployment_yaml        │ Full deployment YAML (first 4KB)          │   │
│  │ replicaset_info (JSONB)│ ReplicaSet metadata that owns pod        │   │
│  │ node_info (JSONB)      │ Node details (CPU, memory, conditions)    │   │
│  │ restart_count          │ Number of pod restarts                     │   │
│  │ image_details (JSONB[])│ Container image info & pull details       │   │
│  │ env_vars (JSONB[])     │ Pod environment variables                  │   │
│  │ volume_mounts (JSONB[])│ Pod volume mount information               │   │
│  │ collected_at           │ Timestamp when context was collected      │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ TABLE: ai_investigations                                             │   │
│  ├──────────────────────────────────────────────────────────────────────┤   │
│  │ id (UUID)              │ Auto-generated unique ID                    │   │
│  │ incident_id (FK)       │ Links to atlas_incidents.id                │   │
│  │ summary                │ "Pod stuck in ImagePullBackOff for 5mins" │   │
│  │ root_cause             │ "Image registry is unavailable"           │   │
│  │ confidence_score       │ 0.92 (92% confidence)                      │   │
│  │ impact_assessment      │ "User requests failing due to unavailable│   │
│  │                        │  pod replicas"                             │   │
│  │ evidence (JSONB[])     │ [{"source": "logs", "detail": "..."},     │   │
│  │                        │  {"source": "events", "detail": "..."}]  │   │
│  │ recommended_fix        │ "Pull image from backup registry or       │   │
│  │                        │  retry with exponential backoff"          │   │
│  │ model_version          │ "groq/qwen3-32b" or "kubeatlas-rules-v1" │   │
│  │ investigated_at        │ Timestamp of investigation                 │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ TABLE: remediation_recommendations                                   │   │
│  ├──────────────────────────────────────────────────────────────────────┤   │
│  │ id (UUID)              │ Auto-generated unique ID                    │   │
│  │ incident_id (FK)       │ Links to atlas_incidents.id                │   │
│  │ investigation_id (FK)  │ Links to ai_investigations.id              │   │
│  │ action_type            │ "restart_pod" | "restart_deployment" |   │   │
│  │                        │ "rollback_deployment" | "scale_deployment"│   │
│  │ reason                 │ "Best action due to 0.92 confidence"      │   │
│  │ confidence_score       │ 0.92                                        │   │
│  │ risk_score             │ 0.15 (15% risk of causing issues)         │   │
│  │ expected_outcome       │ "Pod will restart with fresh image"       │   │
│  │ parameters (JSONB)     │ {"namespace": "demo", "pod_name":         │   │
│  │                        │  "nginx-pod-1", "kubectl_command":       │   │
│  │                        │  "kubectl restart pod ..."}               │   │
│  │ status                 │ ENUM: pending | approved | rejected |    │   │
│  │                        │       executing | succeeded | failed     │   │
│  │ created_at             │ Timestamp                                  │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ TABLE: execution_history                                             │   │
│  ├──────────────────────────────────────────────────────────────────────┤   │
│  │ id (UUID)              │ Auto-generated unique ID                    │   │
│  │ cluster_id             │ "local"                                     │   │
│  │ recommendation_id (FK) │ Links to remediation_recommendations.id   │   │
│  │ approved_by            │ User who approved the action               │   │
│  │ action_type            │ "restart_pod"                               │   │
│  │ parameters (JSONB)     │ Same as recommendation parameters          │   │
│  │ success                │ true/false                                  │   │
│  │ failure_reason         │ "Pod does not exist anymore"              │   │
│  │ rolled_back            │ false/true                                  │   │
│  │ rollback_reason        │ "Action caused service degradation"       │   │
│  │ started_at/completed_at│ Timestamps                                │   │
│  │ duration_ms            │ Time taken to execute (milliseconds)       │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
      ▲                    ▲                    ▲
      │                    │                    │
      │ Read              │ Read              │ Update
      │                    │                    │
      │                    │                    │
   ┌──┴────────────────────┴────────────────────┴──┐
   │   GO PIPELINE (cmd/worker)                   │
   │   internal/pipeline/orchestrator.go          │
   │                                              │
   │  1. Health Engine: Evaluates resource health│
   │     Compares: status_snapshot vs expected   │
   │  2. Incident Engine: Creates/updates        │
   │     incidents when health changes          │
   │  3. Context Builder: Collects incident      │
   │     context (logs, events, etc.)           │
   │  4. Calls AI Service if unhealthy           │
   │                                              │
   └──────┬────────────────────────────────┬─────┘
          │                                │
          │ HTTP POST                      │
          │ /v1/investigate                │
          │                                │
          ▼                                ▼
    ┌──────────────────────────────────────────────┐
    │   PYTHON AI SERVICE (services/ai)            │
    │   Receives from Go:                          │
    │   • incident (title, severity, status)      │
    │   • resource (kind, namespace, name, labels)│
    │   • context (logs, events, restart_count...)│
    │                                              │
    │   Processes with:                            │
    │   • LangChain + Groq LLM                    │
    │   • Generates: summary, root_cause, fixes   │
    │                                              │
    │   Returns:                                   │
    │   • InvestigationResponse                    │
    │   • RemediateResponse                        │
    │                                              │
    └──────┬───────────────────────────────────────┘
           │ HTTP Response
           │
           ▼
    ┌──────────────────────────────────────────────┐
    │   GO SAVES RESULTS TO DATABASE               │
    │   • ai_investigations table                  │
    │   • remediation_recommendations table        │
    │   • Updates atlas_incidents status           │
    │                                              │
    └──────┬───────────────────────────────────────┘
           │ SELECT & Display
           │
           ▼
    ┌──────────────────────────────────────────────┐
    │   GO API SERVER (cmd/api)                    │
    │   Serves dashboards & recommendations:      │
    │   GET /api/v1/atlas/overview                 │
    │   GET /api/v1/incidents                      │
    │   GET /api/v1/atlas/{id}/workflow            │
    │                                              │
    └──────────────────────────────────────────────┘
           │ REST API calls
           │
           ▼
    ┌──────────────────────────────────────────────┐
    │   REACT FRONTEND (web/)                      │
    │   • Displays resources                       │
    │   • Shows incidents & investigations         │
    │   • Lists remediation recommendations        │
    │   • Allows approving/executing actions       │
    │                                              │
    └──────────────────────────────────────────────┘
```

---

## Database Tables Summary

| Table | Purpose | Key Data |
|-------|---------|----------|
| `cluster_resources` | Stores all K8s resources discovered by collector | Pods, Deployments, Services, Ingresses with full spec & status |
| `resource_health` | Tracks health state transitions | HEALTHY → WARNING → CRITICAL |
| `atlas_incidents` | Records when health degrades | Incident metadata, timeline, severity |
| `incident_context` | Deep context for investigation | Logs, events, describe, YAML, restart counts |
| `ai_investigations` | LLM analysis results | Root cause, confidence, recommended fix |
| `remediation_recommendations` | Proposed remediation actions | Action type, risk score, kubectl command |
| `execution_history` | Audit trail of executed actions | Who approved, what ran, success/failure |

---

## Complete Workflow: Step by Step

### **Phase 1: Resource Discovery** (Collector)
```
1. K8s Informer detects Pod state change
2. Collector converts K8s object → ClusterResource
3. INSERT/UPDATE cluster_resources with:
   - Pod name, namespace, labels
   - Full spec & status (as JSONB)
   - Owner references (Deployment/ReplicaSet)
```

### **Phase 2: Health Evaluation** (Pipeline)
```
1. Health engine reads cluster_resources
2. Evaluates status_snapshot against rules:
   - Pod.Status.Phase == "Running"? → HEALTHY
   - Pod.Status.Phase == "Pending" + timeout? → WARNING
   - Pod restarts > 3? → CRITICAL
3. UPSERTs resource_health with new state
```

### **Phase 3: Incident Creation** (Pipeline)
```
1. Health engine detects state change:
   HEALTHY → CRITICAL
2. Creates atlas_incidents row:
   - title: "Pod nginx is now CRITICAL"
   - severity: critical
   - status: open
   - health_before: HEALTHY
   - health_after: CRITICAL
```

### **Phase 4: Context Collection** (Pipeline)
```
1. Incident opens → Context Builder triggered
2. Fetches from cluster:
   - Pod logs (last 30 lines)
   - Recent K8s events
   - Describe output
   - Deployment YAML
   - Node info (CPU, memory)
   - Image pull details
3. INSERTs incident_context table with all this data
```

### **Phase 5: AI Investigation** (Go + Python)
```
1. Go service reads from DB:
   - atlas_incidents
   - cluster_resources  
   - incident_context
2. HTTP POST to Python AI service:
   {
     "incident": {...},
     "resource": {...},
     "context": {logs, events, describe_data, ...}
   }
3. Python LLM analyzes all data
4. Returns investigation results
5. Go UPSERTs ai_investigations table:
   - summary: "Pod stuck due to image unavailable"
   - root_cause: "Registry timeout"
   - confidence_score: 0.92
```

### **Phase 6: Remediation** (Go + Python)
```
1. Go calls Python /v1/remediate with:
   - incident data
   - resource data
   - investigation results
2. Python generates recommendations:
   - action_type: "restart_pod"
   - risk_score: 0.15
   - expected_outcome: "Pod will pull image again"
3. Go INSERTs remediation_recommendations
4. Marks as status: "pending"
```

### **Phase 7: Execution** (User approved)
```
1. User clicks "Execute" in web UI
2. Go service marks recommendation as "approved"
3. Executes kubectl command from parameters
4. UPSERTs execution_history:
   - success: true/false
   - failure_reason: (if failed)
   - duration_ms: 1234
```

---

## Key Insights

✅ **Database is the single source of truth**
   - Collector → DB (resources discovered)
   - Health Engine → DB (health states)
   - Incident Engine → DB (incidents created)
   - Python AI → Receives via HTTP, Go saves to DB
   - Execution history → All actions recorded

✅ **JSONB columns store flexibility**
   - `spec_snapshot` & `status_snapshot` store full K8s objects
   - Allows drilling down into deployment specs, pod statuses
   - No schema migration needed when K8s objects change

✅ **No direct DB access from Python**
   - Python receives data via HTTP request body
   - Python returns analysis results via HTTP response
   - Go service saves all results to DB
   - Keeps Python stateless & scalable

✅ **Audit trail complete**
   - Every state change tracked in timestamps
   - execution_history shows who did what when
   - Enables rollback & investigation of past incidents

