ALTER TABLE change_events
    ADD COLUMN IF NOT EXISTS graph_node_id UUID REFERENCES graph_nodes(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_change_events_node ON change_events (graph_node_id);

CREATE TABLE IF NOT EXISTS node_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id     UUID NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,
    reason      TEXT,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
