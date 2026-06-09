# 🚀 KubeAtlas (On-going)

### AI-Powered Kubernetes Operations & Troubleshooting Platform

Transforming Kubernetes cluster data into actionable operational intelligence through AI-driven investigation, root-cause analysis, and remediation guidance.

---

## 🌟 Overview

KubeAtlas is an AI-powered Kubernetes operations platform designed to help engineers investigate incidents, understand cluster behavior, and accelerate troubleshooting.

By combining real-time Kubernetes context with Large Language Models (LLMs), KubeAtlas converts complex infrastructure signals into human-readable insights, helping teams reduce debugging time and improve operational efficiency.

---

## KubeAtlas Investigation Flow

```text
Incident occurs in Kubernetes
        │
        ▼
Go Worker watches cluster (via client-go)
        │
        │ Collects incident context
        ▼
Go gathers:
  • Events
  • Pod Logs
  • Pod Describe Output
  • Deployment YAML
        │
        ▼
POST http://localhost:8090/v1/investigate
        │
        ├── Incident Data
        ├── Resource Metadata
        └── Context (Events, Logs, Describe Data)
        │
        ▼
Python AI Service receives investigation payload
        │
        ▼
Python does NOT query Kubernetes again
        │
        ▼
Python sends collected context to Groq LLM
        │
        ▼
Groq generates root cause analysis and remediation
        │
        ▼
Python returns structured JSON response to Go
        │
        ▼
Go stores investigation results in PostgreSQL
        │
        ▼
Results are served to the KubeAtlas UI Dashboard
```

### Key Design Principle

KubeAtlas follows a **context-first AI architecture**:

- Kubernetes data is collected only once by the Go worker.
- Python AI service remains Kubernetes-agnostic.
- AI analysis is performed using the incident context received from Go.
- Investigation results are persisted in PostgreSQL for historical tracking and trend analysis.
- The UI consumes stored investigations for visualization and remediation guidance.

### Tech Stack

| Component | Technology |
|------------|------------|
| Cluster Monitoring | Go + client-go |
| AI Investigation API | Python + FastAPI |
| LLM Provider | Groq |
| Data Storage | PostgreSQL |
| Frontend Dashboard | React |
| Container Platform | Kubernetes |
| Communication | REST API |
```
# KubeAtlas Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│ Kubernetes Cluster                                          │
└────────────────────────────┬────────────────────────────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
    ┌─────────▼────────┐  ┌──▼──────────┐   │
    │  Graph Builder   │  │  Ingester   │   │
    │ (Watches all     │  │ (Watches    │   │
    │ resources via    │  │ Events &    │   │
    │ client-go)       │  │ Audit Logs) │   │
    └────────┬─────────┘  └──┬──────────┘   │
             │               │              │
             └───────┬───────┘              │
                     │                      │
                     ▼                      │
              ┌──────────────┐              │
              │  Go Worker   │◄─────────────┘
              │ (client-go)  │
              └──────┬───────┘
                     │
                     │ Collects Incident Context
                     │ • Events
                     │ • Pod Logs
                     │ • Resource Metadata
                     │ • Deployment YAML
                     │ • Cluster Relationships
                     ▼
         ┌──────────────────────────┐
         │                          │
         │ Investigation Request    │
         │ HTTP POST                │
         │ /v1/investigate          │
         ▼                          ▼
    ┌─────────────┐          ┌──────────────────┐
    │ PostgreSQL  │          │ Python AI Service│
    │             │◄─────────│ (Groq LLM)       │
    │ Stores:     │          │                  │
    │ • Graph     │          │ No Kubernetes    │
    │ • Events    │          │ Queries During   │
    │ • Changes   │          │ Investigation    │
    │ • Incidents │          │                  │
    └──────┬──────┘          └──────────────────┘
           │
           ▼
    ┌─────────────┐
    │  Go API     │
    │   Server    │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  React UI   │
    │ Dashboard   │
    └─────────────┘
```

## Architecture Overview

KubeAtlas continuously observes the Kubernetes cluster and builds a live understanding of resource relationships, changes, and incidents.

### Core Components

#### Graph Builder
- Watches Kubernetes resources using `client-go`
- Builds a dependency graph of cluster objects
- Tracks relationships between Pods, Deployments, Services, Ingresses, ConfigMaps, Secrets, and Nodes

#### Ingester
- Consumes Kubernetes Events and Audit Logs
- Captures cluster changes in real time
- Creates a historical timeline of cluster activity

#### Go Worker
- Central orchestration component
- Collects incident-specific context
- Retrieves logs, events, manifests, and metadata
- Sends enriched investigation requests to the AI service

#### Python AI Service
- FastAPI-based investigation engine
- Receives complete incident context from Go
- Uses Groq LLM for root cause analysis and remediation generation
- Remains Kubernetes-agnostic during investigations

#### PostgreSQL
Stores:
- Resource dependency graph
- Change history
- Audit records
- Incident investigations
- AI-generated remediation results

#### Go API Server
- Serves investigation results
- Exposes REST APIs
- Retrieves graph and incident data from PostgreSQL

#### React Dashboard
Provides:
- Cluster topology visualization
- Incident timeline
- AI-powered root cause analysis
- Recommended remediation actions
- Historical investigation search

---

## Investigation Workflow

1. Incident occurs inside the Kubernetes cluster.
2. Go Worker gathers logs, events, manifests, and resource metadata.
3. Context is sent to the Python AI Service.
4. Python forwards the enriched context to Groq LLM.
5. Groq generates root cause analysis and remediation recommendations.
6. Investigation results are stored in PostgreSQL.
7. Go API serves the results.
8. React Dashboard displays findings to platform engineers.

### Key Design Principle

**Collect once, analyze anywhere.**

The Go layer is solely responsible for Kubernetes communication, while the AI layer focuses entirely on reasoning over the provided context. This separation improves scalability, reduces API calls to the cluster, and enables vendor-independent AI investigations.
```
---

## 🎯 Mission

KubeAtlas aims to bridge the gap between Kubernetes operations and artificial intelligence by empowering engineers with intelligent investigation capabilities, automated operational insights, and scalable cloud-native troubleshooting workflows.

> Building the future of AI-assisted Kubernetes Operations.
