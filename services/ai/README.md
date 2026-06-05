# KubeAtlas AI Service (Python / FastAPI)

Investigation and remediation microservice. Designed for **LangChain** and multiple LLM providers.

## Endpoints

- `GET /health` — provider status (`rules`, `openai`, `anthropic`)
- `GET /v1/cluster/resources` — discover cluster resources using kubeconfig or in-cluster config
- `POST /v1/investigate` — root cause analysis from incident context
- `POST /v1/remediate` — ranked remediation actions with risk scores

## Run locally

```bash
cd services/ai
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8090
```

## LLM configuration

| Variable | Effect |
|----------|--------|
| `OPENAI_API_KEY` | Use `langchain-openai` (model: `OPENAI_MODEL`, default `gpt-4o-mini`) |
| `ANTHROPIC_API_KEY` | Use `langchain-anthropic` if OpenAI not set |
| `GROK_API_KEY` | Use Grok via `langchain-openai` with model `GROK_MODEL` or `grok-1.0` |
| *(none)* | Deterministic rules engine (`kubeatlas-rules-v1`) |

## Go integration

Set on the worker:

```bash
export AI_SERVICE_URL=http://localhost:8090
go run ./cmd/worker
```

Unset `AI_SERVICE_URL` to use the embedded Go rules engine instead.
