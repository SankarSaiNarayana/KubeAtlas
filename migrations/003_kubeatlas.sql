-- KubeAtlas: resource discovery, health, incidents, AI workflow, execution
DO $$ BEGIN
    CREATE TYPE health_state AS ENUM ('HEALTHY', 'WARNING', 'CRITICAL');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE incident_severity AS ENUM ('warning', 'critical');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE incident_status AS ENUM ('open', 'investigating', 'awaiting_approval', 'resolved');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE action_status AS ENUM ('pending', 'approved', 'rejected', 'executing', 'succeeded', 'failed', 'rolled_back');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS cluster_resources (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      TEXT NOT NULL DEFAULT 'local',
    resource_uid    TEXT NOT NULL,
    api_version     TEXT NOT NULL,
    kind            TEXT NOT NULL,
    namespace       TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL,
    labels          JSONB NOT NULL DEFAULT '{}',
    spec_snapshot   JSONB NOT NULL DEFAULT '{}',
    status_snapshot JSONB NOT NULL DEFAULT '{}',
    node_name       TEXT,
    owner_kind      TEXT,
    owner_name      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (cluster_id, resource_uid)
);

CREATE INDEX IF NOT EXISTS idx_cluster_resources_cluster ON cluster_resources (cluster_id);
CREATE INDEX IF NOT EXISTS idx_cluster_resources_kind_ns ON cluster_resources (cluster_id, kind, namespace);
CREATE INDEX IF NOT EXISTS idx_cluster_resources_uid ON cluster_resources (cluster_id, resource_uid);

CREATE TABLE IF NOT EXISTS resource_health (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      TEXT NOT NULL DEFAULT 'local',
    resource_id     UUID NOT NULL REFERENCES cluster_resources(id) ON DELETE CASCADE,
    health          health_state NOT NULL DEFAULT 'HEALTHY',
    reason          TEXT NOT NULL DEFAULT '',
    details         JSONB NOT NULL DEFAULT '{}',
    evaluated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (resource_id)
);

CREATE INDEX IF NOT EXISTS idx_resource_health_cluster ON resource_health (cluster_id, health);

CREATE TABLE IF NOT EXISTS atlas_incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      TEXT NOT NULL DEFAULT 'local',
    resource_id     UUID NOT NULL REFERENCES cluster_resources(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    severity        incident_severity NOT NULL,
    status          incident_status NOT NULL DEFAULT 'open',
    reason          TEXT NOT NULL,
    health_before   health_state,
    health_after    health_state NOT NULL,
    opened_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_atlas_incidents_status ON atlas_incidents (cluster_id, status);
CREATE INDEX IF NOT EXISTS idx_atlas_incidents_resource ON atlas_incidents (resource_id);

CREATE TABLE IF NOT EXISTS incident_context (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id     UUID NOT NULL REFERENCES atlas_incidents(id) ON DELETE CASCADE,
    logs            JSONB NOT NULL DEFAULT '[]',
    events          JSONB NOT NULL DEFAULT '[]',
    describe_data   JSONB NOT NULL DEFAULT '{}',
    deployment_yaml TEXT,
    replicaset_info JSONB NOT NULL DEFAULT '{}',
    node_info       JSONB NOT NULL DEFAULT '{}',
    restart_count   INT NOT NULL DEFAULT 0,
    image_details   JSONB NOT NULL DEFAULT '[]',
    env_vars        JSONB NOT NULL DEFAULT '[]',
    volume_mounts   JSONB NOT NULL DEFAULT '[]',
    raw_payload     JSONB NOT NULL DEFAULT '{}',
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (incident_id)
);

CREATE TABLE IF NOT EXISTS ai_investigations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id         UUID NOT NULL REFERENCES atlas_incidents(id) ON DELETE CASCADE,
    summary             TEXT NOT NULL,
    root_cause          TEXT NOT NULL,
    confidence_score    NUMERIC(5,4) NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 1),
    impact_assessment   TEXT NOT NULL,
    evidence            JSONB NOT NULL DEFAULT '[]',
    recommended_fix     TEXT NOT NULL,
    model_version       TEXT NOT NULL DEFAULT 'kubeatlas-rules-v1',
    investigated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (incident_id)
);

CREATE TABLE IF NOT EXISTS remediation_recommendations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id         UUID NOT NULL REFERENCES atlas_incidents(id) ON DELETE CASCADE,
    investigation_id    UUID REFERENCES ai_investigations(id) ON DELETE SET NULL,
    action_type         TEXT NOT NULL,
    reason              TEXT NOT NULL,
    confidence_score    NUMERIC(5,4) NOT NULL,
    risk_score          NUMERIC(5,4) NOT NULL,
    expected_outcome    TEXT NOT NULL,
    parameters          JSONB NOT NULL DEFAULT '{}',
    status              action_status NOT NULL DEFAULT 'pending',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_remediation_incident ON remediation_recommendations (incident_id);
CREATE INDEX IF NOT EXISTS idx_remediation_status ON remediation_recommendations (status);

CREATE TABLE IF NOT EXISTS pending_actions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recommendation_id   UUID NOT NULL REFERENCES remediation_recommendations(id) ON DELETE CASCADE,
    requested_by        TEXT NOT NULL DEFAULT 'system',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (recommendation_id)
);

CREATE TABLE IF NOT EXISTS approved_actions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recommendation_id   UUID NOT NULL REFERENCES remediation_recommendations(id) ON DELETE CASCADE,
    approved_by         TEXT NOT NULL,
    approved_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (recommendation_id)
);

CREATE TABLE IF NOT EXISTS execution_history (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id          TEXT NOT NULL DEFAULT 'local',
    recommendation_id   UUID NOT NULL REFERENCES remediation_recommendations(id) ON DELETE CASCADE,
    approved_by         TEXT NOT NULL,
    action_type         TEXT NOT NULL,
    parameters          JSONB NOT NULL DEFAULT '{}',
    success             BOOLEAN NOT NULL,
    failure_reason      TEXT,
    rolled_back         BOOLEAN NOT NULL DEFAULT FALSE,
    rollback_reason     TEXT,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    duration_ms         BIGINT
);

CREATE INDEX IF NOT EXISTS idx_execution_history_cluster ON execution_history (cluster_id, started_at DESC);
