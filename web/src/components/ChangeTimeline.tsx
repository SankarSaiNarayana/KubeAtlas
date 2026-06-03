import { ChangeEvent } from "../api/client";

const SOURCE_ICON: Record<string, string> = {
  gitops: "⎇",
  kubectl: "⌘",
  audit: "◎",
  robusta: "◆",
  api: "•",
};

function formatTime(iso: string) {
  const d = new Date(iso);
  const now = new Date();
  const mins = Math.round((now.getTime() - d.getTime()) / 60000);
  if (mins < 60) return `${mins}m ago`;
  if (mins < 1440) return `${Math.floor(mins / 60)}h ago`;
  return d.toLocaleString();
}

export default function ChangeTimeline({
  changes,
  compact = false,
}: {
  changes: ChangeEvent[];
  compact?: boolean;
}) {
  if (changes.length === 0) {
    return <p className="muted empty-inline">No changes in the last 24 hours</p>;
  }

  return (
    <ul className={`timeline ${compact ? "timeline-compact" : ""}`}>
      {changes.map((c) => (
        <li key={c.id} className="timeline-item">
          <div className="timeline-marker">{SOURCE_ICON[c.source] ?? "•"}</div>
          <div className="timeline-body">
            <div className="timeline-head">
              <span className="timeline-resource">
                {c.resource.kind}/{c.resource.namespace}/{c.resource.name}
              </span>
              <span className="timeline-time">{formatTime(c.occurred_at)}</span>
            </div>
            <p className="timeline-summary">{c.diff_summary}</p>
            <div className="timeline-meta">
              <span className="chip">{c.verb}</span>
              <span className="chip chip-actor">{c.actor}</span>
              <span className="chip chip-muted">{c.source}</span>
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}
