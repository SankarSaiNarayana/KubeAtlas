export default function StatCard({
  label,
  value,
  hint,
  accent,
}: {
  label: string;
  value: number | string;
  hint?: string;
  accent?: "green" | "blue" | "amber" | "red";
}) {
  return (
    <article className={`stat-card ${accent ? `stat-${accent}` : ""}`}>
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
      {hint && <span className="stat-hint">{hint}</span>}
    </article>
  );
}
