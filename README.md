# 🚀 KubeAtlas (On-going)

### AI-Powered Kubernetes Operations & Troubleshooting Platform

Transforming Kubernetes cluster data into actionable operational intelligence through AI-driven investigation, root-cause analysis, and remediation guidance.

---

## 🌟 Overview

KubeAtlas is an AI-powered Kubernetes operations platform designed to help engineers investigate incidents, understand cluster behavior, and accelerate troubleshooting.

By combining real-time Kubernetes context with Large Language Models (LLMs), KubeAtlas converts complex infrastructure signals into human-readable insights, helping teams reduce debugging time and improve operational efficiency.

---

Incident occurs in K8s
     ↓
Go Worker watches cluster (via client-go)
     ↓ (collects incident context)
Go collects: events, logs, pod describe, deployment YAML
     ↓
Go calls: POST http://localhost:8090/v1/investigate
     ├─ incident data
     ├─ resource metadata
     └─ context (events, logs, describe data)
     ↓
Python AI service RECEIVES this data
     ↓
Python does NOT query K8s again
     ↓
Python calls Groq LLM with received context
     ↓
Groq returns investigation → Python returns JSON to Go
     ↓
Go stores in PostgreSQL + sends to UI

---

## 🎯 Mission

KubeAtlas aims to bridge the gap between Kubernetes operations and artificial intelligence by empowering engineers with intelligent investigation capabilities, automated operational insights, and scalable cloud-native troubleshooting workflows.

> Building the future of AI-assisted Kubernetes Operations.
