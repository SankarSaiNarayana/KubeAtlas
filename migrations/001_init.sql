-- kube_dashboard initial schema
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE IF NOT EXISTS graph_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      TEXT NOT NULL DEFAULT 'local',
    resource_uid    TEXT,
    api_version     TEXT NOT NULL,
    kind            TEXT NOT NULL,
    namespace       TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL,
    labels          JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'unknown',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, api_version, kind, namespace, name)
);

CREATE INDEX IF NOT EXISTS idx_graph_nodes_cluster ON graph_nodes (cluster_id);
CREATE INDEX IF NOT EXISTS idx_graph_nodes_kind_ns ON graph_nodes (kind, namespace);

CREATE TABLE IF NOT EXISTS graph_edges (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      TEXT NOT NULL DEFAULT 'local',
    source_id       UUID NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
    target_id       UUID NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
    edge_type       TEXT NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, source_id, target_id, edge_type)
);

CREATE INDEX IF NOT EXISTS idx_graph_edges_source ON graph_edges (source_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_target ON graph_edges (target_id);

CREATE TABLE IF NOT EXISTS change_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      TEXT NOT NULL DEFAULT 'local',
    resource_uid    TEXT,
    api_version     TEXT NOT NULL,
    kind            TEXT NOT NULL,
    namespace       TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL,
    verb            TEXT NOT NULL,
    actor           TEXT NOT NULL DEFAULT 'unknown',
    source          TEXT NOT NULL DEFAULT 'unknown',
    diff_summary    TEXT NOT NULL DEFAULT '',
    payload         JSONB NOT NULL DEFAULT '{}',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_change_events_resource ON change_events (cluster_id, kind, namespace, name);
CREATE INDEX IF NOT EXISTS idx_change_events_occurred ON change_events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_change_events_actor ON change_events (actor);

CREATE TABLE IF NOT EXISTS incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      TEXT NOT NULL DEFAULT 'local',
    title           TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'open',
    resource_kind   TEXT,
    resource_ns     TEXT,
    resource_name   TEXT,
    alert_labels    JSONB NOT NULL DEFAULT '{}',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents (status);
